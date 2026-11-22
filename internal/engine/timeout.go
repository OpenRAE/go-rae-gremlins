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
	"fmt"
	"math"
	"sync"
	"time"
)

const (
	// DefaultTimeoutCoefficient is the default multiplier for the timeout length
	// of each test run.
	DefaultTimeoutCoefficient = 5
	// DefaultTimeoutSafetyMargin is the default multiplier of the measured
	// test time of a package that gives the adaptive timeout.
	DefaultTimeoutSafetyMargin = 2

	// minTimeoutBase is the smallest time that a multiplier applies to, so
	// a very fast run does not give an unreasonably short timeout.
	minTimeoutBase = time.Second
	// testTimeoutGrace is added to the timeout of go test, so Gremlins, not
	// go test, stops a test run at the timeout.
	testTimeoutGrace = 2 * time.Second
	// maxTimeout is the largest timeout. It leaves room for testTimeoutGrace.
	maxTimeout = time.Duration(math.MaxInt64) - testTimeoutGrace
)

// timeoutPlan gives the timeout of each mutant test run. A package with no
// sample uses the fallback timeout. A sample is the elapsed time of a test
// run of the package whose tests passed. The plan keeps the largest sample
// of each package, and a later run uses margin times that sample, capped
// at the fallback. The executors of one dealer share one plan.
type timeoutPlan struct {
	samples  map[string]time.Duration
	mu       sync.Mutex
	fallback time.Duration
	margin   int
	adaptive bool
}

// newTimeoutPlan returns the plan for baseline, the elapsed time of the
// coverage run. The fallback timeout is coefficient times baseline. A zero
// coefficient or margin selects its default. A plan that is not adaptive
// always uses the fallback timeout.
func newTimeoutPlan(baseline time.Duration, coefficient, margin int, adaptive bool) *timeoutPlan {
	if coefficient == 0 {
		coefficient = DefaultTimeoutCoefficient
	}
	if margin == 0 {
		margin = DefaultTimeoutSafetyMargin
	}

	return &timeoutPlan{
		samples:  make(map[string]time.Duration),
		fallback: scaleTimeout(baseline, coefficient),
		margin:   margin,
		adaptive: adaptive,
	}
}

// timeout returns the timeout of the next test run of pkg.
func (p *timeoutPlan) timeout(pkg string) time.Duration {
	if !p.adaptive {
		return p.fallback
	}
	p.mu.Lock()
	sample, ok := p.samples[pkg]
	p.mu.Unlock()
	if !ok {
		return p.fallback
	}

	return min(scaleTimeout(sample, p.margin), p.fallback)
}

// record keeps elapsed as the sample of pkg when it is larger than the
// current sample. Call it only for a complete test run whose tests passed.
func (p *timeoutPlan) record(pkg string, elapsed time.Duration) {
	if !p.adaptive || elapsed <= 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if elapsed > p.samples[pkg] {
		p.samples[pkg] = elapsed
	}
}

// String describes the timeouts of the plan for the run log.
func (p *timeoutPlan) String() string {
	if !p.adaptive {
		return fmt.Sprintf("Timeout of each test run: %s", p.fallback)
	}

	return fmt.Sprintf("Timeout of each test run: %s, or %d times the longest passing run of its package, if shorter",
		p.fallback, p.margin)
}

// scaleTimeout returns n times the larger of d and minTimeoutBase, at most
// maxTimeout.
func scaleTimeout(d time.Duration, n int) time.Duration {
	d = max(d, minTimeoutBase)
	if int64(n) > int64(maxTimeout/d) {
		return maxTimeout
	}

	return d * time.Duration(n)
}
