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

package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

const (
	signalWait    = 10 * time.Second
	probeLockName = "probe.lock"
)

// TestFeatureSignalContextSurvivesRepeatedSignals sends an interrupt and then
// two termination signals to the test process. The first signal must cancel
// the context, and the later signals must not panic the process.
func TestFeatureSignalContextSurvivesRepeatedSignals(t *testing.T) {
	ctx, stop := signalContext()
	t.Cleanup(stop)
	// The os/signal loop delivers a signal to every registered channel in one
	// pass. A receipt on seen for the second SIGTERM proves that the pass for
	// the first SIGTERM is complete.
	seen := make(chan os.Signal, 1)
	signal.Notify(seen, syscall.SIGTERM)
	t.Cleanup(func() { signal.Stop(seen) })

	sendSignal(t, syscall.SIGINT)
	select {
	case <-ctx.Done():
	case <-time.After(signalWait):
		t.Fatalf("context not done %s after SIGINT", signalWait)
	}
	if context.Cause(ctx) == nil {
		t.Error("context.Cause after SIGINT = nil, want the signal cause")
	}

	for i := range 2 {
		sendSignal(t, syscall.SIGTERM)
		select {
		case <-seen:
		case <-time.After(signalWait):
			t.Fatalf("SIGTERM #%d not delivered within %s", i+1, signalWait)
		}
	}
}

func sendSignal(t *testing.T, sig syscall.Signal) {
	t.Helper()
	if err := syscall.Kill(syscall.Getpid(), sig); err != nil {
		t.Fatalf("send %v to the test process: %v", sig, err)
	}
}

// blockingProbeFiles is a module whose test holds an exclusive lock on the
// file in PROBE_LOCK, reports readiness in PROBE_READY, and
// then blocks for at most one minute. The operating system releases the lock
// when the test process exits, also while the process is a zombie.
var blockingProbeFiles = map[string]string{
	"go.mod":   "module probe\n\ngo 1.25\n",
	"probe.go": "package probe\n\nfunc Add(a, b int) int { return a + b }\n",
	"probe_test.go": `package probe

import (
	"os"
	"syscall"
	"testing"
	"time"
)

func TestAdd(t *testing.T) {
	lock, err := os.OpenFile(os.Getenv("PROBE_LOCK"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	ready := os.Getenv("PROBE_READY")
	if err := os.WriteFile(ready+".tmp", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(ready+".tmp", ready); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Minute)
	_ = Add(1, 2)
}
`,
}

// TestRegressionMainInterruptDuringCoverage sends an interrupt to the binary
// while the coverage run executes the test of the module. The binary must exit
// 1 with the cancellation diagnostic and stop the test process of the module.
func TestRegressionMainInterruptDuringCoverage(t *testing.T) {
	module := writeModule(t, blockingProbeFiles)
	probeDir := t.TempDir()
	lockFile := filepath.Join(probeDir, probeLockName)
	readyFile := filepath.Join(probeDir, "probe.ready")
	cmd, output := mainCommand(t, module, unleashCmd)
	cmd.Env = append(cmd.Env, "PROBE_LOCK="+lockFile, "PROBE_READY="+readyFile)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start main: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() {
		if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			t.Errorf("kill main: %v", err)
		}
		if err := <-done; err != nil {
			t.Logf("main stopped: %v", err)
		}
	})

	waitProbeReady(t, readyFile, done)
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("send the interrupt to main: %v", err)
	}
	select {
	case err := <-done:
		done <- err
		if code := exitCode(t, err); code != 1 {
			t.Errorf("exit code = %d, want 1; output:\n%s", code, output)
		}
	case <-time.After(signalWait):
		t.Fatalf("main still running %s after the interrupt", signalWait)
	}
	if !strings.Contains(output.String(), "mutation testing cancelled") {
		t.Errorf("output does not report the cancellation; output:\n%s", output)
	}
	waitLockReleased(t, probeDir)
}

// waitProbeReady waits until the test of the module holds its lock.
func waitProbeReady(t *testing.T, readyFile string, done chan error) {
	t.Helper()
	deadline := time.After(mainHelperLimit)
	for {
		_, err := os.Stat(readyFile)
		if err == nil {
			return
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("check the probe readiness: %v", err)
		}
		select {
		case err := <-done:
			done <- err
			t.Fatalf("main exited before the probe test started: %v", err)
		case <-deadline:
			t.Fatalf("probe test did not start within %s", mainHelperLimit)
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// waitLockReleased waits until no process holds the lock of the probe test
// in probeDir.
func waitLockReleased(t *testing.T, probeDir string) {
	t.Helper()
	lock, err := os.OpenInRoot(probeDir, probeLockName)
	if err != nil {
		t.Fatalf("open the probe lock: %v", err)
	}
	defer func() {
		if err := lock.Close(); err != nil {
			t.Errorf("close the probe lock: %v", err)
		}
	}()
	deadline := time.Now().Add(signalWait)
	for {
		err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			t.Fatalf("lock the probe lock: %v", err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("probe test process still holds its lock %s after main exited", signalWait)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
