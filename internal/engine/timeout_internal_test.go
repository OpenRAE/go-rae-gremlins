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
	"math"
	"sync"
	"testing"
	"time"
)

const (
	fastPkg = "example.com/fast"
	slowPkg = "example.com/slow"
)

func TestFeatureTimeoutPlanFallback(t *testing.T) {
	testCases := map[string]struct {
		baseline    time.Duration
		want        time.Duration
		coefficient int
	}{
		"baseline times coefficient": {
			baseline: 3 * time.Second, coefficient: 4, want: 12 * time.Second,
		},
		"zero coefficient selects the default": {
			baseline: 3 * time.Second, want: 3 * time.Second * DefaultTimeoutCoefficient,
		},
		"baseline below one second uses one second": {
			baseline: 100 * time.Millisecond, coefficient: 5, want: 5 * time.Second,
		},
		"overflow saturates": {
			baseline: time.Hour, coefficient: math.MaxInt, want: maxTimeout,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			plan := newTimeoutPlan(tc.baseline, tc.coefficient, 0, true)

			if got := plan.timeout(fastPkg); got != tc.want {
				t.Errorf("timeout() with no sample = %s, want the fallback %s", got, tc.want)
			}
		})
	}
}

func TestFeatureTimeoutPlanAdaptsToSamples(t *testing.T) {
	testCases := map[string]struct {
		samples []time.Duration
		want    time.Duration
		margin  int
	}{
		"sample times the default margin": {
			samples: []time.Duration{3 * time.Second}, want: 3 * time.Second * DefaultTimeoutSafetyMargin,
		},
		"sample times a configured margin": {
			samples: []time.Duration{3 * time.Second}, margin: 3, want: 9 * time.Second,
		},
		"sample below one second uses one second": {
			samples: []time.Duration{100 * time.Millisecond}, margin: 2, want: 2 * time.Second,
		},
		"the largest sample is kept": {
			samples: []time.Duration{4 * time.Second, 2 * time.Second}, margin: 2, want: 8 * time.Second,
		},
		"a larger later sample replaces the record": {
			samples: []time.Duration{2 * time.Second, 4 * time.Second}, margin: 2, want: 8 * time.Second,
		},
		"the fallback caps the timeout": {
			samples: []time.Duration{40 * time.Second}, margin: 2, want: 50 * time.Second,
		},
		"a margin overflow saturates at the fallback": {
			samples: []time.Duration{40 * time.Second}, margin: math.MaxInt, want: 50 * time.Second,
		},
		"a zero sample is ignored": {
			samples: []time.Duration{0}, margin: 2, want: 50 * time.Second,
		},
		"a negative sample is ignored": {
			samples: []time.Duration{-time.Second}, margin: 2, want: 50 * time.Second,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			plan := newTimeoutPlan(10*time.Second, 5, tc.margin, true)
			for _, s := range tc.samples {
				plan.record(fastPkg, s)
			}

			if got := plan.timeout(fastPkg); got != tc.want {
				t.Errorf("timeout() after samples %v = %s, want %s", tc.samples, got, tc.want)
			}
		})
	}
}

func TestFeatureTimeoutPlanIsolatesPackages(t *testing.T) {
	plan := newTimeoutPlan(10*time.Second, 5, 2, true)

	plan.record(fastPkg, 2*time.Second)

	if got, want := plan.timeout(fastPkg), 4*time.Second; got != want {
		t.Errorf("timeout(%q) = %s, want %s", fastPkg, got, want)
	}
	if got, want := plan.timeout(slowPkg), 50*time.Second; got != want {
		t.Errorf("timeout(%q) with no sample = %s, want the fallback %s", slowPkg, got, want)
	}
}

func TestFeatureTimeoutPlanNotAdaptive(t *testing.T) {
	plan := newTimeoutPlan(10*time.Second, 5, 2, false)

	plan.record(fastPkg, 2*time.Second)

	if got, want := plan.timeout(fastPkg), 50*time.Second; got != want {
		t.Errorf("timeout() of a plan that does not adapt = %s, want the fallback %s", got, want)
	}
}

func TestFeatureTimeoutPlanString(t *testing.T) {
	testCases := map[string]struct {
		want     string
		adaptive bool
	}{
		"adaptive": {
			adaptive: true,
			want:     "Timeout of each test run: 50s, or 3 times the longest passing run of its package, if shorter",
		},
		"not adaptive": {
			want: "Timeout of each test run: 50s",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			plan := newTimeoutPlan(10*time.Second, 5, 3, tc.adaptive)

			if got := plan.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestFeatureTimeoutPlanConcurrentUse runs record and timeout from many
// goroutines. The race detector reports unsynchronized access.
func TestFeatureTimeoutPlanConcurrentUse(t *testing.T) {
	plan := newTimeoutPlan(10*time.Second, 5, 2, true)
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Go(func() {
			plan.record(fastPkg, time.Duration(i+1)*time.Second)
			_ = plan.timeout(fastPkg)
		})
	}
	wg.Wait()

	if got, want := plan.timeout(fastPkg), 32*time.Second; got != want {
		t.Errorf("timeout() after concurrent samples = %s, want %s from the largest sample", got, want)
	}
}
