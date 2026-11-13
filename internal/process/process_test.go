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
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/OpenRAE/go-rae-gremlins/internal/process"
)

const (
	// helperRoleEnv makes the test binary run a helper role instead of the
	// tests.
	helperRoleEnv = "GO_TEST_PROCESS_ROLE"
	// helperRun selects no test, so a child without a role does nothing.
	helperRun   = "-test.run=^$"
	helperLimit = 30 * time.Second
	stopWait    = 10 * time.Second
	failingCode = 3

	treeParentBlock = "parent-block"
	treeParentExit  = "parent-exit"
	treeDescendant  = "descendant"
	// inheritOutputEnv makes the descendant hold the output pipe of its parent.
	inheritOutputEnv = "GO_TEST_PROCESS_INHERIT_OUTPUT"
)

// TestMain runs the helper role in helperRoleEnv instead of the tests.
func TestMain(m *testing.M) {
	if role := os.Getenv(helperRoleEnv); role != "" {
		os.Exit(runHelper(role)) // skipcq: RVV-A0003
	}
	os.Exit(m.Run()) // skipcq: RVV-A0003
}

// runHelper runs a helper role and returns its exit code.
func runHelper(role string) int {
	switch role {
	case "fail":
		return failingCode
	case "block":
		if _, err := os.Stdout.WriteString("ready\n"); err != nil {
			return 2
		}
		time.Sleep(helperLimit)
	case treeParentBlock, treeParentExit:
		return runTreeParent(role)
	case treeDescendant:
		return runTreeDescendant()
	}

	return 0
}

// runTreeParent starts one descendant and waits until it is ready. Then it
// writes its first letter to the report descriptor 3, and blocks or exits.
func runTreeParent(role string) int {
	report := os.NewFile(3, "report")
	readyR, readyW, err := os.Pipe()
	if err != nil {
		return 2
	}
	// #nosec G204 G702 - The helper starts the test binary itself.
	cmd := exec.CommandContext(context.Background(), os.Args[0], helperRun)
	cmd.Env = append(os.Environ(), helperRoleEnv+"="+treeDescendant)
	cmd.ExtraFiles = []*os.File{report, readyW}
	if os.Getenv(inheritOutputEnv) == "1" {
		cmd.Stdout = os.Stdout
	}
	if err := cmd.Start(); err != nil {
		return 2
	}
	if err := readyW.Close(); err != nil {
		return 2
	}
	if _, err := readyR.Read(make([]byte, 1)); err != nil {
		return 2
	}
	if _, err := report.Write([]byte(role[:1])); err != nil {
		return 2
	}
	if role == treeParentBlock {
		time.Sleep(helperLimit)
	}

	return 0
}

// runTreeDescendant writes its first letter to the report descriptor 3,
// signals readiness on descriptor 4, and blocks. It keeps descriptor 3 open
// until it exits, so the reader of the report pipe gets EOF only then.
func runTreeDescendant() int {
	if _, err := os.NewFile(3, "report").Write([]byte(treeDescendant[:1])); err != nil {
		return 2
	}
	ready := os.NewFile(4, "ready")
	if _, err := ready.Write([]byte{1}); err != nil {
		return 2
	}
	if err := ready.Close(); err != nil {
		return 2
	}
	time.Sleep(helperLimit)

	return 0
}

// helperCommand returns a command that runs the helper role.
func helperCommand(ctx context.Context, role string) *exec.Cmd {
	// #nosec G204 G702 - The test runs its own binary.
	cmd := exec.CommandContext(ctx, os.Args[0], helperRun)
	cmd.Env = append(os.Environ(), helperRoleEnv+"="+role)

	return cmd
}

func TestFeatureRunReturnsTheCommandResult(t *testing.T) {
	if err := process.Run(helperCommand(context.Background(), "exit")); err != nil {
		t.Errorf("Run(successful command) = %v, want nil", err)
	}

	err := process.Run(helperCommand(context.Background(), "fail"))
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != failingCode {
		t.Errorf("Run(failing command) = %v, want exit code %d", err, failingCode)
	}
}

func TestFeatureRunStopsTheCommandOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	readyR, readyW, err := os.Pipe()
	if err != nil {
		t.Fatalf("create the readiness pipe: %v", err)
	}
	t.Cleanup(func() {
		closeFile(t, readyR)
		closeFile(t, readyW)
	})
	cmd := helperCommand(ctx, "block")
	cmd.Stdout = readyW
	done := make(chan error, 1)
	go func() { done <- process.Run(cmd) }()

	waitReady(t, readyR, done)
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Error("Run(cancelled command) = nil, want an error")
		}
	case <-time.After(stopWait):
		t.Fatalf("Run still waiting %s after the cancellation", stopWait)
	}
}

// waitReady waits until the helper writes to the readiness pipe.
func waitReady(t *testing.T, readyR *os.File, done <-chan error) {
	t.Helper()
	read := make(chan error, 1)
	go func() {
		_, err := readyR.Read(make([]byte, 1))
		read <- err
	}()
	select {
	case err := <-read:
		if err != nil {
			t.Fatalf("read the helper readiness: %v", err)
		}
	case err := <-done:
		t.Fatalf("Run returned %v before the cancellation, want it blocked", err)
	case <-time.After(stopWait):
		t.Fatalf("helper not ready within %s", stopWait)
	}
}

// closeFile closes f and reports an error other than an earlier close.
func closeFile(t *testing.T, f *os.File) {
	t.Helper()
	if err := f.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
		t.Errorf("close %s: %v", f.Name(), err)
	}
}
