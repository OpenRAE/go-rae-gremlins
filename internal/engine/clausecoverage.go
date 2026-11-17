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
	"path"
	"slices"

	"github.com/OpenRAE/go-rae-gremlins/internal/coverage"
)

// fileCoverage tells if the tests executed a token position of one parsed file.
//
// A Go coverage profile has a block for each switch and select clause body,
// but no block for the clause expressions. fileCoverage adds the clause
// expressions that the tests evaluated. It derives them from the clause
// bodies that ran and from the Go evaluation order of the statements.
//
// fileCoverage also tells if the coverage profile can show a token position.
// The profile has no block for a file that the host build excludes. It also
// has no block for the code that the compiler evaluates outside function
// bodies: package-level constant and type declarations, and the signatures of
// functions, methods, and function literals.
//
// The profile also has no block for the initializers of package-level
// variables. Go runs these initializers when it initializes the package,
// before any other code of the package. Thus an executed block in a file of
// the package proves that the initializers ran.
type fileCoverage struct {
	set         *token.FileSet
	profile     coverage.Profile
	clauseExprs []exprSpan
	compiled    []exprSpan
	varDecls    []exprSpan
	varFuncBody []exprSpan
	built       bool
	pkgRan      bool
}

// exprSpan is the source range [start, end) of an expression.
type exprSpan struct {
	start token.Pos
	end   token.Pos
}

// newFileCoverage returns the coverage of file. built tells if the host build
// includes the file.
func newFileCoverage(set *token.FileSet, file *ast.File, profile coverage.Profile, built bool) fileCoverage {
	c := fileCoverage{set: set, profile: profile, built: built}
	c.pkgRan = profile.IsDirCovered(path.Dir(set.Position(file.Package).Filename))
	for _, decl := range file.Decls {
		addDecl(&c, decl)
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.SwitchStmt:
			c.clauseExprs = append(c.clauseExprs, c.evaluatedCaseExprs(n.Body)...)
		case *ast.SelectStmt:
			c.clauseExprs = append(c.clauseExprs, c.evaluatedCommExprs(n.Body)...)
		default:
			// Other nodes have no clause expressions.
		}

		return true
	})

	return c
}

func (c fileCoverage) isCovered(pos token.Pos) bool {
	if c.profile.IsCovered(c.set.Position(pos)) {
		return true
	}
	if c.pkgRan && c.inVarInitializer(pos) {
		return true
	}

	return within(c.clauseExprs, pos)
}

// isObservable tells if a test run can cover pos. The tests of the host build
// cannot run a file that the build excludes. The compiler evaluates the code
// in the compiled spans, so no test run executes it. The profile shows a
// package-level variable initializer only when the tests ran other code of
// the package.
func (c fileCoverage) isObservable(pos token.Pos) bool {
	switch {
	case !c.built, within(c.compiled, pos):
		return false
	case c.inVarInitializer(pos):
		return c.pkgRan
	default:
		return true
	}
}

// addDecl records in c the spans of a package-level declaration that the
// profile cannot show directly.
func addDecl(c *fileCoverage, decl ast.Decl) {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		c.compiled = append(c.compiled, signatureSpan(d.Pos(), d.End(), d.Body))
	case *ast.GenDecl:
		addGenDecl(c, d)
	default:
		// A bad declaration from a syntax error has no spans.
	}
}

// addGenDecl records in c the spans of a package-level constant, type, or
// variable declaration. Import declarations have no spans.
func addGenDecl(c *fileCoverage, d *ast.GenDecl) {
	if d.Tok == token.CONST || d.Tok == token.TYPE {
		c.compiled = append(c.compiled, spanOf(d))
	}
	if d.Tok != token.VAR {
		return
	}
	c.varDecls = append(c.varDecls, spanOf(d))
	for _, f := range outerFuncLits(d) {
		c.compiled = append(c.compiled, signatureSpan(f.Pos(), f.End(), f.Body))
		c.varFuncBody = append(c.varFuncBody, spanOf(f.Body))
	}
}

// inVarInitializer tells if pos is in a package-level variable declaration
// and not in the body of a function literal of it. A function literal body
// has its own coverage blocks.
func (c fileCoverage) inVarInitializer(pos token.Pos) bool {
	return within(c.varDecls, pos) && !within(c.varFuncBody, pos)
}

// signatureSpan returns the span of a function signature from start to the
// opening brace of body. A function without a body is all signature.
func signatureSpan(start, end token.Pos, body *ast.BlockStmt) exprSpan {
	if body != nil {
		end = body.Lbrace
	}

	return exprSpan{start: start, end: end}
}

// outerFuncLits returns the outermost function literals of a node.
func outerFuncLits(n ast.Node) []*ast.FuncLit {
	var lits []*ast.FuncLit
	ast.Inspect(n, func(node ast.Node) bool {
		if f, ok := node.(*ast.FuncLit); ok {
			lits = append(lits, f)

			return false
		}

		return true
	})

	return lits
}

func spanOf(n ast.Node) exprSpan {
	return exprSpan{start: n.Pos(), end: n.End()}
}

