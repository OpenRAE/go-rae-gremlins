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

package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/OpenRAE/go-rae-gremlins/internal/configuration"
	"github.com/OpenRAE/go-rae-gremlins/internal/engine/workdir"
	"github.com/OpenRAE/go-rae-gremlins/internal/engine/workerpool"
	"github.com/OpenRAE/go-rae-gremlins/internal/gomodule"
	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
	"github.com/OpenRAE/go-rae-gremlins/internal/process"
)

// DefaultTimeoutCoefficient is the default multiplier for the timeout length
// of each test run.
const DefaultTimeoutCoefficient = 5

// ExecutorDealer is the initializer for new workerpool.Executor.
type ExecutorDealer interface {
	// NewExecutor returns the workerpool.Executor of mut. The executor sends
	// one ExecutorResult to outCh and calls wg.Done when it completes.
	NewExecutor(mut mutator.Mutator, outCh chan<- ExecutorResult, wg *sync.WaitGroup) workerpool.Executor
}

// ExecutorResult is the outcome of one workerpool.Executor. Each executor
// sends exactly one result. Err is not nil when the executor could not
// produce a valid status for Mutant.
type ExecutorResult struct {
	Mutant mutator.Mutator
	Err    error
}

// MutantExecutorDealer is a ExecutorDealer for the initialisation of a mutantExecutor.
//
// By default, it sets uses exec.Command to perform the tests on the source
// code. This can be overridden, for example in tests.
//
// The apply and rollback functions are wrappers around the TokenMutator apply and
// rollback. These can be overridden with nop functions in tests. Not an
// ideal setup. In the future we can think of a better way to handle this.
type MutantExecutorDealer struct {
	wdDealer          workdir.Dealer
	execContext       execContext
	mod               gomodule.GoModule
	buildTags         string
	testExecutionTime time.Duration
	dryRun            bool
	integrationMode   bool
	testCPU           int
}

// ExecutorDealerOption is the defining option for the initialisation of a ExecutorDealer.
type ExecutorDealerOption func(j MutantExecutorDealer) MutantExecutorDealer

// WithExecContext overrides the default exec.Command with a custom executor.
func WithExecContext(c execContext) ExecutorDealerOption {
	return func(m MutantExecutorDealer) MutantExecutorDealer {
		m.execContext = c

		return m
	}
}

// NewExecutorDealer initialises a MutantExecutorDealer.
func NewExecutorDealer(mod gomodule.GoModule, wdd workdir.Dealer, elapsed time.Duration, opts ...ExecutorDealerOption) *MutantExecutorDealer {
	buildTags := configuration.Get[string](configuration.UnleashTagsKey)
	dryRun := configuration.Get[bool](configuration.UnleashDryRunKey)
	integrationMode := configuration.Get[bool](configuration.UnleashIntegrationMode)
	testCPU := configuration.Get[int](configuration.UnleashTestCPUKey)
	tCoefficient := configuration.Get[int](configuration.UnleashTimeoutCoefficientKey)

	coefficient := DefaultTimeoutCoefficient
	if tCoefficient != 0 {
		coefficient = tCoefficient
	}

	if testCPU != 0 && integrationMode {
		testCPU /= testCPU
	}

	// Use a minimum of 1 second for timeout calculation to prevent
	// unreasonably short timeouts when coverage runs very quickly
	baseTime := elapsed
	if baseTime < time.Second {
		baseTime = time.Second
	}

	jd := MutantExecutorDealer{
		mod:               mod,
		wdDealer:          wdd,
		buildTags:         buildTags,
		dryRun:            dryRun,
		integrationMode:   integrationMode,
		testCPU:           testCPU,
		testExecutionTime: baseTime * time.Duration(coefficient),
		execContext:       exec.CommandContext,
	}

	for _, opt := range opts {
		jd = opt(jd)
	}

	return &jd
}

// NewExecutor returns a new workerpool.Executor for the given mutator.Mutator.
// It gets an output channel of ExecutorResult and a sync.WaitGroup. The
// executor sends one result to the channel, and the wait group will be done
// when the executor is complete.
func (m MutantExecutorDealer) NewExecutor(
	mut mutator.Mutator, outCh chan<- ExecutorResult, wg *sync.WaitGroup,
) workerpool.Executor {
	mj := mutantExecutor{
		mutant:            mut,
		outCh:             outCh,
		wg:                wg,
		wdDealer:          m.wdDealer,
		module:            m.mod,
		dryRun:            m.dryRun,
		integrationMode:   m.integrationMode,
		buildTags:         m.buildTags,
		execContext:       m.execContext,
		testCPU:           m.testCPU,
		testExecutionTime: m.testExecutionTime,
	}

	return &mj
}

type execContext = func(ctx context.Context, name string, args ...string) *exec.Cmd

type mutantExecutor struct {
	mutant            mutator.Mutator
	wdDealer          workdir.Dealer
	outCh             chan<- ExecutorResult
	wg                *sync.WaitGroup
	execContext       execContext
	module            gomodule.GoModule
	buildTags         string
	testExecutionTime time.Duration
	dryRun            bool
	integrationMode   bool
	testCPU           int
}

