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

package process

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

const (
	// internalRoleEnv selects a helper role of TestMain in process_test.go.
	internalRoleEnv = "GO_TEST_PROCESS_ROLE"
	internalWait    = 10 * time.Second
)

// TestFeatureStopOutsideTreeReturnsOnlyTheCause checks the stop of a command whose
// process tree did not start. The expected exit of the command must not join
// the cause, so the cause cannot look like a test result.
func TestFeatureStopOutsideTreeReturnsOnlyTheCause(t *testing.T) {
	testCases := []struct {
		name   string
		role   string
		exited bool
	}{
		{
			name: "a running command is killed",
			role: "block",
		},
		{
			name:   "a command that already exited is only waited for",
			role:   "exit",
			exited: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.exited && runtime.GOOS == "windows" {
				t.Skip("Windows returns an error when it kills a process that exited and was not waited for")
			}
			cmd, output := startInternalHelper(t, tc.role)
			waitOutput(t, output, tc.exited)
			cause := errors.New("start the process tree")

			err := stopOutsideTree(cmd, cause)

			if !errors.Is(err, cause) || err.Error() != cause.Error() {
				t.Errorf("stopOutsideTree() = %v, want only %v", err, cause)
			}
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				t.Errorf("stopOutsideTree() = %v, want no exit error of the command", err)
			}
			if cmd.ProcessState == nil {
				t.Error("stopOutsideTree() did not wait for the command")
			}
		})
	}
}

// startInternalHelper starts the test binary in role and returns the read
// end of its stdout.
func startInternalHelper(t *testing.T, role string) (*exec.Cmd, *os.File) {
	t.Helper()
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("create the output pipe: %v", err)
	}
	t.Cleanup(func() {
		if err := outR.Close(); err != nil {
			t.Errorf("close the output pipe: %v", err)
		}
	})
	// #nosec G204 G702 - The test runs its own binary.
	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), internalRoleEnv+"="+role)
	cmd.Stdout = outW
	startErr := cmd.Start()
	if err := outW.Close(); err != nil {
		t.Fatalf("close the output pipe of the test: %v", err)
	}
	if startErr != nil {
		t.Fatalf("start the helper %s: %v", role, startErr)
	}
	t.Cleanup(func() {
		if cmd.ProcessState != nil {
			return
		}
		if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			t.Errorf("kill the helper: %v", err)
		}
		var exitErr *exec.ExitError
		if err := cmd.Wait(); err != nil && !errors.As(err, &exitErr) {
			t.Errorf("wait for the helper: %v", err)
		}
	})

	return cmd, outR
}

// waitOutput waits for the first output byte of the helper, or for EOF when
// exited is set. EOF comes when the helper exits, because it starts no
// descendant.
func waitOutput(t *testing.T, output *os.File, exited bool) {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		var err error
		if exited {
			_, err = io.ReadAll(output)
		} else {
			_, err = output.Read(make([]byte, 1))
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("read the helper output: %v", err)
		}
	case <-time.After(internalWait):
		t.Fatalf("no helper output within %s", internalWait)
	}
}
