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
	"testing"
	"testing/fstest"

	"github.com/google/go-cmp/cmp"

	"github.com/OpenRAE/go-rae-gremlins/internal/configuration"
	"github.com/OpenRAE/go-rae-gremlins/internal/coverage"
	"github.com/OpenRAE/go-rae-gremlins/internal/engine"
	"github.com/OpenRAE/go-rae-gremlins/internal/gomodule"
	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
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
}
`

// TestFeatureRunDiscoversEachOperatorInOrder checks the mutation types that
// the discovery gives for each operator, and their order. The expected list
// comes from the mutation type reference, not from the discovery code.
func TestFeatureRunDiscoversEachOperatorInOrder(t *testing.T) {
	want := []string{
		"4:9 ARITHMETIC_BASE",
		"5:8 INVERT_NEGATIVES", "5:8 ARITHMETIC_BASE",
		"6:6 INVERT_NEGATIVES", "6:6 ARITHMETIC_BASE",
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
