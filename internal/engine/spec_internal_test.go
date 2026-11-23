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
	"errors"
	"go/ast"
	"go/token"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
)

// tokenTable returns the token replacement table of s.
func tokenTable(t *testing.T, s spec) map[token.Token]token.Token {
	t.Helper()
	switch s := s.(type) {
	case tokenSpec:
		return s.replacements
	case logicalSpec:
		return s.replacements
	default:
		t.Fatalf("spec of %s has unknown kind %T", s.mutationType(), s)

		return nil
	}
}

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

	got := tokenTable(t, builtins.byType[mutator.InvertAssignments])
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("InvertAssignments replacements mismatch (-want +got):\n%s", diff)
	}
}

func TestRegressionTokenMutationsChangeTheToken(t *testing.T) {
	t.Parallel()

	for _, s := range builtins.byType {
		for original, mutated := range tokenTable(t, s) {
			if original == mutated {
				t.Errorf("%s maps %q to itself; the mutant changes nothing", s.mutationType(), original)
			}
		}
	}
}

// TestFeatureSpecMutatesEachOperator checks the edit of each operator of each
// mutation type, as the pages in docs/docs/usage/mutations/ give them, and
// the restoration of the original token.
func TestFeatureSpecMutatesEachOperator(t *testing.T) {
	t.Parallel()

	want := map[mutator.Type]map[token.Token]token.Token{
		mutator.ArithmeticBase: {
			token.ADD: token.SUB, token.SUB: token.ADD, token.MUL: token.QUO, token.QUO: token.MUL, token.REM: token.MUL,
		},
		mutator.ConditionalsBoundary: {
			token.GTR: token.GEQ, token.LSS: token.LEQ, token.GEQ: token.GTR, token.LEQ: token.LSS,
		},
		mutator.ConditionalsNegation: {
			token.EQL: token.NEQ, token.NEQ: token.EQL, token.LEQ: token.GTR,
			token.GTR: token.LEQ, token.LSS: token.GEQ, token.GEQ: token.LSS,
		},
		mutator.IncrementDecrement: {token.INC: token.DEC, token.DEC: token.INC},
		mutator.InvertAssignments: {
			token.ADD_ASSIGN: token.SUB_ASSIGN, token.SUB_ASSIGN: token.ADD_ASSIGN, token.MUL_ASSIGN: token.QUO_ASSIGN,
			token.QUO_ASSIGN: token.MUL_ASSIGN, token.REM_ASSIGN: token.MUL_ASSIGN,
		},
		mutator.InvertBitwise: {
			token.AND: token.OR, token.OR: token.AND, token.XOR: token.AND,
			token.AND_NOT: token.AND, token.SHL: token.SHR, token.SHR: token.SHL,
		},
		mutator.InvertBitwiseAssignments: {
			token.AND_ASSIGN: token.OR_ASSIGN, token.OR_ASSIGN: token.AND_ASSIGN, token.XOR_ASSIGN: token.AND_ASSIGN,
			token.AND_NOT_ASSIGN: token.AND_ASSIGN, token.SHL_ASSIGN: token.SHR_ASSIGN, token.SHR_ASSIGN: token.SHL_ASSIGN,
		},
		mutator.InvertLogical:   {token.LAND: token.LOR, token.LOR: token.LAND},
		mutator.InvertLoopCtrl:  {token.BREAK: token.CONTINUE, token.CONTINUE: token.BREAK},
		mutator.InvertNegatives: {token.SUB: token.ADD},
		mutator.RemoveSelfAssignments: {
			token.ADD_ASSIGN: token.ASSIGN, token.SUB_ASSIGN: token.ASSIGN, token.MUL_ASSIGN: token.ASSIGN,
			token.QUO_ASSIGN: token.ASSIGN, token.REM_ASSIGN: token.ASSIGN, token.AND_ASSIGN: token.ASSIGN,
			token.OR_ASSIGN: token.ASSIGN, token.XOR_ASSIGN: token.ASSIGN, token.AND_NOT_ASSIGN: token.ASSIGN,
			token.SHL_ASSIGN: token.ASSIGN, token.SHR_ASSIGN: token.ASSIGN,
		},
	}

	for _, mt := range mutator.Types {
		s := builtins.byType[mt]
		for orig, mutated := range want[mt] {
			tok := orig
			n := &NodeToken{tok: &tok}
			if !s.eligible(n) {
				t.Errorf("%s eligible(%s) = false, want true", mt, orig)

				continue
			}
			restore, err := s.mutate(n)
			if err != nil {
				t.Errorf("%s mutate(%s) error = %v, want nil", mt, orig, err)

				continue
			}
			if tok != mutated {
				t.Errorf("%s mutate(%s) = %s, want %s", mt, orig, tok, mutated)
			}
			restore()
			if tok != orig {
				t.Errorf("%s restore after mutate(%s) = %s, want %s", mt, orig, tok, orig)
			}
		}
		if got := len(tokenTable(t, s)); got != len(want[mt]) {
			t.Errorf("%s has %d token replacements, want %d", mt, got, len(want[mt]))
		}
	}
}

