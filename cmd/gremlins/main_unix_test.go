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
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

const signalWait = 10 * time.Second

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
