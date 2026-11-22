/*
 * Copyright 2022 The Gremlins Authors
 *
 *    Licensed under the Apache License, Version 2.0 (the "License");
 *    you may not use this file except in compliance with the License.
 *    You may obtain a copy of the License at
 *
 *        http://www.apache.org/licenses/LICENSE-2.0
 *
 *    Unless required by applicable law or agreed to in writing, software
 *    distributed under the License is distributed on an "AS IS" BASIS,
 *    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *    See the License for the specific language governing permissions and
 *    limitations under the License.
 */

package engine_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/OpenRAE/go-rae-gremlins/internal/configuration"
	"github.com/OpenRAE/go-rae-gremlins/internal/engine"
	"github.com/OpenRAE/go-rae-gremlins/internal/engine/workdir"
	"github.com/OpenRAE/go-rae-gremlins/internal/engine/workerpool"
	"github.com/OpenRAE/go-rae-gremlins/internal/gomodule"
	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
)

const (
	testWorkerName = "test"
	examplePackage = "example.com/my/package"
	exampleCallDir = "test/dir"
	exampleTags    = "tag1,t1g2"
)

// testWaitBound is the longest a bounded test wait allows before it fails with
// the blocked operation name. It is far below the package test timeout, so a
// dropped result or a stuck join identifies the operation instead of consuming
// that timeout.
const testWaitBound = 30 * time.Second

// recvWithin returns the next value from ch. It fails with op when the value
// does not arrive within testWaitBound.
func recvWithin[T any](t *testing.T, ch <-chan T, op string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(testWaitBound):
		t.Fatalf("%s did not complete within %s", op, testWaitBound)

		var zero T

		return zero
	}
}

// waitWithin waits for wg in its own goroutine. It fails with op when the join
// does not complete within testWaitBound. It owns the goroutine, so a blocked
// join reports op instead of consuming the package test timeout.
func waitWithin(t *testing.T, wg *sync.WaitGroup, op string) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		wg.Wait()
	}()
	select {
	case <-done:
	case <-time.After(testWaitBound):
		t.Fatalf("%s did not complete within %s", op, testWaitBound)
	}
}

// startExecutor runs ex on w in its own goroutine, returns the single result it
// reports, and joins wg. It fails with op when the result or the join does not
// complete within testWaitBound. It owns the goroutine, so a blocked send or
// join reports op instead of consuming the package test timeout.
func startExecutor(
	t *testing.T,
	ex workerpool.Executor,
	w *workerpool.Worker,
	outCh <-chan engine.ExecutorResult,
	wg *sync.WaitGroup,
	op string,
) engine.ExecutorResult {
	t.Helper()
	go ex.Start(context.Background(), w)
	res := recvWithin(t, outCh, op+" result")
	waitWithin(t, wg, op+" join")

	return res
}

func TestApplyAndRollback(t *testing.T) {
	t.Run("applies and rolls back", func(t *testing.T) {
		wdDealer := newWdDealerStub(t)
		tmpDir, err := wdDealer.Get("")
		if err != nil {
			t.Fatalf("get the working directory: %v", err)
		}
		mod := gomodule.GoModule{
			Name:       expectedModule,
			Root:       tmpDir,
			CallingDir: ".",
		}
		mjd := newExecutorDealer(t, mod, wdDealer, expectedTimeout, engine.WithExecContext(fakeExecCommandSuccess))
		mut := &mutantStub{
			status:  mutator.Runnable,
			mutType: mutator.ConditionalsBoundary,
			pkg:     expectedModule,
		}
		outCh := make(chan engine.ExecutorResult)
		wg := sync.WaitGroup{}
		wg.Add(1)
		executor := mjd.NewExecutor(mut, outCh, &wg)
		w := &workerpool.Worker{
			Name: testWorkerName,
			ID:   1,
		}

		startExecutor(t, executor, w, outCh, &wg, "executor")

		if !mut.applyCalled {
			t.Error("executor did not call Apply(), want one call")
		}

		if !mut.rollbackCalled {
			t.Error("executor did not call Rollback(), want one call")
		}
	})
}

type execContext = func(ctx context.Context, name string, args ...string) *exec.Cmd