// TestFeatureSpecRejectsIneligibleOperator checks that a spec does not change
// an operator that its type cannot mutate.
func TestFeatureSpecRejectsIneligibleOperator(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		operand ast.Expr
		tok     token.Token
		mt      mutator.Type
	}{
		"token spec":                 {tok: token.LAND, mt: mutator.ArithmeticBase},
		"logical spec":               {tok: token.ADD, mt: mutator.InvertLogical},
		"logical spec, unary minus":  {tok: token.SUB, mt: mutator.InvertLogical, operand: &ast.Ident{Name: "a"}},
		"logical spec, not equal to": {tok: token.NEQ, mt: mutator.InvertLogical},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tok := tc.tok
			operand := tc.operand
			n := &NodeToken{tok: &tok}
			if operand != nil {
				n.operand = &operand
			}
			s := builtins.byType[tc.mt]
			if s.eligible(n) {
				t.Errorf("%s eligible(%s) = true, want false", tc.mt, tc.tok)
			}

			_, err := s.mutate(n)

			if !errors.Is(err, ErrNoMutation) {
				t.Errorf("%s mutate(%s) error = %v, want %v", tc.mt, tc.tok, err, ErrNoMutation)
			}
			if tok != tc.tok || operand != tc.operand {
				t.Errorf("%s mutate(%s) changed the operator to %s %v", tc.mt, tc.tok, tok, operand)
			}
		})
	}
}

// TestFeatureRegistryAcceptsTheBuiltinSpecs checks that the built-in specs
// form a registry with one spec for each type of mutator.Types.
func TestFeatureRegistryAcceptsTheBuiltinSpecs(t *testing.T) {
	t.Parallel()

	r, err := newRegistry(builtinSpecs()...)

	if err != nil || r == nil {
		t.Fatalf("newRegistry(builtinSpecs()) = %v, %v, want a registry, nil", r, err)
	}
	for _, mt := range mutator.Types {
		if s, ok := r.byType[mt]; !ok || s.mutationType() != mt {
			t.Errorf("newRegistry(builtinSpecs()) spec of %s = %v, want a spec of that type", mt, s)
		}
	}
}

// TestFeatureRegistryRejectsInvalidRegistration checks that newRegistry
// accepts only one spec for each type of mutator.Types.
func TestFeatureRegistryRejectsInvalidRegistration(t *testing.T) {
	t.Parallel()

	testCases := map[string][]spec{
		"nil spec":              append(builtinSpecs(), nil),
		"unknown type":          append(builtinSpecs(), tokenSpec{mt: mutator.Type(len(mutator.Types))}),
		"second spec of a type": append(builtinSpecs(), tokenSpec{mt: mutator.InvertLogical}),
		"missing type":          builtinSpecs()[1:],
		"no specs":              nil,
	}
	for name, specs := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			r, err := newRegistry(specs...)

			if !errors.Is(err, errInvalidRegistry) || r != nil {
				t.Errorf("newRegistry() = %v, %v, want nil, %v", r, err, errInvalidRegistry)
			}
		})
	}
}

// TestFeatureMustRegistryPanicsOnInvalidRegistration checks that an invalid
// built-in registration stops the program.
func TestFeatureMustRegistryPanicsOnInvalidRegistration(t *testing.T) {
	t.Parallel()

	defer func() {
		err, ok := recover().(error)
		if !ok || !errors.Is(err, errInvalidRegistry) {
			t.Errorf("mustRegistry() panic = %v, want %v", err, errInvalidRegistry)
		}
	}()
	mustRegistry()
}
