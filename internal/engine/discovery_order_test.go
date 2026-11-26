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

package engine_test

import (
	"fmt"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/google/go-cmp/cmp"

	"github.com/OpenRAE/go-rae-gremlins/internal/configuration"
	"github.com/OpenRAE/go-rae-gremlins/internal/coverage"
	"github.com/OpenRAE/go-rae-gremlins/internal/engine"
	"github.com/OpenRAE/go-rae-gremlins/internal/gomodule"
	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
	"github.com/OpenRAE/go-rae-gremlins/internal/typeinfo"
)

// operatorsSource has one statement for each operator that a mutation type
// can mutate. The comment of each line names the operator.
const operatorsSource = `package main

func f(a, b int, p, q bool) int {
	c := a + b // ADD
	c = a - b // SUB
	c = -c // unary SUB
	c = a * b // MUL
	c = a / b // QUO
	c = a % b // REM
	c += a // ADD_ASSIGN
	c -= a // SUB_ASSIGN
	c *= a // MUL_ASSIGN
	c /= a // QUO_ASSIGN
	c %= a // REM_ASSIGN
	c &= a // AND_ASSIGN
	c |= a // OR_ASSIGN
	c ^= a // XOR_ASSIGN
	c &^= a // AND_NOT_ASSIGN
	c <<= a // SHL_ASSIGN
	c >>= a // SHR_ASSIGN
	c = a & b // AND
	c = a | b // OR
	c = a ^ b // XOR
	c = a &^ b // AND_NOT
	c = a << b // SHL
	c = a >> b // SHR
	c++ // INC
	c-- // DEC
	p = a == b // EQL
	p = a != b // NEQ
	p = a < b // LSS
	p = a <= b // LEQ
	p = a > b // GTR
	p = a >= b // GEQ
	p = p && q // LAND
	p = p || q // LOR
	p = !q // NOT
	for {
		if p {
			break // BREAK
		}
		continue // CONTINUE
	}
	c = +c // unary ADD
	c = ^c // unary XOR
	_ = &c // unary AND
	switch {
	case p:
		break // BREAK outside a loop
	}
L:
	for {
		switch {
		case p:
			break L // BREAK with the label of a loop
		}
		select {
		default:
			break // BREAK in a select in a loop
		}
	}
M:
	switch {
	case q:
		break M // BREAK with the label of a switch
	}
	for range 3 {
		func() {
			switch {
			case p:
				break // BREAK in a function literal in a loop
			}
		}()
	}
}
`

// TestFeatureRunDiscoversEachOperatorInOrder checks the mutation types that
// the discovery gives for each operator, and their order. The expected list
// comes from the mutation type reference, not from the discovery code.
func TestFeatureRunDiscoversEachOperatorInOrder(t *testing.T) {
	want := []string{
		"4:9 ARITHMETIC_BASE",
		"5:8 ARITHMETIC_BASE",
		"6:6 INVERT_NEGATIVES",
		"7:8 ARITHMETIC_BASE",
		"8:8 ARITHMETIC_BASE",
		"9:8 ARITHMETIC_BASE",
		"10:4 INVERT_ASSIGNMENTS", "10:4 REMOVE_SELF_ASSIGNMENTS",
		"11:4 INVERT_ASSIGNMENTS", "11:4 REMOVE_SELF_ASSIGNMENTS",
		"12:4 INVERT_ASSIGNMENTS", "12:4 REMOVE_SELF_ASSIGNMENTS",
		"13:4 INVERT_ASSIGNMENTS", "13:4 REMOVE_SELF_ASSIGNMENTS",
		"14:4 INVERT_ASSIGNMENTS", "14:4 REMOVE_SELF_ASSIGNMENTS",
		"15:4 REMOVE_SELF_ASSIGNMENTS", "15:4 INVERT_BWASSIGN",
		"16:4 REMOVE_SELF_ASSIGNMENTS", "16:4 INVERT_BWASSIGN",
		"17:4 REMOVE_SELF_ASSIGNMENTS", "17:4 INVERT_BWASSIGN",
		"18:4 REMOVE_SELF_ASSIGNMENTS", "18:4 INVERT_BWASSIGN",
		"19:4 REMOVE_SELF_ASSIGNMENTS", "19:4 INVERT_BWASSIGN",
		"20:4 REMOVE_SELF_ASSIGNMENTS", "20:4 INVERT_BWASSIGN",
		"21:8 INVERT_BITWISE",
		"22:8 INVERT_BITWISE",
		"23:8 INVERT_BITWISE",
		"24:8 INVERT_BITWISE",
		"25:8 INVERT_BITWISE",
		"26:8 INVERT_BITWISE",
		"27:3 INCREMENT_DECREMENT",
		"28:3 INCREMENT_DECREMENT",
		"29:8 CONDITIONALS_NEGATION",
		"30:8 CONDITIONALS_NEGATION",
		"31:8 CONDITIONALS_BOUNDARY", "31:8 CONDITIONALS_NEGATION",
		"32:8 CONDITIONALS_BOUNDARY", "32:8 CONDITIONALS_NEGATION",
		"33:8 CONDITIONALS_BOUNDARY", "33:8 CONDITIONALS_NEGATION",
		"34:8 CONDITIONALS_BOUNDARY", "34:8 CONDITIONALS_NEGATION",
		"35:8 INVERT_LOGICAL",
		"36:8 INVERT_LOGICAL",
		"37:6 INVERT_LOGICAL",
		"40:4 INVERT_LOOPCTRL",
		"42:3 INVERT_LOOPCTRL",
		"44:6 ARITHMETIC_BASE",
		"55:4 INVERT_LOOPCTRL",
		"59:4 INVERT_LOOPCTRL",
	}
	enabled := make(map[string]any, len(mutator.Types))
	for _, mt := range mutator.Types {
		enabled[configuration.MutantTypeEnabledKey(mt)] = true
	}
	viperSet(enabled)
	defer viperReset()
	mod := gomodule.GoModule{Name: expectedModule, Root: ".", CallingDir: "."}
	mapFS := fstest.MapFS{"operators.go": {Data: []byte(operatorsSource)}}
	dealer := newJobDealerStub(t)

	mut := engine.New(mod, engine.CodeData{Cov: coverage.Profile{}}, dealer, engine.WithDirFs(mapFS))
	_ = runEngine(t, mut)

	got := make([]string, 0, len(dealer.gotMutants))
	for _, m := range dealer.gotMutants {
		got = append(got, fmt.Sprintf("%d:%d %s", m.Position().Line, m.Position().Column, m.Type()))
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Run() discovered mutants mismatch (-want +got):\n%s", diff)
	}
}