func TestMutatorTestExecution(t *testing.T) {
	testCases := []struct {
		testResult    execContext
		name          string
		mutantStatus  mutator.Status
		wantMutStatus mutator.Status
	}{
		{
			name:          "it skips NOT_COVERED",
			testResult:    fakeExecCommandSuccess,
			mutantStatus:  mutator.NotCovered,
			wantMutStatus: mutator.NotCovered,
		},
		{
			name:          "if tests pass then mutation is LIVED",
			testResult:    fakeExecCommandSuccess,
			mutantStatus:  mutator.Runnable,
			wantMutStatus: mutator.Lived,
		},
		{
			name:          "if tests fails then mutation is KILLED",
			testResult:    fakeExecCommandTestsFailure,
			mutantStatus:  mutator.Runnable,
			wantMutStatus: mutator.Killed,
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			viperSet(map[string]any{configuration.UnleashDryRunKey: false})
			defer viperReset()
			wdDealer := newWdDealerStub(t)
			holder := &commandHolder{}
			mod := gomodule.GoModule{
				Name:       expectedModule,
				Root:       ".",
				CallingDir: ".",
			}
			mjd := newExecutorDealer(t, mod, wdDealer, expectedTimeout,
				engine.WithExecContext(fakeExecCommandWithHolder(holder, tc.testResult)),
			)
			mut := &mutantStub{
				status:  tc.mutantStatus,
				mutType: mutator.ConditionalsBoundary,
				pkg:     expectedModule,
			}
			outCh := make(chan engine.ExecutorResult)
			wg := sync.WaitGroup{}
			wg.Add(1)
			executor := mjd.NewExecutor(mut, outCh, &wg)
			w := &workerpool.Worker{
				Name: testWorkerName,
				ID:   1,
			}

			got := startExecutor(t, executor, w, outCh, &wg, "executor").Mutant

			if got.Status() != tc.wantMutStatus {
				t.Errorf("executor result Status() = %v, want %v", got.Status(), tc.wantMutStatus)
			}

			if tc.mutantStatus != mutator.NotCovered {
				goTmpDirEnv := fmt.Sprintf("GOTMPDIR=%s", wdDealer.WorkDir())
				actualGoTmpDir := ""
				for _, v := range holder.cmd.Env {
					if strings.HasPrefix(v, "GOTMPDIR=") {
						actualGoTmpDir = v

						break
					}
				}
				if goTmpDirEnv != actualGoTmpDir {
					t.Errorf("go test command GOTMPDIR entry = %q, want %q", actualGoTmpDir, goTmpDirEnv)
				}
			}
		})
	}
}

// TestMutatorTestExecutionBuildFailureIsNotViable runs the real go toolchain
// against a small, purpose-built package that does not compile, so the
// classification is proven against go test's actual output rather than an
// invented exit code (issue #59): go test exits 1 for a build failure and
// for an assertion failure alike, so only the compiler's own markers in the
// output can tell them apart.
func TestRegressionMutatorTestExecutionBuildFailureIsNotViable(t *testing.T) {
	const probeModule = "probe"

	viperSet(map[string]any{configuration.UnleashDryRunKey: false})
	defer viperReset()

	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":   "module " + probeModule + "\n\ngo 1.25\n",
		"probe.go": "package probe\n\nfunc Label(n int) string { return \"n\" - n }\n",
		"probe_test.go": `package probe

import "testing"

func TestFeatureLabel(t *testing.T) {
	_ = Label(3)
}
`,
	})

	mod := gomodule.GoModule{
		Name:       probeModule,
		Root:       root,
		CallingDir: ".",
	}
	wdDealer := workdir.NewCachedDealer(t.TempDir(), mod.Root)
	defer wdDealer.Clean()

	mjd := newExecutorDealer(t, mod, wdDealer, expectedTimeout)
	mut := &mutantStub{
		status:  mutator.Runnable,
		mutType: mutator.ConditionalsBoundary,
		pkg:     probeModule,
	}
	outCh := make(chan engine.ExecutorResult, 1)
	wg := sync.WaitGroup{}
	wg.Add(1)
	executor := mjd.NewExecutor(mut, outCh, &wg)
	w := &workerpool.Worker{
		Name: testWorkerName,
		ID:   1,
	}

	executor.Start(context.Background(), w)
	waitWithin(t, &wg, "executor join")

	got := recvWithin(t, outCh, "executor result").Mutant
	if got.Status() != mutator.NotViable {
		t.Errorf("status = %v, want %v", got.Status(), mutator.NotViable)
	}
}

