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

package workerpool_test

import (
	"context"
	"fmt"
	"go/token"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/OpenRAE/go-rae-gremlins/internal/configuration"
	"github.com/OpenRAE/go-rae-gremlins/internal/engine/workerpool"
	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
)

// testWaitBound is the longest a bounded test wait allows before it fails with
// the blocked operation name. It is far below the package test timeout, so a
// dropped result identifies the operation instead of consuming that timeout.
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

// runWithin runs fn in its own goroutine. It fails with op when fn does not
// return within testWaitBound. It owns the goroutine, so a blocked call reports
// op instead of consuming the package test timeout.
func runWithin(t *testing.T, fn func(), op string) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
	case <-time.After(testWaitBound):
		t.Fatalf("%s did not complete within %s", op, testWaitBound)
	}
}

type ExecutorMock struct {
	mutant mutator.Mutator
	outCh  chan<- mutator.Mutator
}

func (tj *ExecutorMock) Start(_ context.Context, w *workerpool.Worker) {
	fm := fakeMutant{
		name: w.Name,
		id:   w.ID,
	}
	tj.outCh <- fm
}

func TestWorker(t *testing.T) {
	executorQueue := make(chan workerpool.Executor)
	outCh := make(chan mutator.Mutator)

	worker := workerpool.NewWorker(1, "test")
	worker.Start(context.Background(), executorQueue)

	tj := &ExecutorMock{
		mutant: &fakeMutant{},
		outCh:  outCh,
	}

	executorQueue <- tj
	close(executorQueue)

	m := recvWithin(t, outCh, "worker result")
	got, ok := m.(fakeMutant)
	if !ok {
		t.Fatal("it should be a fakeMutant")
	}

	if got.name != "test" {
		t.Errorf("want %q, got %q", "test", got.name)
	}
	if got.id != 1 {
		t.Errorf("want %d, got %d", 1, got.id)
	}
}

func TestPool(t *testing.T) {
	t.Run("test executes work", func(t *testing.T) {
		configuration.Set(configuration.UnleashWorkersKey, 1)
		defer configuration.Reset()

		outCh := make(chan mutator.Mutator)

		pool := workerpool.Initialize("test")
		pool.Start(context.Background())
		defer pool.Stop()

		tj := &ExecutorMock{
			mutant: &fakeMutant{},
			outCh:  outCh,
		}

		// A context that is never done cannot end the submission early.
		pool.AppendExecutor(context.Background(), tj)

		m := recvWithin(t, outCh, "pool result")
		got, ok := m.(fakeMutant)
		if !ok {
			t.Fatal("it should be a fakeMutant")
		}

		if got.name != "test" {
			t.Errorf("want %q, got %q", "test", got.name)
		}
		if got.id != 0 {
			t.Errorf("want %d, got %d", 0, got.id)
		}
	})

	t.Run("default uses runtime CPUs as number of workers", func(t *testing.T) {
		configuration.Set(configuration.UnleashWorkersKey, 0)
		defer configuration.Reset()

		pool := workerpool.Initialize("test")
		pool.Start(context.Background())
		defer pool.Stop()

		if pool.ActiveWorkers() != runtime.NumCPU() {
			t.Errorf("want %d, got %d", runtime.NumCPU(), pool.ActiveWorkers())
		}
	})

	t.Run("default uses half of runtime CPUs as number of workers in integration mode", func(t *testing.T) {
		configuration.Set(configuration.UnleashWorkersKey, 0)
		configuration.Set(configuration.UnleashIntegrationMode, true)
		defer configuration.Reset()

		pool := workerpool.Initialize("test")
		pool.Start(context.Background())
		defer pool.Stop()

		if pool.ActiveWorkers() != runtime.NumCPU()/2 {
			t.Errorf("want %d, got %d", runtime.NumCPU()/2, pool.ActiveWorkers())
		}
	})

	t.Run("can override CPUs", func(t *testing.T) {
		configuration.Set(configuration.UnleashWorkersKey, 3)
		defer configuration.Reset()

		pool := workerpool.Initialize("test")
		pool.Start(context.Background())
		defer pool.Stop()

		if pool.ActiveWorkers() != 3 {
			t.Errorf("want %d, got %d", 3, pool.ActiveWorkers())
		}
	})

	t.Run("in integration mode, overrides CPUs by half", func(t *testing.T) {
		configuration.Set(configuration.UnleashWorkersKey, 2)
		configuration.Set(configuration.UnleashIntegrationMode, true)
		defer configuration.Reset()

		pool := workerpool.Initialize("test")
		pool.Start(context.Background())
		defer pool.Stop()

		if pool.ActiveWorkers() != 1 {
			t.Errorf("want %d, got %d", 1, pool.ActiveWorkers())
		}
	})
}

