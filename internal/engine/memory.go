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
	"fmt"

	"github.com/OpenRAE/go-rae-gremlins/internal/configuration"
)

const (
	// defaultBudgetCap is the largest default memory budget, 1 GiB.
	defaultBudgetCap = 1 << 30
	// memoryLimitCoefficient multiplies the peak memory of the coverage run
	// to give the memory limit of each mutant test run.
	memoryLimitCoefficient = 2
)

// memoryPlan divides the memory budget of a run between the mutant test
// runs. Each test run reserves one slot, which holds limit bytes of the
// budget, so the limits of the concurrent test runs never exceed the budget.
type memoryPlan struct {
	slots chan struct{}
	limit uint64
}

// newMemoryPlan returns the plan for peak, the peak memory of the coverage
// run. The limit is memoryLimitCoefficient times peak, capped at the
// budget. A peak of zero, which means unknown, gives a limit of the whole
// budget. available returns the available memory of the system.
func newMemoryPlan(peak uint64, available func() (uint64, error)) (memoryPlan, error) {
	budget, err := memoryBudget(available)
	if err != nil {
		return memoryPlan{}, err
	}
	limit := budget
	if peak != 0 && peak <= budget/memoryLimitCoefficient {
		limit = peak * memoryLimitCoefficient
	}

	return memoryPlan{limit: limit, slots: make(chan struct{}, budget/limit)}, nil
}

// memoryBudget returns the configured memory budget. Without one, it
// returns the smaller of defaultBudgetCap and half of the available memory.
func memoryBudget(available func() (uint64, error)) (uint64, error) {
	if s := configuration.Get[string](configuration.UnleashMemoryBudgetKey); s != "" {
		budget, err := configuration.ParseSize(s)
		if err != nil {
			return 0, fmt.Errorf("invalid memory budget: %w", err)
		}

		return budget, nil
	}
	free, err := available()
	if err != nil {
		return 0, fmt.Errorf("find the available memory, or set %s: %w", configuration.UnleashMemoryBudgetKey, err)
	}
	budget := min(defaultBudgetCap, free/2)
	if budget == 0 {
		return 0, errors.New("no memory is available for the mutant test runs")
	}

	return budget, nil
}

// reserve waits for a free slot of the budget. It returns the function that
// frees the slot, or the error of ctx when ctx is done first.
func (p memoryPlan) reserve(ctx context.Context) (func(), error) {
	select {
	case p.slots <- struct{}{}:
		return func() { <-p.slots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
