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

	"github.com/OpenRAE/go-rae-gremlins/internal/typeinfo"
)

// NodeToken is the reference to the actualToken that will be mutated during
// the mutation testing, with the context that the specs need to select it.
// A NodeToken can also be a function candidate of the extreme mutation
// types. Its token is then token.FUNC, which is not in the AST, and TokPos is
// the position of the func keyword.
type NodeToken struct {
	tok     *token.Token
	operand *ast.Expr
	fn      *funcCandidate
	TokPos  token.Pos
	// facts are the operand facts of the operator from its types.
	facts typeinfo.Fact
	// continuable reports whether a continue statement with the label of
	// a break statement is valid at the break.
	continuable bool
}

// operatorFacts holds the operand facts of the operators of a parsed file by
// position.
type operatorFacts map[token.Pos]typeinfo.Fact

// fileScope is the information about a parsed file that the specs need to
// select its candidates.
type fileScope struct {
	// facts holds the operand facts of the operators of the file.
	facts operatorFacts
	// pkg holds the package-level names of the package of the file and the
	// import names of the file. A nil set means unknown names.
	pkg pkgNames
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
	if n.fn != nil {
		return token.FUNC
	}

	return *n.tok
}

// SetTok sets the token.Token of the tokenNode. A function candidate has no
// token to set.
func (n *NodeToken) SetTok(t token.Token) {
	*n.tok = t
}

// newFuncToken returns the function candidate of decl in a file where the
// names pkg hide predeclared identifiers.
func newFuncToken(decl *ast.FuncDecl, pkg pkgNames) *NodeToken {
	return &NodeToken{fn: newFuncCandidate(decl, pkg), TokPos: decl.Pos()}
}

// coverPos returns the position that the coverage profile must show to cover
// the mutants of n. For a function, it is the first statement of the body,
// where the first coverage block of the body starts.
func (n *NodeToken) coverPos() token.Pos {
	if n.fn != nil && len(n.fn.decl.Body.List) > 0 {
		return n.fn.decl.Body.List[0].Pos()
	}

	return n.TokPos
}

// snippetPos returns the position of the line at the center of the snippets
// of the mutants of n. For a function, it is the opening brace of the body,
// so the snippets show the added statement also after a long signature.
func (n *NodeToken) snippetPos() token.Pos {
	if n.fn != nil {
		return n.fn.decl.Body.Lbrace
	}

	return n.TokPos
}

// span returns the start and the end of the source of n that the diff
// selection checks. For a function, it is the full declaration.
func (n *NodeToken) span() (token.Pos, token.Pos) {
	if n.fn != nil {
		return n.fn.decl.Pos(), n.fn.decl.End()
	}

	return n.TokPos, n.TokPos
}

// inspectTokens calls fn for each node of root that NewTokenNode supports, and
// for each function declaration with a body, in depth-first order. Each node
// gets its operand facts and the package names from scope, and its loop
// context. A function declaration comes before the operators of its body.
// inspectTokens does not go into the type constraints of type parameters and
// interfaces. An operator in a constraint, for example the | of a type union,
// is a type term and not an operation. fn returns false to stop the walk.
// inspectTokens returns false if fn stopped the walk.
func inspectTokens(root ast.Node, scope fileScope, fn func(*NodeToken) bool) bool {
	constraints := make(map[ast.Expr]struct{})
	stopped := false
	ast.PreorderStack(root, nil, func(n ast.Node, stack []ast.Node) bool {
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
		if decl, isFunc := n.(*ast.FuncDecl); isFunc && decl.Body != nil {
			t, ok = newFuncToken(decl, scope.pkg), true
		}
		if !ok {
			return true
		}
		t.facts = scope.facts[t.TokPos]
		if b, isBranch := n.(*ast.BranchStmt); isBranch && b.Tok == token.BREAK {
			t.continuable = continueValid(b, stack)
		}
		stopped = !fn(t)

		return !stopped
	})

	return !stopped
}

// continueValid reports whether a continue statement with the label of the
// break statement b is valid where b is. stack holds the ancestors of b.
// Without a label, continue needs a loop around it in the same function.
// With a label, the label must name a loop around it.
func continueValid(b *ast.BranchStmt, stack []ast.Node) bool {
	for i := len(stack) - 1; i >= 0; i-- {
		switch n := stack[i].(type) {
		case *ast.FuncLit, *ast.FuncDecl:
			return false
		case *ast.ForStmt, *ast.RangeStmt:
			if b.Label == nil {
				return true
			}
		case *ast.LabeledStmt:
			if b.Label != nil && n.Label.Name == b.Label.Name {
				return isLoop(n.Stmt)
			}
		}
	}

	return false
}

// isLoop reports whether s is a for statement or a for range statement.
func isLoop(s ast.Stmt) bool {
	switch s.(type) {
	case *ast.ForStmt, *ast.RangeStmt:
		return true
	default:
		return false
	}
}

// unary reports whether the operator of n is the operator of a unary
// expression.
func (n *NodeToken) unary() bool {
	return n.operand != nil
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
