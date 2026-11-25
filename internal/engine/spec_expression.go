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
	"go/ast"
	"go/token"
)

// isNegation reports whether n is the operator of a unary ! expression.
func isNegation(n *NodeToken) bool {
	return n.unary() && n.Tok() == token.NOT
}

// doubleNegation changes the unary expression !x of n to !!x. It puts the
// added ! in front of the original operand, so x keeps its grouping and is
// evaluated once. The added expression has no position, so the original
// positions do not change. doubleNegation returns the function that puts
// back the original operand.
func doubleNegation(n *NodeToken) func() {
	orig := *n.operand
	*n.operand = &ast.UnaryExpr{Op: token.NOT, X: orig}

	return func() { *n.operand = orig }
}
