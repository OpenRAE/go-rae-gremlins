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

	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
	"github.com/OpenRAE/go-rae-gremlins/internal/typeinfo"
)

// builtins is the registry of the built-in mutation types.
var builtins = mustRegistry(builtinSpecs()...)

// builtinSpecs returns the specs of the built-in mutation types. Their order
// is the order of the mutants of one operator. For example, - gives an
// INVERT_NEGATIVES mutant before an ARITHMETIC_BASE mutant.
func builtinSpecs() []spec {
	return []spec{
		tokenSpec{mt: mutator.InvertNegatives, accepts: (*NodeToken).unary, replacements: map[token.Token]token.Token{
			token.SUB: token.ADD,
		}},
		tokenSpec{mt: mutator.ArithmeticBase, accepts: arithmeticOperands, replacements: map[token.Token]token.Token{
			token.ADD: token.SUB,
			token.MUL: token.QUO,
			token.QUO: token.MUL,
			token.REM: token.MUL,
			token.SUB: token.ADD,
		}},
		tokenSpec{mt: mutator.ConditionalsBoundary, replacements: map[token.Token]token.Token{
			token.GEQ: token.GTR,
			token.GTR: token.GEQ,
			token.LEQ: token.LSS,
			token.LSS: token.LEQ,
		}},
		tokenSpec{mt: mutator.ConditionalsNegation, replacements: map[token.Token]token.Token{
			token.EQL: token.NEQ,
			token.GEQ: token.LSS,
			token.GTR: token.LEQ,
			token.LEQ: token.GTR,
			token.LSS: token.GEQ,
			token.NEQ: token.EQL,
		}},
		tokenSpec{mt: mutator.IncrementDecrement, replacements: map[token.Token]token.Token{
			token.DEC: token.INC,
			token.INC: token.DEC,
		}},
		tokenSpec{mt: mutator.InvertAssignments, accepts: validSubtraction, replacements: map[token.Token]token.Token{
			token.ADD_ASSIGN: token.SUB_ASSIGN,
			token.MUL_ASSIGN: token.QUO_ASSIGN,
			token.QUO_ASSIGN: token.MUL_ASSIGN,
			token.REM_ASSIGN: token.MUL_ASSIGN,
			token.SUB_ASSIGN: token.ADD_ASSIGN,
		}},
		tokenSpec{mt: mutator.RemoveSelfAssignments, accepts: assignableShiftCount, replacements: map[token.Token]token.Token{
			token.ADD_ASSIGN:     token.ASSIGN,
			token.AND_ASSIGN:     token.ASSIGN,
			token.AND_NOT_ASSIGN: token.ASSIGN,
			token.MUL_ASSIGN:     token.ASSIGN,
			token.OR_ASSIGN:      token.ASSIGN,
			token.QUO_ASSIGN:     token.ASSIGN,
			token.REM_ASSIGN:     token.ASSIGN,
			token.SHL_ASSIGN:     token.ASSIGN,
			token.SHR_ASSIGN:     token.ASSIGN,
			token.SUB_ASSIGN:     token.ASSIGN,
			token.XOR_ASSIGN:     token.ASSIGN,
		}},
		tokenSpec{mt: mutator.InvertBitwise, accepts: binary, replacements: map[token.Token]token.Token{
			token.AND:     token.OR,
			token.OR:      token.AND,
			token.XOR:     token.AND,
			token.AND_NOT: token.AND,
			token.SHL:     token.SHR,
			token.SHR:     token.SHL,
		}},
		tokenSpec{mt: mutator.InvertBitwiseAssignments, replacements: map[token.Token]token.Token{
			token.AND_ASSIGN:     token.OR_ASSIGN,
			token.OR_ASSIGN:      token.AND_ASSIGN,
			token.XOR_ASSIGN:     token.AND_ASSIGN,
			token.AND_NOT_ASSIGN: token.AND_ASSIGN,
			token.SHL_ASSIGN:     token.SHR_ASSIGN,
			token.SHR_ASSIGN:     token.SHL_ASSIGN,
		}},
		logicalSpec{tokenSpec{mt: mutator.InvertLogical, replacements: map[token.Token]token.Token{
			token.LAND: token.LOR,
			token.LOR:  token.LAND,
		}}},
		tokenSpec{mt: mutator.InvertLoopCtrl, accepts: validLoopControl, replacements: map[token.Token]token.Token{
			token.BREAK:    token.CONTINUE,
			token.CONTINUE: token.BREAK,
		}},
	}
}

// The accepts checks below select only the operators whose mutants can build
// and that no other type edits the same way. See
// docs/adr/0011-mutant-eligibility-type-information.md.

// arithmeticOperands accepts binary operators and unary +. INVERT_NEGATIVES
// owns unary -. A + whose - form does not type-check, for example with
// strings, is not accepted.
func arithmeticOperands(n *NodeToken) bool {
	if n.unary() {
		return n.Tok() != token.SUB
	}

	return validSubtraction(n)
}

// validSubtraction accepts a + or += whose - form type-checks. For example,
// strings have no - operator.
func validSubtraction(n *NodeToken) bool {
	return !n.facts.Has(typeinfo.InvalidSubtraction)
}

// assignableShiftCount accepts an assignment operator unless it is a shift
// whose count cannot be assigned to the left operand. x = n then does not
// build.
func assignableShiftCount(n *NodeToken) bool {
	return !n.facts.Has(typeinfo.UnassignableShiftCount)
}

// binary accepts the operator of a binary expression or a statement. The
// unary & takes an address, and | has no unary form.
func binary(n *NodeToken) bool {
	return !n.unary()
}

// validLoopControl accepts continue, and a break where a continue with the
// same label is valid.
func validLoopControl(n *NodeToken) bool {
	return n.Tok() != token.BREAK || n.continuable
}
