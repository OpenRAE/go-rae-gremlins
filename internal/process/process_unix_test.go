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

package process_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/OpenRAE/go-rae-gremlins/internal/process"
)

// runWaitDelay is the documented bound of Run on open output pipes.
const runWaitDelay = 5 * time.Second

func TestFeatureRunStopsDescendants(t *testing.T) {
	testCases := []struct {
		wantErr       error
		name          string
		parent        string
		cancel        bool
		inheritOutput bool
		wantKilled    bool
	}{
		{
			name:          "cancellation stops the parent and its descendant",
			parent:        treeParentBlock,
			cancel:        true,
			inheritOutput: true,
			wantKilled:    true,
		},
		{
			name:   "a descendant left after a normal exit is stopped",
			parent: treeParentExit,
		},
		{
			name:          "a descendant that holds the output pipe is stopped after the wait delay",
			parent:        treeParentExit,
			inheritOutput: true,
			wantErr:       exec.ErrWaitDelay,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reportR, reportW := reportPipe(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cmd := helperCommand(ctx, tc.parent)
			if tc.inheritOutput {
				cmd.Env = append(cmd.Env, inheritOutputEnv+"=1")
			}
			cmd.ExtraFiles = []*os.File{reportW}
			var output bytes.Buffer
			cmd.Stdout = &output
			done := make(chan error, 1)
			go func() { done <- process.Run(cmd) }()

			waitReports(t, reportR, tc.parent[:1]+treeDescendant[:1])
			if tc.cancel {
				cancel()
			}
			var err error
			select {
			case err = <-done:
			case <-time.After(runWaitDelay + stopWait):
				t.Fatalf("Run still waiting %s after the stop", runWaitDelay+stopWait)
			}
			checkRunError(t, err, tc.wantErr, tc.wantKilled)
			if err := reportW.Close(); err != nil {
				t.Fatalf("close the report pipe: %v", err)
			}
			waitTreeExited(t, reportR)
		})
	}
}

// checkRunError checks that err is wantErr, or the exit of a process that a
// signal killed when wantKilled is set.
func checkRunError(t *testing.T, err, wantErr error, wantKilled bool) {
	t.Helper()
	if wantKilled {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != -1 {
			t.Errorf("Run() = %v, want the exit of a killed process", err)
		}

		return
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("Run() = %v, want %v", err, wantErr)
	}
}

// reportPipe returns a pipe whose ends the test closes at cleanup.
func reportPipe(t *testing.T) (reader, writer *os.File) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create the report pipe: %v", err)
	}
	t.Cleanup(func() {
		closeFile(t, reader)
		closeFile(t, writer)
	})
	if err := reader.SetReadDeadline(time.Now().Add(runWaitDelay + 2*stopWait)); err != nil {
		t.Fatalf("set the report pipe deadline: %v", err)
	}

	return reader, writer
}

// waitReports reads until every role in want has reported.
func waitReports(t *testing.T, reportR *os.File, want string) {
	t.Helper()
	var got []byte
	buf := make([]byte, len(want))
	for _, role := range []byte(want) {
		for !bytes.Contains(got, []byte{role}) {
			n, err := reportR.Read(buf)
			got = append(got, buf[:n]...)
			if err != nil {
				t.Fatalf("read the helper reports %q: %v, want %q", got, err, want)
			}
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