// TestFeatureExecutorFailureIsReported proves that each executor failure
// gives a result error instead of a panic or a missing result (issue #106).
// After a failed apply or rollback, the source in the worker directory is
// unknown, so the executor must discard that directory before the worker
// takes the next mutant. A test run that cannot start is never folded into
// mutator.Lived (issue #59).
func TestFeatureExecutorFailureIsReported(t *testing.T) {
	errWorkdir := errors.New("workdir stub error")
	workerDir := fmt.Sprintf("%s-%d", testWorkerName, 1)
	missingExec := func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "gremlins-no-such-executable")
	}

	testCases := []struct {
		fnGet         func(idf string) (string, error)
		testResult    execContext
		wantErr       error
		name          string
		wantDiscarded []string
		applyError    bool
		rollbackError bool
		wantApply     bool
		wantRollback  bool
	}{
		{
			name:       "workdir failure",
			fnGet:      func(_ string) (string, error) { return "", errWorkdir },
			testResult: fakeExecCommandSuccess,
			wantErr:    errWorkdir,
		},
		{
			name:          "apply failure discards the worker directory",
			testResult:    fakeExecCommandSuccess,
			applyError:    true,
			wantErr:       errApplyStub,
			wantApply:     true,
			wantDiscarded: []string{workerDir},
		},
		{
			name:          "rollback failure discards the worker directory",
			testResult:    fakeExecCommandSuccess,
			rollbackError: true,
			wantErr:       errRollbackStub,
			wantApply:     true,
			wantRollback:  true,
			wantDiscarded: []string{workerDir},
		},
		{
			name:         "test run failure keeps the restored worker directory",
			testResult:   missingExec,
			wantErr:      exec.ErrNotFound,
			wantApply:    true,
			wantRollback: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			viperSet(map[string]any{configuration.UnleashDryRunKey: false})
			defer viperReset()

			wdDealer := newWdDealerStub(t)
			if tc.fnGet != nil {
				wdDealer.fnGet = tc.fnGet
			}
			mut := &mutantStub{
				status:           mutator.Runnable,
				mutType:          mutator.ConditionalsBoundary,
				pkg:              expectedModule,
				hasApplyError:    tc.applyError,
				hasRollbackError: tc.rollbackError,
			}

			got := runExecutorOnce(t, wdDealer, tc.testResult, mut)

			if !errors.Is(got.Err, tc.wantErr) {
				t.Errorf("result error = %v, want %v", got.Err, tc.wantErr)
			}
			if got.Mutant != mut {
				t.Errorf("result mutant = %v, want the executed mutant", got.Mutant)
			}
			checkFailedMutant(t, mut, tc.wantApply, tc.wantRollback)
			if !cmp.Equal(wdDealer.discarded, tc.wantDiscarded) {
				t.Errorf("discarded worker directories = %v, want %v", wdDealer.discarded, tc.wantDiscarded)
			}
		})
	}
}

// runExecutorOnce starts one executor of mut on worker 1 and returns its
// single result.
func runExecutorOnce(
	t *testing.T, wdDealer *dealerStub, testResult execContext, mut *mutantStub,
) engine.ExecutorResult {
	t.Helper()
	mod := gomodule.GoModule{
		Name:       expectedModule,
		Root:       ".",
		CallingDir: ".",
	}
	mjd := newExecutorDealer(t, mod, wdDealer, expectedTimeout, engine.WithExecContext(testResult))
	outCh := make(chan engine.ExecutorResult, 1)
	wg := sync.WaitGroup{}
	wg.Add(1)
	executor := mjd.NewExecutor(mut, outCh, &wg)

	executor.Start(context.Background(), &workerpool.Worker{Name: testWorkerName, ID: 1})
	waitWithin(t, &wg, "executor join")

	select {
	case got := <-outCh:
		return got
	default:
		t.Fatal("executor sent no result, want one result")
	}

	return engine.ExecutorResult{}
}

