//go:build unix

/*
 * Copyright 2026 The Gremlins Authors
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
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/OpenRAE/go-rae-gremlins/internal/configuration"
	"github.com/OpenRAE/go-rae-gremlins/internal/engine"
	"github.com/OpenRAE/go-rae-gremlins/internal/engine/workerpool"
	"github.com/OpenRAE/go-rae-gremlins/internal/gomodule"
	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
)

// TestFeatureExecutorStopsProcessTree runs a test process that starts a
// descendant and blocks. A timeout or a cancellation during the run must stop
// the complete process tree, roll back the mutation, and report as expected.
func TestFeatureExecutorStopsProcessTree(t *testing.T) {
	testCases := []struct {
		name        string
		coefficient int
		wantStatus  mutator.Status
		cancel      bool
		wantResult  bool
	}{
		{
			name:        "timeout stops the tree and reports TIMED OUT",
			coefficient: 1,
			wantResult:  true,
			wantStatus:  mutator.TimedOut,
		},
		{
			name:        "cancellation during the run stops the tree and reports nothing",
			coefficient: 60,
			cancel:      true,
			wantStatus:  mutator.Runnable,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			viperSet(map[string]any{
				configuration.UnleashDryRunKey:             false,
				configuration.UnleashTimeoutCoefficientKey: tc.coefficient,
			})
			defer viperReset()

			reportR, reportW := treePipe(t)
			treeExec := func(ctx context.Context, name string, _ ...string) *exec.Cmd {
				cmd := getCmd(ctx, []string{treeHelperRun, "--", name})
				cmd.Env = []string{treeRoleEnv + "=" + treeRoleParent}
				cmd.ExtraFiles = []*os.File{reportW}

				return cmd
			}
			mod := gomodule.GoModule{Name: expectedModule, Root: ".", CallingDir: "."}
			mjd := engine.NewExecutorDealer(mod, newWdDealerStub(t), 0, engine.WithExecContext(treeExec))
			mut := &mutantStub{status: mutator.Runnable, mutType: mutator.ConditionalsBoundary, pkg: expectedModule}
			outCh := make(chan engine.ExecutorResult, 1)
			wg := sync.WaitGroup{}
			wg.Add(1)
			executor := mjd.NewExecutor(mut, outCh, &wg)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			go func() {
				defer close(done)
				executor.Start(ctx, &workerpool.Worker{Name: testWorkerName, ID: 1})
			}()

			waitTreeStarted(t, reportR)
			if tc.cancel {
				cancel()
			}
			select {
			case <-done:
			case <-time.After(treeWait):
				t.Fatalf("executor still running %s after the stop, want it stopped", treeWait)
			}
			if err := reportW.Close(); err != nil {
				t.Fatalf("close the report pipe: %v", err)
			}
			waitTreeExited(t, reportR)

			checkTreeResult(t, outCh, tc.wantResult)
			if mut.status != tc.wantStatus {
				t.Errorf("mutant status = %v, want %v", mut.status, tc.wantStatus)
			}
			if !mut.rollbackCalled {
				t.Error("rollback called = false, want true")
			}
		})
	}
}

// treePipe returns a pipe whose ends the test closes at cleanup.
func treePipe(t *testing.T) (reader, writer *os.File) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create the report pipe: %v", err)
	}
	t.Cleanup(func() {
		for _, f := range []*os.File{reader, writer} {
			if err := f.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
				t.Errorf("close the report pipe: %v", err)
			}
		}
	})
	if err := reader.SetReadDeadline(time.Now().Add(2 * treeWait)); err != nil {
		t.Fatalf("set the report pipe deadline: %v", err)
	}

	return reader, writer
}

// waitTreeStarted reads until both helper roles have reported.
func waitTreeStarted(t *testing.T, reportR *os.File) {
	t.Helper()
	var got []byte
	buf := make([]byte, 2)
	for !bytes.ContainsRune(got, rune(treeRoleParent[0])) || !bytes.ContainsRune(got, rune(treeRoleChild[0])) {
		n, err := reportR.Read(buf)
		got = append(got, buf[:n]...)
		if err != nil {
			t.Fatalf("read the helper reports %q: %v, want the parent and the descendant", got, err)
		}
	}
}

// waitTreeExited reads until EOF, which comes only when no helper holds the
// write end of the pipe.
func waitTreeExited(t *testing.T, reportR *os.File) {
	t.Helper()
	extra, err := io.ReadAll(reportR)
	if errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatal("a helper process is still running, want the complete tree stopped")
	}
	if err != nil {
		t.Fatalf("read the report pipe to EOF: %v", err)
	}
	if len(extra) != 0 {
		t.Errorf("unexpected helper reports %q after the stop", extra)
	}
}

func checkTreeResult(t *testing.T, outCh <-chan engine.ExecutorResult, wantResult bool) {
	t.Helper()
	select {
	case got := <-outCh:
		if !wantResult {
			t.Errorf("executor reported a mutant with status %v, want no result", got.Mutant.Status())
		}
		if got.Err != nil {
			t.Errorf("executor result error = %v, want nil", got.Err)
		}
	default:
		if wantResult {
			t.Error("executor sent no result, want one result")
		}
	}
}
