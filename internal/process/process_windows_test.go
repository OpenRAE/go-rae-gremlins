//go:build windows

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
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"github.com/OpenRAE/go-rae-gremlins/internal/process"
)

// stopWaitMillis is stopWait in the milliseconds of WaitForSingleObject.
const stopWaitMillis = uint32(stopWait / time.Millisecond)

// TestFeatureRunStopsDescendants runs a helper that starts a blocking
// descendant. A cancellation and a normal exit of the helper must both stop
// the descendant before Run returns.
func TestFeatureRunStopsDescendants(t *testing.T) {
	testCases := []struct {
		name       string
		cancel     bool
		wantKilled bool
	}{
		{
			name:       "cancellation stops the parent and its descendant",
			cancel:     true,
			wantKilled: true,
		},
		{
			name: "a descendant left after a normal exit is stopped",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			stdinR, stdinW := testPipe(t)
			pidR, pidW := testPipe(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cmd := helperCommand(ctx, pidParent)
			cmd.Stdin = stdinR
			cmd.Stdout = pidW
			done := make(chan error, 1)
			go func() { done <- process.Run(cmd) }()

			descendant := openDescendant(t, pidR, done)
			if event, err := windows.WaitForSingleObject(descendant, 0); event != uint32(windows.WAIT_TIMEOUT) {
				t.Fatalf("descendant state before the stop = %d, %v, want running", event, err)
			}
			if tc.cancel {
				cancel()
			} else if _, err := stdinW.Write([]byte{1}); err != nil {
				t.Fatalf("release the parent: %v", err)
			}
			var err error
			select {
			case err = <-done:
			case <-time.After(stopWait):
				t.Fatalf("Run still waiting %s after the stop", stopWait)
			}
			checkWindowsRunError(t, err, tc.wantKilled)
			// Run returns after it terminates the job. Process termination
			// completes asynchronously, so wait a bounded time for the exit.
			event, err := windows.WaitForSingleObject(descendant, stopWaitMillis)
			if event != windows.WAIT_OBJECT_0 {
				t.Errorf("descendant state after Run = %d, %v, want exited", event, err)
			}
		})
	}
}

// checkWindowsRunError checks that err is nil, or the exit of a killed
// process when wantKilled is set.
func checkWindowsRunError(t *testing.T, err error, wantKilled bool) {
	t.Helper()
	if !wantKilled {
		if err != nil {
			t.Errorf("Run() = %v, want nil", err)
		}

		return
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Errorf("Run() = %v, want the exit of a killed process", err)
	}
}

// openDescendant reads the descendant process ID from the helper and opens
// a handle to it. The handle keeps the ID reserved until the test ends.
func openDescendant(t *testing.T, pidR *os.File, done <-chan error) windows.Handle {
	t.Helper()
	line := make(chan string, 1)
	readErr := make(chan error, 1)
	go func() {
		text, err := bufio.NewReader(pidR).ReadString('\n')
		if err != nil {
			readErr <- err

			return
		}
		line <- text
	}()
	var text string
	select {
	case text = <-line:
	case err := <-readErr:
		t.Fatalf("read the descendant process ID: %v", err)
	case err := <-done:
		t.Fatalf("Run returned %v before the stop, want it blocked", err)
	case <-time.After(stopWait):
		t.Fatalf("no descendant process ID within %s", stopWait)
	}
	pid, err := strconv.ParseUint(strings.TrimSpace(text), 10, 32)
	if err != nil {
		t.Fatalf("parse the descendant process ID %q: %v", text, err)
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		t.Fatalf("open the descendant process %d: %v", pid, err)
	}
	t.Cleanup(func() {
		if err := windows.CloseHandle(handle); err != nil {
			t.Errorf("close the descendant handle: %v", err)
		}
	})

	return handle
}

// testPipe returns a pipe whose ends the test closes at cleanup.
func testPipe(t *testing.T) (reader, writer *os.File) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create a pipe: %v", err)
	}
	t.Cleanup(func() {
		closeFile(t, reader)
		closeFile(t, writer)
	})

	return reader, writer
}