// checkFailedMutant checks that a failed execution left mut without a new
// status and called Apply and Rollback as expected.
func checkFailedMutant(t *testing.T, mut *mutantStub, wantApply, wantRollback bool) {
	t.Helper()
	if mut.status != mutator.Runnable {
		t.Errorf("mutant status = %v, want %v", mut.status, mutator.Runnable)
	}
	if mut.applyCalled != wantApply {
		t.Errorf("apply called = %t, want %t", mut.applyCalled, wantApply)
	}
	if mut.rollbackCalled != wantRollback {
		t.Errorf("rollback called = %t, want %t", mut.rollbackCalled, wantRollback)
	}
}

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("create directory of %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

const expectedTimeout = 10 * time.Second

// goTestProcessEnv makes the test binary run as a helper child.
const goTestProcessEnv = "GO_TEST_PROCESS=1"

type commandHolder struct {
	cmd     *exec.Cmd
	command string
	args    []string
	timeout time.Duration
	m       sync.Mutex
}

func TestMutatorRun(t *testing.T) {
	testCases := []struct {
		name               string
		pkg                string
		callDir            string
		tags               string
		wantPath           string
		timeoutCoefficient int
		intMode            bool
	}{
		{
			name:     "normal mode",
			intMode:  false,
			pkg:      examplePackage,
			callDir:  exampleCallDir,
			tags:     exampleTags,
			wantPath: "-count=1 " + examplePackage,
		},
		{
			name:     "integration mode",
			intMode:  true,
			pkg:      examplePackage,
			callDir:  exampleCallDir,
			tags:     exampleTags,
			wantPath: "-p 1 ./...",
		},
		{
			name:               "it can override timeout coefficient",
			timeoutCoefficient: 4,
			pkg:                examplePackage,
			callDir:            exampleCallDir,
			tags:               exampleTags,
			wantPath:           "-count=1 " + examplePackage,
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			settings := map[string]any{
				configuration.UnleashIntegrationMode: tc.intMode,
				configuration.UnleashTagsKey:         tc.tags,
			}
			if tc.timeoutCoefficient != 0 {
				settings[configuration.UnleashTimeoutCoefficientKey] = tc.timeoutCoefficient
			}
			viperSet(settings)
			defer viperReset()

			mod := gomodule.GoModule{
				Name:       expectedModule,
				Root:       ".",
				CallingDir: tc.callDir,
			}
			wdDealer := newWdDealerStub(t)
			holder := &commandHolder{}
			mjd := newExecutorDealer(t, mod, wdDealer, expectedTimeout,
				engine.WithExecContext(fakeExecCommandSuccessWithHolder(holder)))
			mut := &mutantStub{
				status:  mutator.Runnable,
				mutType: mutator.ConditionalsBoundary,
				pkg:     tc.pkg,
			}
			outCh := make(chan engine.ExecutorResult)
			wg := sync.WaitGroup{}
			wg.Add(1)
			executor := mjd.NewExecutor(mut, outCh, &wg)
			w := &workerpool.Worker{
				Name: testWorkerName,
				ID:   1,
			}
			startExecutor(t, executor, w, outCh, &wg, "executor")

			wantTimeout := 2*time.Second + expectedTimeout*engine.DefaultTimeoutCoefficient
			if tc.timeoutCoefficient != 0 {
				wantTimeout = 2*time.Second + expectedTimeout*time.Duration(tc.timeoutCoefficient)
			}
			want := fmt.Sprintf("go test -tags %s -timeout %s -failfast %s", tc.tags, wantTimeout, tc.wantPath)
			got := fmt.Sprintf("go %v", strings.Join(holder.args, " "))

			if !cmp.Equal(got, want) {
				t.Errorf("\n+ %s\n- %s\n", got, want)
			}

			wantDeadline := wantTimeout - 2*time.Second
			if diff := absTimeDiff(holder.timeout, wantDeadline); diff > time.Second {
				t.Errorf("go test command deadline = %s, want within 1s of %s", holder.timeout, wantDeadline)
			}
		})
	}
}

// execCall is the arguments and the context deadline of one go test command.
type execCall struct {
	args     []string
	deadline time.Duration
}

// execRecorder records each go test command of a dealer.
type execRecorder struct {
	calls []execCall
	m     sync.Mutex
}

