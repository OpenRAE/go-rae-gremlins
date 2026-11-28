/*
 * Copyright 2022 The Gremlins Authors
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

package configuration_test

import (
	"testing"

	"github.com/OpenRAE/go-rae-gremlins/internal/configuration"
	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
)

// TestFeatureMutantDefaultStatus checks the default state of each mutation
// type.
func TestFeatureMutantDefaultStatus(t *testing.T) {
	t.Parallel()
	type testCase struct {
		mutantType mutator.Type
		expected   bool
	}
	testCases := []testCase{
		{
			mutantType: mutator.ArithmeticBase,
			expected:   true,
		},
		{
			mutantType: mutator.ConditionalsBoundary,
			expected:   true,
		},
		{
			mutantType: mutator.ConditionalsNegation,
			expected:   true,
		},
		{
			mutantType: mutator.IncrementDecrement,
			expected:   true,
		},
		{
			mutantType: mutator.InvertBitwise,
			expected:   false,
		},
		{
			mutantType: mutator.InvertBitwiseAssignments,
			expected:   false,
		},
		{
			mutantType: mutator.InvertLogical,
			expected:   false,
		},
		{
			mutantType: mutator.InvertNegatives,
			expected:   true,
		},
		{
			mutantType: mutator.InvertLoopCtrl,
			expected:   false,
		},
		{
			mutantType: mutator.InvertAssignments,
			expected:   false,
		},
		{
			mutantType: mutator.RemoveSelfAssignments,
			expected:   false,
		},
		{
			mutantType: mutator.ExtremeVoid,
			expected:   false,
		},
		{
			mutantType: mutator.ExtremeZero,
			expected:   false,
		},
		{
			mutantType: mutator.ExtremeConstant,
			expected:   false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.mutantType.String(), func(t *testing.T) {
			t.Parallel()
			got := configuration.IsDefaultEnabled(tc.mutantType)

			if got != tc.expected {
				t.Errorf("IsDefaultEnabled(%s) = %q, want %q", tc.mutantType, enabled(got), enabled(tc.expected))
			}
		})
	}

	// This should prevent the behaviour described in #142
	t.Run("all MutantTypes are testes for default", func(t *testing.T) {
		contains := func(testedMT []testCase, mt mutator.Type) bool {
			for _, c := range testedMT {
				if mt == c.mutantType {
					return true
				}
			}

			return false
		}

		for _, mt := range mutator.Types {
			if contains(testCases, mt) {
				continue
			}

			t.Errorf("MutantTypes contains %q which is not tested for default", mt)
		}
	})
}

func enabled(b bool) string {
	if b {
		return "enabled"
	}

	return "disabled"
}

// TestFeatureIsMutantTypeEnabledSelectsTheDescartesGroup checks the effective
// enablement of ADR 0012. With unleash.descartes, the run tests the Descartes
// types and no other type, whatever the per-type values are. Without it, the
// per-type values apply.
func TestFeatureIsMutantTypeEnabledSelectsTheDescartesGroup(t *testing.T) {
	descartes := map[mutator.Type]bool{
		mutator.ExtremeVoid: true, mutator.ExtremeZero: true, mutator.ExtremeConstant: true,
	}
	testCases := []struct {
		want    func(mt mutator.Type) bool
		name    string
		group   bool
		perType bool
	}{
		{name: "group off, all types on", perType: true, want: func(mutator.Type) bool { return true }},
		{name: "group off, all types off", perType: false, want: func(mutator.Type) bool { return false }},
		{name: "group on, all types on", group: true, perType: true, want: func(mt mutator.Type) bool { return descartes[mt] }},
		{name: "group on, all types off", group: true, perType: false, want: func(mt mutator.Type) bool { return descartes[mt] }},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configuration.Reset()
			t.Cleanup(configuration.Reset)
			configuration.Set(configuration.UnleashDescartesKey, tc.group)
			for _, mt := range mutator.Types {
				configuration.Set(configuration.MutantTypeEnabledKey(mt), tc.perType)
			}

			for _, mt := range mutator.Types {
				if got, want := configuration.IsMutantTypeEnabled(mt), tc.want(mt); got != want {
					t.Errorf("IsMutantTypeEnabled(%s) = %t, want %t", mt, got, want)
				}
			}
		})
	}
}
