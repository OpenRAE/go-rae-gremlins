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

func TestApplyAndRollback(t *testing.T) {
	t.Run("applies and rolls back", func(t *testing.T) {
		wdDealer := newWdDealerStub(t)
		tmpDir, _ := wdDealer.Get("")
		mod := gomodule.GoModule{
			Name:       expectedModule,
			Root:       tmpDir,
			CallingDir: ".",
		}
		mjd := engine.NewExecutorDealer(mod, wdDealer, expectedTimeout, engine.WithExecContext(fakeExecCommandSuccess))
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
		go func() {
			<-outCh
			close(outCh)
		}()

		executor.Start(context.Background(), w)

		wg.Wait()

		if !mut.applyCalled {
			t.Error("expected apply to be called")
		}

		if !mut.rollbackCalled {
			t.Error("expected rollback to be called")
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
			mjd := engine.NewExecutorDealer(mod, wdDealer, expectedTimeout,
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

			var got mutator.Mutator
			mutex := sync.RWMutex{}
			go func() {
				mutex.Lock()
				defer mutex.Unlock()
				got = (<-outCh).Mutant
				close(outCh)
			}()
			executor.Start(context.Background(), w)
			wg.Wait()

			mutex.RLock()
			defer mutex.RUnlock()

			if got.Status() != tc.wantMutStatus {
				t.Errorf("expected mutation to be %v, but got: %v", tc.wantMutStatus, got.Status())
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
					t.Errorf("expected GOTMPDIR to be %s, got %s", goTmpDirEnv, actualGoTmpDir)
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

	mjd := engine.NewExecutorDealer(mod, wdDealer, expectedTimeout)
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
	wg.Wait()

	got := (<-outCh).Mutant
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
	mjd := engine.NewExecutorDealer(mod, wdDealer, expectedTimeout, engine.WithExecContext(testResult))
	outCh := make(chan engine.ExecutorResult, 1)
	wg := sync.WaitGroup{}
	wg.Add(1)
	executor := mjd.NewExecutor(mut, outCh, &wg)

	executor.Start(context.Background(), &workerpool.Worker{Name: testWorkerName, ID: 1})
	wg.Wait()

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
			wantPath: examplePackage,
		},
		{
			name:     "integration mode",
			intMode:  true,
			pkg:      examplePackage,
			callDir:  exampleCallDir,
			tags:     exampleTags,
			wantPath: "./...",
		},
		{
			name:               "it can override timeout coefficient",
			timeoutCoefficient: 4,
			pkg:                examplePackage,
			callDir:            exampleCallDir,
			tags:               exampleTags,
			wantPath:           examplePackage,
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
			mjd := engine.NewExecutorDealer(mod, wdDealer, expectedTimeout,
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
			go func() {
				<-outCh
				close(outCh)
			}()
			executor.Start(context.Background(), w)
			wg.Wait()

			wantTimeout := 2*time.Second + expectedTimeout*engine.DefaultTimeoutCoefficient
			if tc.timeoutCoefficient != 0 {
				wantTimeout = 2*time.Second + expectedTimeout*time.Duration(tc.timeoutCoefficient)
			}
			want := fmt.Sprintf("go test -tags %s -timeout %s -failfast %s", tc.tags, wantTimeout, tc.wantPath)
			got := fmt.Sprintf("go %v", strings.Join(holder.args, " "))

			if !cmp.Equal(got, want) {
				t.Errorf("\n+ %s\n- %s\n", got, want)
			}

			timeoutDifference := absTimeDiff(holder.timeout, expectedTimeout*2)
			diffThreshold := 100 * time.Second
			if timeoutDifference > diffThreshold {
				t.Errorf("expected timeout to be within %s from the set timeout, got %s", diffThreshold, timeoutDifference)
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
			mjd := engine.NewExecutorDealer(mod, wdDealer, expectedTimeout,
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
			go func() {
				<-outCh
				close(outCh)
			}()
			executor.Start(context.Background(), w)
			wg.Wait()

			for _, arg := range holder.args {
				if !tc.cpuPresent && strings.Contains(arg, "-cpu") {
					t.Fatalf("didn't expect to have -cpu flag")
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
				t.Fatalf("want flag %q, got args %s", cpuFlag, holder.args)
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
		mjd := engine.NewExecutorDealer(mod, wdDealer, expectedTimeout,
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
		go func() {
			<-outCh
			close(outCh)
		}()
		executor.Start(context.Background(), w)
		wg.Wait()

		if mut.Workdir() != holder.cmd.Dir {
			t.Errorf("expected working dir to be %s, got %s", holder.cmd.Dir, mut.Workdir())
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
		mjd := engine.NewExecutorDealer(mod, wdDealer, expectedTimeout,
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
		go func() {
			<-outCh
			close(outCh)
		}()
		executor.Start(context.Background(), w)
		wg.Wait()

		if wantRootDir != holder.cmd.Dir {
			t.Errorf("expected working dir to be %q, got %q", wantRootDir, holder.cmd.Dir)
		}
	})
}

func fakeExecCommandSuccess(ctx context.Context, command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestCoverageProcessSuccess", "--", command}
	cs = append(cs, args...)
	// #nosec G204 G702 - We are in tests, we don't care
	cmd := exec.CommandContext(ctx, os.Args[0], cs...)
	cmd.Env = []string{"GO_TEST_PROCESS=1"}

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

func getCmd(ctx context.Context, cs []string) *exec.Cmd {
	// #nosec G204 G702 - We are in tests, we don't care
	cmd := exec.CommandContext(ctx, os.Args[0], cs...)
	cmd.Env = []string{"GO_TEST_PROCESS=1"}

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
			mjd := engine.NewExecutorDealer(mod, wdDealer, expectedTimeout, engine.WithExecContext(cancelingExec))
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
			wg.Wait()

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