// Start is the implementation of the workerpool.Executor definition and is the
// method responsible for performing the actual mutation testing.
// The executor runs on its mutator.Mutator.
// If it is RUNNABLE, and it is not in dry-run mode, it will apply the mutation,
// run the tests and mark the TokenMutator as either KILLED or LIVED depending
// on the result. If the tests pass, it means the TokenMutator survived, so it
// will be LIVED, if the tests fail, the TokenMutator will be KILLED.
// The timeout of the test is managed outside the run of the test, using
// a context with timeout. This is done because the Go test command doesn't
// make it easy to distinguish failures from timeouts.
// Start sends one ExecutorResult for the mutant, unless ctx is done. A failure
// to get the working directory, apply the mutation, run the tests, or restore
// the source is sent as the result error. When ctx is done, the executor stops
// its test run, rolls back the mutation, and does not report the mutant.
func (m *mutantExecutor) Start(ctx context.Context, w *workerpool.Worker) {
	defer m.wg.Done()
	if ctx.Err() != nil {
		return
	}
	workerName := fmt.Sprintf("%s-%d", w.Name, w.ID)
	err := m.execute(ctx, workerName)
	if ctx.Err() != nil {
		return
	}
	m.outCh <- ExecutorResult{Mutant: m.mutant, Err: err}
}

// execute applies the mutation, runs the tests, and restores the source as
// one operation. When the apply or the restoration fails, the source in the
// worker directory is unknown, so execute discards that directory. The next
// executor of the worker then gets a new copy of the module. When ctx is done,
// execute does not set the mutant status.
func (m *mutantExecutor) execute(ctx context.Context, workerName string) error {
	rootDir, err := m.wdDealer.Get(workerName)
	if err != nil {
		return fmt.Errorf("get working directory for worker %s: %w", workerName, err)
	}

	workingDir := filepath.Join(rootDir, m.module.CallingDir)
	m.mutant.SetWorkdir(workingDir)

	if m.mutant.Status() == mutator.NotCovered || m.mutant.Status() == mutator.Skipped || m.dryRun {
		return nil
	}

	if err := m.mutant.Apply(); err != nil {
		return m.discard(workerName, fmt.Errorf("apply mutation at %s: %w", m.mutant.Position(), err))
	}

	status, runErr := m.runTests(ctx, rootDir, m.mutant.Pkg())
	if runErr != nil {
		runErr = fmt.Errorf("run tests of mutation at %s: %w", m.mutant.Position(), runErr)
	}

	if err := m.mutant.Rollback(); err != nil {
		restoreErr := fmt.Errorf("restore mutation at %s: %w", m.mutant.Position(), err)

		return m.discard(workerName, errors.Join(runErr, restoreErr))
	}

	if runErr != nil {
		return runErr
	}

	if ctx.Err() == nil {
		m.mutant.SetStatus(status)
	}

	return nil
}

// discard removes the working directory of workerName and returns cause
// joined with any discard error.
func (m *mutantExecutor) discard(workerName string, cause error) error {
	if err := m.wdDealer.Discard(workerName); err != nil {
		return errors.Join(cause, fmt.Errorf("discard working directory of worker %s: %w", workerName, err))
	}

	return cause
}

// runTests runs the tests for pkg and classifies the mutant status from the
// result. A mutant whose package does not compile must be NOT VIABLE, not
// KILLED: the test suite never ran, so it has no opinion on the mutation.
// go test exits 1 for both a build failure and a genuine assertion failure,
// so the exit code alone cannot distinguish them; the classification reads
// the compiler's own "[build failed]"/"[setup failed]" markers from the
// captured output instead.
// An error not caused by the test run itself (for example a missing `go`
// executable) is returned as an error rather than folded into a status, so
// an infrastructure failure is never reported as a passing mutant.
func (m *mutantExecutor) runTests(ctx context.Context, rootDir, pkg string) (mutator.Status, error) {
	ctx, cancel := context.WithTimeout(ctx, m.testExecutionTime)
	defer cancel()

	cmd := m.execContext(ctx, "go", m.getTestArgs(pkg)...)
	cmd.Dir = m.mutant.Workdir()
	if m.integrationMode {
		cmd.Dir = rootDir
	}
	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Env = append(cmd.Env, fmt.Sprintf("GOTMPDIR=%s", m.wdDealer.WorkDir()))

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	err := process.Run(cmd)

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return mutator.TimedOut, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return getTestFailedStatus(output.Bytes()), nil
	}

	if err != nil {
		return 0, fmt.Errorf("failed to run tests for %s: %w", pkg, err)
	}

	return mutator.Lived, nil
}

func (m *mutantExecutor) getTestArgs(pkg string) []string {
	args := []string{"test"}
	if m.buildTags != "" {
		args = append(args, "-tags", m.buildTags)
	}
	// Here we add some seconds to the timeout to be sure it's gremlins that catches the test
	// timeout and not the test itself. The timeout on the test prevents the test.* processes
	// from hanging forever.
	args = append(args, "-timeout", (2*time.Second + m.testExecutionTime).String())
	args = append(args, "-failfast")

	if m.testCPU != 0 {
		args = append(args, "-cpu", fmt.Sprintf("%d", m.testCPU))
	}

	path := pkg
	if m.integrationMode {
		path = "./..."
	}
	args = append(args, path)

	return args
}

// buildFailureMarkers are the markers cmd/go prints in place of a test's
// PASS/FAIL line when the package under test, or one of its dependencies,
// did not build or initialize.
var buildFailureMarkers = [][]byte{
	[]byte("[build failed]"),
	[]byte("[setup failed]"),
}

func getTestFailedStatus(output []byte) mutator.Status {
	for _, marker := range buildFailureMarkers {
		if bytes.Contains(output, marker) {
			return mutator.NotViable
		}
	}

	return mutator.Killed
}