func within(spans []exprSpan, pos token.Pos) bool {
	return slices.ContainsFunc(spans, func(s exprSpan) bool { return pos >= s.start && pos < s.end })
}

// bodyRan tells if the body of the clause with the given colon ran.
// The coverage block of a clause body starts immediately after the colon,
// also when the body is empty.
func (c fileCoverage) bodyRan(colon token.Pos) bool {
	return c.profile.IsCovered(c.set.Position(colon + 1))
}

// evaluatedCaseExprs returns the case expressions of an expression switch
// that the tests evaluated.
//
// Go evaluates the case expressions from top to bottom and from left to right,
// and stops at the first match. The default clause runs only when no case
// matches. A clause body can also run because the previous clause body ran
// and ends with fallthrough. Thus a body that ran proves only that the first
// clause of its fallthrough chain, the entry clause, was reached:
//   - all expressions of the clauses before the entry clause were evaluated,
//   - the first expression of the entry clause was evaluated,
//   - all case expressions were evaluated if the entry clause is the default
//     clause and the body that ran is its own.
func (c fileCoverage) evaluatedCaseExprs(body *ast.BlockStmt) []exprSpan {
	clauses := clausesOf[*ast.CaseClause](body)
	full, first := c.caseEvidence(clauses)

	var spans []exprSpan
	for k, cc := range clauses {
		switch {
		case k < full:
			spans = append(spans, spansOf(cc.List)...)
		case first[k] && len(cc.List) > 0:
			spans = append(spans, spansOf(cc.List[:1])...)
		default:
			// The tests did not prove that this clause was reached.
		}
	}

	return spans
}

// caseEvidence returns the number of leading clauses with all expressions
// evaluated, and for each clause if its first expression was evaluated.
func (c fileCoverage) caseEvidence(clauses []*ast.CaseClause) (int, []bool) {
	full := 0
	first := make([]bool, len(clauses))
	for j, cc := range clauses {
		if !c.bodyRan(cc.Colon) {
			continue
		}
		entry := c.entryClause(clauses, j)
		if entry == j && cc.List == nil {
			return len(clauses), first
		}
		full = max(full, entry)
		first[entry] = true
	}

	return full, first
}

// entryClause returns the index of the first clause of the fallthrough chain
// that ends at clause j.
func (c fileCoverage) entryClause(clauses []*ast.CaseClause, j int) int {
	entry := j
	for entry > 0 && c.fellThrough(clauses[entry-1]) {
		entry--
	}

	return entry
}

// fellThrough tells if the body of the clause ran and ends with fallthrough.
func (c fileCoverage) fellThrough(cc *ast.CaseClause) bool {
	if len(cc.Body) == 0 || !c.bodyRan(cc.Colon) {
		return false
	}
	br, ok := cc.Body[len(cc.Body)-1].(*ast.BranchStmt)

	return ok && br.Tok == token.FALLTHROUGH
}

// evaluatedCommExprs returns the communication expressions of a select
// statement that the tests evaluated.
//
// On entry to a select statement, Go evaluates the channel operands of all
// receive operations and the channel and value expressions of all send
// statements. Thus a clause body that ran proves that all of them were
// evaluated. Go evaluates the left side of a receive assignment only for the
// selected clause.
func (c fileCoverage) evaluatedCommExprs(body *ast.BlockStmt) []exprSpan {
	clauses := clausesOf[*ast.CommClause](body)
	ran := slices.ContainsFunc(clauses, func(cc *ast.CommClause) bool { return c.bodyRan(cc.Colon) })
	if !ran {
		return nil
	}

	var spans []exprSpan
	for _, cc := range clauses {
		spans = append(spans, c.commSpans(cc)...)
	}

	return spans
}

func (c fileCoverage) commSpans(cc *ast.CommClause) []exprSpan {
	switch s := cc.Comm.(type) {
	case *ast.SendStmt, *ast.ExprStmt:
		return []exprSpan{{start: s.Pos(), end: s.End()}}
	case *ast.AssignStmt:
		return c.receiveSpans(s, cc.Colon)
	default:
		// The default clause has no communication.
		return nil
	}
}

// receiveSpans returns the evaluated expressions of a receive assignment.
func (c fileCoverage) receiveSpans(s *ast.AssignStmt, colon token.Pos) []exprSpan {
	spans := spansOf(s.Rhs)
	if c.bodyRan(colon) {
		spans = append(spans, spansOf(s.Lhs)...)
	}

	return spans
}

func clausesOf[T ast.Stmt](body *ast.BlockStmt) []T {
	var clauses []T
	for _, stmt := range body.List {
		if cc, ok := stmt.(T); ok {
			clauses = append(clauses, cc)
		}
	}

	return clauses
}

func spansOf(exprs []ast.Expr) []exprSpan {
	spans := make([]exprSpan, 0, len(exprs))
	for _, e := range exprs {
		spans = append(spans, exprSpan{start: e.Pos(), end: e.End()})
	}

	return spans
}
