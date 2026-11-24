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
)

// builtins is the registry of the built-in mutation types.
var builtins = mustRegistry(builtinSpecs()...)

// builtinSpecs returns the specs of the built-in mutation types. Their order
// is the order of the mutants of one operator. For example, - gives an
// INVERT_NEGATIVES mutant before an ARITHMETIC_BASE mutant.
func builtinSpecs() []spec {
	return []spec{
		tokenSpec{mt: mutator.InvertNegatives, replacements: map[token.Token]token.Token{
			token.SUB: token.ADD,
		}},
		tokenSpec{mt: mutator.ArithmeticBase, replacements: map[token.Token]token.Token{
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
		tokenSpec{mt: mutator.InvertAssignments, replacements: map[token.Token]token.Token{
			token.ADD_ASSIGN: token.SUB_ASSIGN,
			token.MUL_ASSIGN: token.QUO_ASSIGN,
			token.QUO_ASSIGN: token.MUL_ASSIGN,
			token.REM_ASSIGN: token.MUL_ASSIGN,
			token.SUB_ASSIGN: token.ADD_ASSIGN,
		}},
		tokenSpec{mt: mutator.RemoveSelfAssignments, replacements: map[token.Token]token.Token{
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
		tokenSpec{mt: mutator.InvertBitwise, replacements: map[token.Token]token.Token{
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
		tokenSpec{mt: mutator.InvertLoopCtrl, replacements: map[token.Token]token.Token{
			token.BREAK:    token.CONTINUE,
			token.CONTINUE: token.BREAK,
		}},
	}
}
