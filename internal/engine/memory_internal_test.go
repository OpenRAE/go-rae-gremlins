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

package engine

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/OpenRAE/go-rae-gremlins/internal/configuration"
)

const mib = 1 << 20

// availableBytes returns an available-memory function that reports n.
func availableBytes(n uint64) func() (uint64, error) {
	return func() (uint64, error) { return n, nil }
}

// setBudget sets the configured memory budget for the test and restores the
// previous budget when the test ends. It does not change other keys, which
// other tests of the package set.
func setBudget(t *testing.T, budget string) {
	t.Helper()
	prev := configuration.Get[string](configuration.UnleashMemoryBudgetKey)
	t.Cleanup(func() { configuration.Set(configuration.UnleashMemoryBudgetKey, prev) })
	configuration.Set(configuration.UnleashMemoryBudgetKey, budget)
}

func TestFeatureNewMemoryPlanDividesTheBudget(t *testing.T) {
	testCases := map[string]struct {
		budget    string
		available uint64
		peak      uint64
		wantLimit uint64
		wantSlots int
	}{
		"twice the peak of a small run": {
			budget: "1G", peak: 100 * mib, wantLimit: 200 * mib, wantSlots: 5,
		},
		"an unknown peak takes the whole budget": {
			budget: "1G", peak: 0, wantLimit: 1024 * mib, wantSlots: 1,
		},
		"a limit above the budget is capped": {
			budget: "1G", peak: 600 * mib, wantLimit: 1024 * mib, wantSlots: 1,
		},
		"the default budget is at most 1 GiB": {
			available: 8192 * mib, peak: 128 * mib, wantLimit: 256 * mib, wantSlots: 4,
		},
		"the default budget is at most half of the available memory": {
			available: 1024 * mib, peak: 128 * mib, wantLimit: 256 * mib, wantSlots: 2,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			setBudget(t, tc.budget)
			plan, err := newMemoryPlan(tc.peak, availableBytes(tc.available))
			if err != nil {
				t.Fatalf("newMemoryPlan(%d) = %v, want nil", tc.peak, err)
			}
			if plan.limit != tc.wantLimit {
				t.Errorf("newMemoryPlan(%d).limit = %d, want %d", tc.peak, plan.limit, tc.wantLimit)
			}
			if cap(plan.slots) != tc.wantSlots {
				t.Errorf("newMemoryPlan(%d) slots = %d, want %d", tc.peak, cap(plan.slots), tc.wantSlots)
			}
		})
	}
}

func TestFeatureNewMemoryPlanReportsBudgetFailures(t *testing.T) {
	errProbe := errors.New("probe failed")
	testCases := map[string]struct {
		budget    string
		available func() (uint64, error)
		want      string
	}{
		"an invalid budget": {
			budget: "lots", available: availableBytes(1024 * mib), want: "invalid memory budget",
		},
		"an unknown available memory": {
			available: func() (uint64, error) { return 0, errProbe }, want: configuration.UnleashMemoryBudgetKey,
		},
		"no available memory": {
			available: availableBytes(1), want: "no memory is available",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			setBudget(t, tc.budget)
			_, err := newMemoryPlan(mib, tc.available)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("newMemoryPlan() error = %v, want an error that contains %q", err, tc.want)
			}
		})
	}
}

func TestFeatureMemoryPlanReserveWaitsForAFreeSlot(t *testing.T) {
	plan := memoryPlan{limit: mib, slots: make(chan struct{}, 1)}
	release, err := plan.reserve(context.Background())
	if err != nil {
		t.Fatalf("reserve(first) = %v, want nil", err)
	}

	waiting, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := plan.reserve(waiting); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("reserve(full budget) = %v, want %v", err, context.DeadlineExceeded)
	}

	release()
	second, err := plan.reserve(context.Background())
	if err != nil {
		t.Fatalf("reserve(after release) = %v, want nil", err)
	}
	second()
}

func TestFeatureMarkerWriterFindsSplitMarkersInBoundedMemory(t *testing.T) {
	markers := [][]byte{[]byte("[build failed]")}
	testCases := map[string]struct {
		writes []string
		want   bool
	}{
		"no marker":            {writes: []string{"ok  \tpkg\t0.1s\n"}, want: false},
		"one write":            {writes: []string{"FAIL\tpkg [build failed]\n"}, want: true},
		"marker split in two":  {writes: []string{"FAIL\tpkg [build", " failed]\n"}, want: true},
		"marker split by byte": {writes: strings.Split("[build failed]", ""), want: true},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			w := newMarkerWriter(markers)
			for _, s := range tc.writes {
				if n, err := w.Write([]byte(s)); n != len(s) || err != nil {
					t.Fatalf("Write(%q) = %d, %v, want %d, nil", s, n, err, len(s))
				}
			}
			if w.found() != tc.want {
				t.Errorf("found() after %q = %t, want %t", tc.writes, w.found(), tc.want)
			}
		})
	}

	w := newMarkerWriter(markers)
	if _, err := w.Write([]byte(strings.Repeat("x", mib))); err != nil {
		t.Fatalf("Write(1 MiB) = %v, want nil", err)
	}
	if len(w.tail) > len(markers[0]) {
		t.Errorf("kept %d bytes after 1 MiB of output, want at most %d", len(w.tail), len(markers[0]))
	}
}
