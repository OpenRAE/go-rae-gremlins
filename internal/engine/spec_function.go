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
	"go/constant"
	"go/token"
	"slices"

	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
)

// extremeSpec is a Descartes spec of ADR 0012. It adds a return statement
// before the statements of a function body. The original statements stay as
// unreachable code, so the imports and variables that they use stay in use.
type extremeSpec struct {
	// values returns the result values of the added return statement of fn.
	// It returns false when the type cannot mutate fn.
	values func(fn *funcCandidate) ([]ast.Expr, bool)
	mt     mutator.Type
}

func (s extremeSpec) mutationType() mutator.Type {
	return s.mt
}

func (extremeSpec) tokens() []token.Token {
	return []token.Token{token.FUNC}
}

func (s extremeSpec) eligible(n *NodeToken) bool {
	_, ok := s.returnValues(n)

	return ok
}

func (s extremeSpec) mutate(n *NodeToken) (func(), error) {
	values, ok := s.returnValues(n)
	if !ok {
		return nil, noMutation(s.mt, n.Tok())
	}
	body := n.fn.decl.Body
	orig := body.List
	body.List = append([]ast.Stmt{&ast.ReturnStmt{Results: values}}, orig...)

	return func() { body.List = orig }, nil
}

// returnValues returns the values of the added return statement for the
// function candidate n. It returns false when n is not a function candidate,
// when the type cannot mutate the function, and when the body already is the
// mutation.
func (s extremeSpec) returnValues(n *NodeToken) ([]ast.Expr, bool) {
	fn := n.fn
	if fn == nil || len(fn.decl.Body.List) == 0 {
		return nil, false
	}
	values, ok := s.values(fn)
	if !ok || fn.onlyReturns(values) {
		return nil, false
	}

	return values, true
}

// voidValues gives the EXTREME_VOID mutation of a function without results:
// a return statement without values.
func voidValues(fn *funcCandidate) ([]ast.Expr, bool) {
	return nil, len(fn.results) == 0
}

// zeroValues gives the EXTREME_ZERO mutation: the zero value of each result.
func zeroValues(fn *funcCandidate) ([]ast.Expr, bool) {
	return fn.literals(zeroLiterals)
}

// constantValues gives the EXTREME_CONSTANT mutation: true, 1, or "A" for
// each basic result and nil for each nilable result. One or more results must
// be basic.
func constantValues(fn *funcCandidate) ([]ast.Expr, bool) {
	if !slices.ContainsFunc(fn.results, isBasic) {
		return nil, false
	}

	return fn.literals(constantLiterals)
}

// literals returns the value of each result of fn from table. It returns
// false when fn has no results, when table has no value for a result kind,
// and when a shadowed name hides a predeclared value identifier.
func (fn *funcCandidate) literals(table map[resultKind]literal) ([]ast.Expr, bool) {
	if len(fn.results) == 0 {
		return nil, false
	}
	values := make([]ast.Expr, 0, len(fn.results))
	for _, kind := range fn.results {
		l, ok := table[kind]
		if !ok || (l.kind == token.IDENT && !fn.predeclared(l.value)) {
			return nil, false
		}
		values = append(values, l.expr())
	}

	return values, true
}

// literal is the Go source of a result value: a predeclared identifier or a
// basic literal.
type literal struct {
	value string
	kind  token.Token
}

// expr returns a new expression of l. It has no position.
func (l literal) expr() ast.Expr {
	if l.kind == token.IDENT {
		return ast.NewIdent(l.value)
	}

	return &ast.BasicLit{Kind: l.kind, Value: l.value}
}

// zeroLiterals holds the zero value of each kind of result that has one.
var zeroLiterals = map[resultKind]literal{
	nilableResult: {kind: token.IDENT, value: "nil"},
	boolResult:    {kind: token.IDENT, value: "false"},
	numberResult:  {kind: token.INT, value: "0"},
	stringResult:  {kind: token.STRING, value: `""`},
}

// constantLiterals holds the second Descartes constant of each basic kind of
// result, and nil for a nilable result.
var constantLiterals = map[resultKind]literal{
	nilableResult: {kind: token.IDENT, value: "nil"},
	boolResult:    {kind: token.IDENT, value: "true"},
	numberResult:  {kind: token.INT, value: "1"},
	stringResult:  {kind: token.STRING, value: `"A"`},
}

// onlyReturns reports whether the body of fn is one return statement of the
// values. A mutant of such a body is equivalent. A return statement without
// values in a function with results returns the zero values, because no
// other statement sets the results.
func (fn *funcCandidate) onlyReturns(values []ast.Expr) bool {
	list := fn.decl.Body.List
	if len(list) != 1 {
		return false
	}
	ret, ok := list[0].(*ast.ReturnStmt)
	if !ok {
		return false
	}
	results := ret.Results
	if len(results) == 0 {
		results, _ = zeroValues(fn)
	}

	return slices.EqualFunc(results, values, fn.sameValue)
}

// sameValue reports whether the expression got, without its parentheses, is
// the literal value want: the same predeclared identifier, or a literal of
// the same constant value.
func (fn *funcCandidate) sameValue(got, want ast.Expr) bool {
	got = ast.Unparen(got)
	switch w := want.(type) {
	case *ast.Ident:
		g, ok := got.(*ast.Ident)

		return ok && g.Name == w.Name && fn.predeclared(g.Name)
	case *ast.BasicLit:
		g, ok := got.(*ast.BasicLit)

		return ok && sameConstant(g, w)
	default:
		return false
	}
}

// sameConstant reports whether two literals have the same constant value. A
// number and a string are never the same.
func sameConstant(a, b *ast.BasicLit) bool {
	x := constant.MakeFromLiteral(a.Value, a.Kind, 0)
	y := constant.MakeFromLiteral(b.Value, b.Kind, 0)
	if isNumber(x) != isNumber(y) || x.Kind() == constant.Unknown || y.Kind() == constant.Unknown {
		return false
	}

	return constant.Compare(x, token.EQL, y)
}

func isNumber(v constant.Value) bool {
	switch v.Kind() {
	case constant.Int, constant.Float, constant.Complex:
		return true
	case constant.Unknown, constant.Bool, constant.String:
		return false
	default:
		return false
	}
}

// isBasic reports whether k is the kind of a bool, numeric, or string result.
func isBasic(k resultKind) bool {
	return k == boolResult || k == numberResult || k == stringResult
}
