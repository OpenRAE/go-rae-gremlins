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

// evaluatedFuncLitTails returns the code of the statements in list that the
// tests evaluated and that the coverage profile has no block for.
//
// The Go cover tool ends the coverage block of a statement at the body of its
// first function literal. Thus the statement code after that body is in no
// block. Go evaluates this code with the statement, and the cover tool counts
// a statement as one unit. Thus the code is evaluated when the block of the
// statement start ran. Each function literal body has its own blocks, so
// isCovered does not use the returned spans for a function literal body.
func (c fileCoverage) evaluatedFuncLitTails(list []ast.Stmt) []exprSpan {
	var spans []exprSpan
	for _, s := range list {
		for l, ok := s.(*ast.LabeledStmt); ok; l, ok = s.(*ast.LabeledStmt) {
			s = l.Stmt
		}
		cut, end := funcLitCut(s)
		if cut == token.NoPos || !c.profile.IsCovered(c.set.Position(s.Pos())) {
			continue
		}
		spans = append(spans, exprSpan{start: cut, end: end})
	}

	return spans
}

// funcLitCut returns the position where the Go cover tool ends the coverage
// block of s at a function literal body, and the end of the code that the
// block of s reaches without a function literal. cut is token.NoPos when the
// block of s does not end at a function literal. The cases follow
// statementBoundary of cmd/cover.
func funcLitCut(s ast.Stmt) (cut, end token.Pos) {
	switch s := s.(type) {
	case *ast.IfStmt:
		return firstFuncLitBody(s.Init, s.Cond), s.Body.Lbrace
	case *ast.ForStmt:
		return firstFuncLitBody(s.Init, s.Cond, s.Post), s.Body.Lbrace
	case *ast.RangeStmt:
		return firstFuncLitBody(s.X), s.Body.Lbrace
	case *ast.SwitchStmt:
		return firstFuncLitBody(s.Init, s.Tag), s.Body.Lbrace
	case *ast.TypeSwitchStmt:
		return firstFuncLitBody(s.Init), s.Body.Lbrace
	case *ast.BlockStmt, *ast.SelectStmt, *ast.CaseClause, *ast.CommClause:
		return token.NoPos, token.NoPos
	default:
		return firstFuncLitBody(s), s.End()
	}
}

// firstFuncLitBody returns the opening brace of the body of the first
// function literal in nodes, or token.NoPos when nodes have none.
func firstFuncLitBody(nodes ...ast.Node) token.Pos {
	for _, n := range nodes {
		if n == nil {
			continue
		}
		if lits := outerFuncLits(n); len(lits) > 0 {
			return lits[0].Body.Lbrace
		}
	}

	return token.NoPos
}