func TestRegressionPoolReadsEnvironmentSettings(t *testing.T) {
	t.Run("workers", func(t *testing.T) {
		initConfiguration(t)
		want := runtime.NumCPU() + 1
		t.Setenv("GREMLINS_UNLEASH_WORKERS", strconv.Itoa(want))

		pool := workerpool.Initialize("test")
		pool.Start(context.Background())
		defer pool.Stop()

		if got := pool.ActiveWorkers(); got != want {
			t.Errorf("ActiveWorkers() = %d, want %d", got, want)
		}
	})

	t.Run("integration mode", func(t *testing.T) {
		initConfiguration(t)
		configuration.Set(configuration.UnleashWorkersKey, 2)
		t.Setenv("GREMLINS_UNLEASH_INTEGRATION", "true")

		pool := workerpool.Initialize("test")
		pool.Start(context.Background())
		defer pool.Stop()

		if got := pool.ActiveWorkers(); got != 1 {
			t.Errorf("ActiveWorkers() = %d, want 1", got)
		}
	})
}

// initConfiguration loads an empty configuration that reads GREMLINS_*
// environment variables, and resets it after the test.
func initConfiguration(t *testing.T) {
	t.Helper()
	configuration.Reset()
	t.Cleanup(configuration.Reset)
	if err := configuration.Init([]string{t.TempDir()}); err != nil {
		t.Fatalf("configuration.Init error = %v, want nil", err)
	}
}

type fakeMutant struct {
	name string
	id   int
}

func (fakeMutant) Type() mutator.Type {
	panic("not used in test")
}

func (fakeMutant) SetType(_ mutator.Type) {
	panic("not used in test")
}

func (fakeMutant) Status() mutator.Status {
	panic("not used in test")
}

func (fakeMutant) SetStatus(_ mutator.Status) {
	panic("not used in test")
}

func (fakeMutant) Position() token.Position {
	panic("not used in test")
}

func (fakeMutant) Pos() token.Pos {
	panic("not used in test")
}

func (fakeMutant) Pkg() string {
	panic("not used in test")
}

func (fakeMutant) SetWorkdir(_ string) {
	panic("not used in test")
}

func (fakeMutant) Workdir() string {
	panic("not used in test")
}

func (fakeMutant) Apply() error {
	panic("not used in test")
}

func (fakeMutant) Rollback() error {
	panic("not used in test")
}

func (fakeMutant) OrigSnippet() []byte {
	panic("not used in test")
}

func (fakeMutant) MutatedSnippet() []byte {
	panic("not used in test")
}

func TestFeaturePoolCancelledSubmissionReturnsWhileQueueIsFull(t *testing.T) {
	configuration.Set(configuration.UnleashWorkersKey, 1)
	defer configuration.Reset()

	outCh := make(chan mutator.Mutator)
	pool := workerpool.Initialize("test")
	pool.Start(context.Background())
	defer pool.Stop()
	tj := &ExecutorMock{mutant: &fakeMutant{}, outCh: outCh}

	// The worker blocks on outCh with the first executor. The second
	// fills the queue, so the third submission can only end by cancellation.
	for i := range 2 {
		if !pool.AppendExecutor(context.Background(), tj) {
			t.Fatalf("AppendExecutor #%d = false, want true", i)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if pool.AppendExecutor(ctx, tj) {
		t.Error("AppendExecutor(cancelled context) = true, want false")
	}

	for i := range 2 {
		recvWithin(t, outCh, fmt.Sprintf("queued result #%d", i))
	}
}

// TestFeatureWithheldResultCleanupStopsWorkers proves that owned cleanup after
// a withheld result leaves no worker running (issue #117). The worker blocks
// while it sends its result and the test withholds the read. The test then
// drains the one result as cleanup, and pool.Stop must return within the bound.
// A worker left running would block Stop until the package test timeout.
func TestFeatureWithheldResultCleanupStopsWorkers(t *testing.T) {
	configuration.Set(configuration.UnleashWorkersKey, 1)
	defer configuration.Reset()

	outCh := make(chan mutator.Mutator)
	pool := workerpool.Initialize("test")
	pool.Start(context.Background())
	tj := &ExecutorMock{mutant: &fakeMutant{}, outCh: outCh}

	if !pool.AppendExecutor(context.Background(), tj) {
		t.Fatal("AppendExecutor() = false, want true")
	}

	// Owned cleanup: drain the withheld result so the worker can finish its
	// executor and exit.
	go func() { <-outCh }()

	runWithin(t, pool.Stop, "pool.Stop after withheld result")
}
