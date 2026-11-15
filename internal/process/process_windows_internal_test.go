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

package process

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
)

// invalidWindowsID is never a process or thread ID, because Windows gives
// only multiples of four.
const invalidWindowsID = 3

// TestFeatureProcessTreeKillBeforeStart cancels a suspended command before
// it is in the job. kill must stop the command, and start must not resume
// it or put it in the job.
func TestFeatureProcessTreeKillBeforeStart(t *testing.T) {
	// #nosec G204 G702 - The test runs its own binary.
	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), internalRoleEnv+"=block")
	tree, err := newProcessTree(cmd)
	if err != nil {
		t.Fatalf("newProcessTree() = %v, want nil", err)
	}
	t.Cleanup(func() {
		if err := tree.close(); err != nil {
			t.Errorf("close() = %v, want nil", err)
		}
	})
	if err := cmd.Start(); err != nil {
		t.Fatalf("start the helper: %v", err)
	}

	if err := tree.kill(); err != nil {
		t.Errorf("kill() before start = %v, want nil", err)
	}
	if err := tree.start(); err != nil {
		t.Errorf("start() after kill = %v, want nil", err)
	}

	if tree.inJob {
		t.Error("start() after kill put the command in the job")
	}
	var exitErr *exec.ExitError
	if err := cmd.Wait(); !errors.As(err, &exitErr) || exitErr.ExitCode() != killedExitCode {
		t.Errorf("Wait() = %v, want the exit of a killed process", err)
	}
}

// TestFeatureResumeProcessReportsFailures checks that a resume that cannot
// find or open a thread returns an error, so Run stops the command.
func TestFeatureResumeProcessReportsFailures(t *testing.T) {
	testCases := []struct {
		resume func() error
		name   string
	}{
		{
			name:   "a process with no thread",
			resume: func() error { return resumeProcess(invalidWindowsID) },
		},
		{
			name:   "a thread that does not exist",
			resume: func() error { return resumeThread(invalidWindowsID) },
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.resume(); err == nil {
				t.Error("resume = nil, want an error")
			}
		})
	}
}
