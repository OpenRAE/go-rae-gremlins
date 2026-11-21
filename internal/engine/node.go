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

package engine

import (
	"go/ast"
	"go/token"
)

// NodeToken is the reference to the actualToken that will be mutated during
// the mutation testing.
type NodeToken struct {
	tok     *token.Token
	operand *ast.Expr
	TokPos  token.Pos
}

// NewTokenNode checks if the ast.Node implementation is supported by
// Gremlins and gets its Tok/Op and relative position.
// It returns false as second parameter if the implementation is not
// supported.
func NewTokenNode(n ast.Node) (*NodeToken, bool) {
	var tok *token.Token
	var operand *ast.Expr
	var pos token.Pos
	switch n := n.(type) {
	case *ast.AssignStmt:
		tok = &n.Tok
		pos = n.TokPos
	case *ast.BinaryExpr:
		tok = &n.Op
		pos = n.OpPos
	case *ast.BranchStmt:
		tok = &n.Tok
		pos = n.TokPos
	case *ast.IncDecStmt:
		tok = &n.Tok
		pos = n.TokPos
	case *ast.UnaryExpr:
		tok = &n.Op
		operand = &n.X
		pos = n.OpPos
	default:
		return &NodeToken{}, false
	}

	return &NodeToken{
		tok:     tok,
		operand: operand,
		TokPos:  pos,
	}, true
}

// Tok returns the reference to the token.Token.
func (n *NodeToken) Tok() token.Token {
	return *n.tok
}

// SetTok sets the token.Token of the tokenNode.
func (n *NodeToken) SetTok(t token.Token) {
	*n.tok = t
}

// wrapOperand puts a unary expression of op in front of the operand of a
// unary expression. It returns a function that puts back the original
// operand, or false if the node has no operand. The added expression has no
// position, so the original positions do not change.
func (n *NodeToken) wrapOperand(op token.Token) (func(), bool) {
	if n.operand == nil {
		return nil, false
	}
	orig := *n.operand
	*n.operand = &ast.UnaryExpr{Op: op, X: orig}

	return func() { *n.operand = orig }, true
}

// inspectTokens calls fn for each node of root that NewTokenNode supports, in
// depth-first order. It does not go into the type constraints of type
// parameters and interfaces. An operator in a constraint, for example the |
// of a type union, is a type term and not an operation. fn returns false to
// stop the walk. inspectTokens returns false if fn stopped the walk.
func inspectTokens(root ast.Node, fn func(*NodeToken) bool) bool {
	constraints := make(map[ast.Expr]struct{})
	stopped := false
	ast.Inspect(root, func(n ast.Node) bool {
		if stopped {
			return false
		}
		if e, ok := n.(ast.Expr); ok {
			if _, skip := constraints[e]; skip {
				return false
			}
		}
		addConstraints(constraints, n)
		t, ok := NewTokenNode(n)
		if !ok {
			return true
		}
		stopped = !fn(t)

		return !stopped
	})

	return !stopped
}

// addConstraints adds to constraints the type constraint expressions that are
// direct parts of n: the constraints of type parameters and the embedded
// elements of an interface.
func addConstraints(constraints map[ast.Expr]struct{}, n ast.Node) {
	switch n := n.(type) {
	case *ast.FuncType:
		addFieldTypes(constraints, n.TypeParams, true)
	case *ast.TypeSpec:
		addFieldTypes(constraints, n.TypeParams, true)
	case *ast.InterfaceType:
		addFieldTypes(constraints, n.Methods, false)
	default:
		// Other nodes have no type constraints.
	}
}

// addFieldTypes adds to constraints the types of the fields of list. If named
// is false, it adds only the types of the fields without names.
func addFieldTypes(constraints map[ast.Expr]struct{}, list *ast.FieldList, named bool) {
	if list == nil {
		return
	}
	for _, f := range list.List {
		if named || len(f.Names) == 0 {
			constraints[f.Type] = struct{}{}
		}
	}
}