// exec returns an execContext that runs first for the first command and a
// passing test process for each later command.
func (r *execRecorder) exec(first execContext) execContext {
	return func(ctx context.Context, command string, args ...string) *exec.Cmd {
		dl, _ := ctx.Deadline()
		r.m.Lock()
		defer r.m.Unlock()
		r.calls = append(r.calls, execCall{args: args, deadline: time.Until(dl)})
		if len(r.calls) == 1 {
			return first(ctx, command, args...)
		}

		return fakeExecCommandSuccess(ctx, command, args...)
	}
}

// testTimeout returns the -timeout value of c and checks that the context
// deadline is the same timeout without the 2 second grace of go test.
func (c execCall) testTimeout(t *testing.T) time.Duration {
	t.Helper()
	for i, arg := range c.args[:len(c.args)-1] {
		if arg != "-timeout" {
			continue
		}
		got, err := time.ParseDuration(c.args[i+1])
		if err != nil {
			t.Fatalf("parse -timeout %q: %v", c.args[i+1], err)
		}
		if diff := absTimeDiff(c.deadline, got-2*time.Second); diff > time.Second {
			t.Errorf("context deadline = %s, want within 1s of the -timeout %s less 2s", c.deadline, got)
		}

		return got
	}
	t.Fatalf("go test args = %q, want a -timeout flag", c.args)

	return 0
}

// TestFeatureExecutorAdaptiveTimeout runs two executors of one dealer, one
// after the other. Both executors exist before the first run, so the second
// executor must select its timeout when it runs.
func TestFeatureExecutorAdaptiveTimeout(t *testing.T) {
	const (
		baseline     = time.Hour
		otherPackage = "example.com/my/other"
	)
	fallbackArg := baseline*engine.DefaultTimeoutCoefficient + 2*time.Second
	testCases := []struct {
		first        execContext
		name         string
		firstPkg     string
		wantFirst    mutator.Status
		intMode      bool
		wantAdaptive bool
	}{
		{
			name:         "a LIVED run sets the timeout of the next run in its package",
			first:        fakeExecCommandSuccess,
			firstPkg:     examplePackage,
			wantFirst:    mutator.Lived,
			wantAdaptive: true,
		},
		{
			name:      "a LIVED run does not change the timeout of another package",
			first:     fakeExecCommandSuccess,
			firstPkg:  otherPackage,
			wantFirst: mutator.Lived,
		},
		{
			name:      "a KILLED run records no sample",
			first:     fakeExecCommandTestsFailure,
			firstPkg:  examplePackage,
			wantFirst: mutator.Killed,
		},
		{
			name:      "integration mode keeps the fallback timeout",
			first:     fakeExecCommandSuccess,
			firstPkg:  examplePackage,
			wantFirst: mutator.Lived,
			intMode:   true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			viperSet(map[string]any{configuration.UnleashIntegrationMode: tc.intMode})
			defer viperReset()
			rec := &execRecorder{}
			mod := gomodule.GoModule{Name: expectedModule, Root: ".", CallingDir: "."}
			mjd := newExecutorDealer(t, mod, newWdDealerStub(t), baseline, engine.WithExecContext(rec.exec(tc.first)))
			first := &mutantStub{status: mutator.Runnable, mutType: mutator.ConditionalsBoundary, pkg: tc.firstPkg}
			second := &mutantStub{status: mutator.Runnable, mutType: mutator.ConditionalsBoundary, pkg: examplePackage}
			outCh := make(chan engine.ExecutorResult)
			var wg sync.WaitGroup
			wg.Add(2)
			executors := []workerpool.Executor{mjd.NewExecutor(first, outCh, &wg), mjd.NewExecutor(second, outCh, &wg)}
			w := &workerpool.Worker{Name: testWorkerName, ID: 1}
			for _, ex := range executors {
				go ex.Start(context.Background(), w)
				if res := recvWithin(t, outCh, "executor result"); res.Err != nil {
					t.Fatalf("executor result error = %v, want nil", res.Err)
				}
			}
			waitWithin(t, &wg, "executors join")

			if first.status != tc.wantFirst {
				t.Fatalf("first mutant status = %v, want %v", first.status, tc.wantFirst)
			}
			if len(rec.calls) != 2 {
				t.Fatalf("go test commands = %d, want 2", len(rec.calls))
			}
			if got := rec.calls[0].testTimeout(t); got != fallbackArg {
				t.Errorf("first -timeout = %s, want the fallback %s", got, fallbackArg)
			}
			got := rec.calls[1].testTimeout(t)
			if !tc.wantAdaptive {
				if got != fallbackArg {
					t.Errorf("second -timeout = %s, want the fallback %s", got, fallbackArg)
				}

				return
			}
			if got < 4*time.Second || got > time.Minute {
				t.Errorf("second -timeout = %s, want from 4s to 1m: two times the short first run, plus 2s", got)
			}
		})
	}
}