// factsFile is the name of the file of factsSource.
const factsFile = "facts.go"

// factsSource has the operators whose eligibility depends on operand facts.
const factsSource = `package main

func g(s, t string, a int, u uint) {
	s = s + t
	s += t
	a <<= u
}
`

// TestFeatureRunAppliesOperandFacts checks that the discovery gives the
// facts of typeinfo to the specs, by the module-relative file name. Without
// facts, the specs keep their earlier selection.
func TestFeatureRunAppliesOperandFacts(t *testing.T) {
	facts := map[int]typeinfo.Fact{
		strings.Index(factsSource, "+ t"):   typeinfo.InvalidSubtraction,
		strings.Index(factsSource, "+= t"):  typeinfo.InvalidSubtraction,
		strings.Index(factsSource, "<<= u"): typeinfo.UnassignableShiftCount,
	}
	withoutFacts := []string{
		"4:8 ARITHMETIC_BASE",
		"5:4 INVERT_ASSIGNMENTS", "5:4 REMOVE_SELF_ASSIGNMENTS",
		"6:4 REMOVE_SELF_ASSIGNMENTS", "6:4 INVERT_BWASSIGN",
	}
	withFacts := []string{
		"5:4 REMOVE_SELF_ASSIGNMENTS",
		"6:4 INVERT_BWASSIGN",
	}
	testCases := map[string]struct {
		facts      typeinfo.Facts
		callingDir string
		want       []string
	}{
		"no facts":                       {callingDir: ".", want: withoutFacts},
		"facts of the file":              {callingDir: ".", facts: typeinfo.Facts{factsFile: facts}, want: withFacts},
		"facts in a calling directory":   {callingDir: "sub", facts: typeinfo.Facts{"sub/" + factsFile: facts}, want: withFacts},
		"facts of a file of another dir": {callingDir: "sub", facts: typeinfo.Facts{factsFile: facts}, want: withoutFacts},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			enabled := make(map[string]any, len(mutator.Types))
			for _, mt := range mutator.Types {
				enabled[configuration.MutantTypeEnabledKey(mt)] = true
			}
			viperSet(enabled)
			defer viperReset()
			mod := gomodule.GoModule{Name: expectedModule, Root: ".", CallingDir: tc.callingDir}
			mapFS := fstest.MapFS{factsFile: {Data: []byte(factsSource)}}
			dealer := newJobDealerStub(t)
			codeData := engine.CodeData{Cov: coverage.Profile{}, Facts: tc.facts}

			mut := engine.New(mod, codeData, dealer, engine.WithDirFs(mapFS))
			_ = runEngine(t, mut)

			got := make([]string, 0, len(dealer.gotMutants))
			for _, m := range dealer.gotMutants {
				got = append(got, fmt.Sprintf("%d:%d %s", m.Position().Line, m.Position().Column, m.Type()))
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Run() discovered mutants mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
