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
	"go/token"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
)

// TestRegressionInvertAssignmentsMappings checks the mutation table in
// docs/docs/usage/mutations/invert_assignments.md.
func TestRegressionInvertAssignmentsMappings(t *testing.T) {
	t.Parallel()

	want := map[token.Token]token.Token{
		token.ADD_ASSIGN: token.SUB_ASSIGN,
		token.SUB_ASSIGN: token.ADD_ASSIGN,
		token.MUL_ASSIGN: token.QUO_ASSIGN,
		token.QUO_ASSIGN: token.MUL_ASSIGN,
		token.REM_ASSIGN: token.MUL_ASSIGN,
	}

	got := tokenMutations[mutator.InvertAssignments]
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("tokenMutations[InvertAssignments] mismatch (-want +got):\n%s", diff)
	}
}

func TestRegressionTokenMutationsChangeTheToken(t *testing.T) {
	t.Parallel()

	for mutantType, mappings := range tokenMutations {
		for original, mutated := range mappings {
			if original == mutated {
				t.Errorf("%s maps %q to itself; the mutant changes nothing", mutantType, original)
			}
		}
	}
}