func TestCPU(t *testing.T) {
	testCases := []struct {
		name        string
		testCPU     int
		wantTestCPU int
		intMode     bool
		cpuPresent  bool
	}{
		{
			name:       "default normal mode doesn't set CPU",
			cpuPresent: false,
		},
		{
			name:       "default integration mode doesn't set CPU",
			intMode:    true,
			cpuPresent: false,
		},
		{
			name:        "normal mode can override CPU",
			testCPU:     1,
			wantTestCPU: 1,
			cpuPresent:  true,
		},
		{
			name:        "integration mode overrides CPU to half",
			intMode:     true,
			testCPU:     2,
			wantTestCPU: 1,
			cpuPresent:  true,
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			viperSet(map[string]any{
				configuration.UnleashIntegrationMode: tc.intMode,
				configuration.UnleashTestCPUKey:      tc.testCPU,
			})
			defer viperReset()

			mod := gomodule.GoModule{
				Name:       expectedModule,
				Root:       ".",
				CallingDir: ".",
			}
			wdDealer := newWdDealerStub(t)
			holder := &commandHolder{}
			mjd := newExecutorDealer(t, mod, wdDealer, expectedTimeout,
				engine.WithExecContext(fakeExecCommandSuccessWithHolder(holder)))
			mut := &mutantStub{
				status:  mutator.Runnable,
				mutType: mutator.ConditionalsBoundary,
				pkg:     "test",
			}
			outCh := make(chan engine.ExecutorResult)
			wg := sync.WaitGroup{}
			wg.Add(1)
			executor := mjd.NewExecutor(mut, outCh, &wg)
			w := &workerpool.Worker{
				Name: testWorkerName,
				ID:   1,
			}
			startExecutor(t, executor, w, outCh, &wg, "executor")

			for _, arg := range holder.args {
				if !tc.cpuPresent && strings.Contains(arg, "-cpu") {
					t.Fatalf("go test command args = %q, want no -cpu flag", holder.args)
				}
				if !tc.cpuPresent {
					return
				}
				got := fmt.Sprintf("go %v", strings.Join(holder.args, " "))
				cpuFlag := fmt.Sprintf("-cpu %d", tc.wantTestCPU)
				if strings.Contains(got, cpuFlag) {
					// PASS
					return
				}
				t.Fatalf("go test command args = %q, want flag %q", holder.args, cpuFlag)
			}

		})
	}
}

func absTimeDiff(a, b time.Duration) time.Duration {
	if a > b {
		return a - b
	}

	return b - a
}

func TestCoverageProcessSuccess(_ *testing.T) {
	if os.Getenv("GO_TEST_PROCESS") != "1" {
		return
	}
	os.Exit(0) // skipcq: RVV-A0003
}

func TestProcessTestsFailure(_ *testing.T) {
	if os.Getenv("GO_TEST_PROCESS") != "1" {
		return
	}
	os.Exit(1) // skipcq: RVV-A0003
}

// allocated holds the memory of TestFeatureHelperAllocates.
var allocated [][]byte

// TestFeatureHelperAllocates is a helper child that makes 512 MiB of memory
// resident, 1 MiB each millisecond, so a memory watchdog sees the growth.
func TestFeatureHelperAllocates(_ *testing.T) {
	if os.Getenv("GO_TEST_PROCESS") != "1" {
		return
	}
	for range 512 {
		chunk := make([]byte, 1<<20)
		for i := 0; i < len(chunk); i += os.Getpagesize() {
			chunk[i] = 1
		}
		allocated = append(allocated, chunk)
		time.Sleep(time.Millisecond)
	}
	os.Exit(0) // skipcq: RVV-A0003
}

func TestFeatureExecutorReportsMemoryError(t *testing.T) {
	viperSet(map[string]any{
		configuration.UnleashDryRunKey:       false,
		configuration.UnleashMemoryBudgetKey: "64M",
	})
	defer viperReset()
	mut := &mutantStub{
		status:  mutator.Runnable,
		mutType: mutator.ConditionalsBoundary,
		pkg:     expectedModule,
	}

	got := runExecutorOnce(t, newWdDealerStub(t), fakeExecCommandAllocates, mut)

	if got.Err != nil {
		t.Fatalf("result error = %v, want nil", got.Err)
	}
	if mut.status != mutator.MemoryError {
		t.Errorf("mutant status = %v, want %v", mut.status, mutator.MemoryError)
	}
}

func TestMutatorRunInTheCorrectFolder(t *testing.T) {
	t.Run("mutation should run in the correct folder", func(t *testing.T) {
		callingDir := exampleCallDir
		mod := gomodule.GoModule{
			Name:       expectedModule,
			Root:       ".",
			CallingDir: callingDir,
		}
		wdDealer := newWdDealerStub(t)
		holder := &commandHolder{}
		mjd := newExecutorDealer(t, mod, wdDealer, expectedTimeout,
			engine.WithExecContext(fakeExecCommandSuccessWithHolder(holder)))
		mut := &mutantStub{
			status:  mutator.Runnable,
			mutType: mutator.ConditionalsBoundary,
			pkg:     examplePackage,
		}
		outCh := make(chan engine.ExecutorResult)
		wg := sync.WaitGroup{}
		wg.Add(1)
		executor := mjd.NewExecutor(mut, outCh, &wg)
		w := &workerpool.Worker{
			Name: testWorkerName,
			ID:   1,
		}
		startExecutor(t, executor, w, outCh, &wg, "executor")

		if mut.Workdir() != holder.cmd.Dir {
			t.Errorf("go test command Dir = %q, want the mutant workdir %q", holder.cmd.Dir, mut.Workdir())
		}
	})
	t.Run("integration mode: mutation should run in the wd root folder", func(t *testing.T) {
		viperSet(map[string]any{
			configuration.UnleashIntegrationMode: true,
		})
		defer viperReset()
		callingDir := exampleCallDir
		mod := gomodule.GoModule{
			Name:       expectedModule,
			Root:       ".",
			CallingDir: callingDir,
		}
		wantRootDir := "/tmp/static"
		wdDealer := &dealerStub{
			t: nil,
			fnGet: func(_ string) (string, error) {
				return wantRootDir, nil
			},
		}

		holder := &commandHolder{}
		mjd := newExecutorDealer(t, mod, wdDealer, expectedTimeout,
			engine.WithExecContext(fakeExecCommandSuccessWithHolder(holder)))
		mut := &mutantStub{
			status:  mutator.Runnable,
			mutType: mutator.ConditionalsBoundary,
			pkg:     examplePackage,
		}
		outCh := make(chan engine.ExecutorResult)
		wg := sync.WaitGroup{}
		wg.Add(1)
		executor := mjd.NewExecutor(mut, outCh, &wg)
		w := &workerpool.Worker{
			Name: testWorkerName,
			ID:   1,
		}
		startExecutor(t, executor, w, outCh, &wg, "executor")

		if wantRootDir != holder.cmd.Dir {
			t.Errorf("go test command Dir = %q, want %q", holder.cmd.Dir, wantRootDir)
		}
	})
}

func fakeExecCommandSuccess(ctx context.Context, command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestCoverageProcessSuccess", "--", command}
	cs = append(cs, args...)
	// #nosec G204 G702 - We are in tests, we don't care
	cmd := exec.CommandContext(ctx, os.Args[0], cs...)
	cmd.Env = []string{goTestProcessEnv}

	return cmd
}

func fakeExecCommandSuccessWithHolder(got *commandHolder) execContext {
	return func(ctx context.Context, command string, args ...string) *exec.Cmd {
		dl, _ := ctx.Deadline()
		got.m.Lock()
		defer got.m.Unlock()

		cs := []string{"-test.run=TestCoverageProcessSuccess", "--", command}
		cs = append(cs, args...)
		cmd := getCmd(ctx, cs)

		got.cmd = cmd
		got.command = command
		got.args = args
		got.timeout = time.Until(dl)

		return cmd
	}
}

func fakeExecCommandWithHolder(got *commandHolder, fakeCmd func(ctx context.Context, command string, args ...string) *exec.Cmd) execContext {
	return func(ctx context.Context, command string, args ...string) *exec.Cmd {
		dl, _ := ctx.Deadline()
		got.m.Lock()
		defer got.m.Unlock()

		cmd := fakeCmd(ctx, command, args...)
		got.cmd = cmd
		got.command = command
		got.args = args
		got.timeout = time.Until(dl)

		return cmd
	}
}

func fakeExecCommandTestsFailure(ctx context.Context, command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestProcessTestsFailure", "--", command}
	cs = append(cs, args...)

	return getCmd(ctx, cs)
}

func fakeExecCommandAllocates(ctx context.Context, command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=^TestFeatureHelperAllocates$", "--", command}
	cs = append(cs, args...)

	return getCmd(ctx, cs)
}

func getCmd(ctx context.Context, cs []string) *exec.Cmd {
	// #nosec G204 G702 - We are in tests, we don't care
	cmd := exec.CommandContext(ctx, os.Args[0], cs...)
	cmd.Env = []string{goTestProcessEnv}

	return cmd
}

func TestFeatureExecutorCancellation(t *testing.T) {
	testCases := []struct {
		name         string
		cancelOnExec bool
		wantApply    bool
	}{
		{
			name:      "cancelled before start does not apply the mutation",
			wantApply: false,
		},
		{
			name:         "cancelled during the test run rolls back and reports nothing",
			cancelOnExec: true,
			wantApply:    true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			viperSet(map[string]any{configuration.UnleashDryRunKey: false})
			defer viperReset()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if !tc.cancelOnExec {
				cancel()
			}
			cancelingExec := func(c context.Context, name string, args ...string) *exec.Cmd {
				cancel()

				return fakeExecCommandSuccess(c, name, args...)
			}
			wdDealer := newWdDealerStub(t)
			mod := gomodule.GoModule{Name: expectedModule, Root: ".", CallingDir: "."}
			mjd := newExecutorDealer(t, mod, wdDealer, expectedTimeout, engine.WithExecContext(cancelingExec))
			mut := &mutantStub{
				status:  mutator.Runnable,
				mutType: mutator.ConditionalsBoundary,
				pkg:     expectedModule,
			}
			outCh := make(chan engine.ExecutorResult, 1)
			wg := sync.WaitGroup{}
			wg.Add(1)
			executor := mjd.NewExecutor(mut, outCh, &wg)

			executor.Start(ctx, &workerpool.Worker{Name: testWorkerName, ID: 1})
			waitWithin(t, &wg, "cancelled executor join")

			select {
			case got := <-outCh:
				t.Errorf("cancelled executor reported a mutant with status %v, want none", got.Mutant.Status())
			default:
			}
			if mut.applyCalled != tc.wantApply {
				t.Errorf("apply called = %t, want %t", mut.applyCalled, tc.wantApply)
			}
			if mut.rollbackCalled != tc.wantApply {
				t.Errorf("rollback called = %t, want %t", mut.rollbackCalled, tc.wantApply)
			}
		})
	}
}

// TestFeatureExecutorDoesNotRunEquivalentMutant checks that the executor
// neither mutates the source nor runs the tests of an EQUIVALENT mutant, and
// reports it with its status unchanged.
func TestFeatureExecutorDoesNotRunEquivalentMutant(t *testing.T) {
	viperSet(map[string]any{configuration.UnleashDryRunKey: false})
	defer viperReset()
	holder := &commandHolder{}
	mut := &mutantStub{status: mutator.Equivalent, mutType: mutator.ConditionalsBoundary, pkg: expectedModule}

	got := runExecutorOnce(t, newWdDealerStub(t), fakeExecCommandWithHolder(holder, fakeExecCommandSuccess), mut)

	if got.Err != nil {
		t.Errorf("result error = %v, want nil", got.Err)
	}
	if got.Mutant.Status() != mutator.Equivalent {
		t.Errorf("result Status() = %v, want %v", got.Mutant.Status(), mutator.Equivalent)
	}
	if mut.applyCalled || mut.rollbackCalled {
		t.Errorf("apply called = %t, rollback called = %t, want both false", mut.applyCalled, mut.rollbackCalled)
	}
	if holder.cmd != nil {
		t.Errorf("executor ran %q %v, want no test run", holder.command, holder.args)
	}
}
