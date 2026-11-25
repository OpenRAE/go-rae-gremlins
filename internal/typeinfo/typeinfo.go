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

// Package typeinfo finds the operand facts of a Go module that mutant
// eligibility needs, from the types of its packages.
package typeinfo

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/tools/go/packages"

	"github.com/OpenRAE/go-rae-gremlins/internal/log"
)

// Fact is a set of properties of the operands of an operator.
type Fact uint8

const (
	// InvalidSubtraction means that the - form of a + or += operator does not
	// type-check, for example because the operands are strings.
	InvalidSubtraction Fact = 1 << iota
	// UnassignableShiftCount means that the count of a <<= or >>= operator
	// has a type that cannot be assigned to the left operand.
	UnassignableShiftCount
)

// Has reports whether f contains all the properties of g.
func (f Fact) Has(g Fact) bool {
	return f&g == g
}

// Facts holds the facts of the operators of a module. The key of the outer
// map is the slash-separated file name relative to the module root. The key
// of the inner map is the byte offset of the operator in the file.
type Facts map[string]map[int]Fact

// File returns the facts of the operators of the file name, which is
// slash-separated and relative to the module root. It returns nil for a file
// without facts.
func (f Facts) File(name string) map[int]Fact {
	return f[name]
}

const loadMode = packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
	packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo

// Load type-checks the packages that pattern matches in the module at root,
// with the build tags and the Go build cache buildCache, and returns their
// operand facts. It does not load test files. A package with errors gives no
// facts, and Load logs it. Load returns an error when the go command cannot
// list the packages, or when ctx is done.
func Load(ctx context.Context, root, pattern, tags, buildCache string) (Facts, error) {
	cfg := &packages.Config{
		Context: ctx,
		Mode:    loadMode,
		Dir:     root,
		// exec.Cmd uses the last value of a duplicate variable, so GOCACHE
		// replaces the value in the environment of Gremlins.
		Env: append(os.Environ(), "GOCACHE="+buildCache),
		// The go test command does not stamp VCS information, so a VCS
		// error must not fail the load of a module that go test can test.
		BuildFlags: buildFlags(tags),
	}
	log.Infof("Loading package types... ")
	start := time.Now()
	pkgs, err := packages.Load(cfg, pattern)
	if err != nil {
		return nil, fmt.Errorf("loading the package types: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	log.Infof("done in %s\n", time.Since(start))

	facts := make(Facts)
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			log.Infof("Type information of %s is not complete: %v\n", pkg.PkgPath, pkg.Errors[0])

			continue
		}
		addPackage(facts, root, pkg)
	}

	return facts, nil
}

func buildFlags(tags string) []string {
	flags := []string{"-buildvcs=false"}
	if tags != "" {
		flags = append(flags, "-tags", tags)
	}

	return flags
}

// addPackage adds the facts of the files of pkg to facts. It skips a file
// outside root, for example a file that cgo generates in the build cache.
func addPackage(facts Facts, root string, pkg *packages.Package) {
	for _, file := range pkg.Syntax {
		tf := pkg.Fset.File(file.Pos())
		rel, err := filepath.Rel(root, tf.Name())
		if err != nil || !filepath.IsLocal(rel) {
			continue
		}
		if f := fileFacts(pkg, tf, file); len(f) > 0 {
			facts[filepath.ToSlash(rel)] = f
		}
	}
}

// fileFacts returns the facts of the operators of file by byte offset.
func fileFacts(pkg *packages.Package, tf *token.File, file *ast.File) map[int]Fact {
	facts := make(map[int]Fact)
	add := func(pos token.Pos, f Fact) {
		facts[tf.Offset(pos)] |= f
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.BinaryExpr:
			if n.Op == token.ADD && !subtractionChecks(pkg, n.OpPos, n.X, n.Y) {
				add(n.OpPos, InvalidSubtraction)
			}
		case *ast.AssignStmt:
			assignFacts(pkg, n, add)
		}

		return true
	})

	return facts
}

// assignFacts adds the facts of the assignment operator of a. Other
// assignment operators need no facts.
func assignFacts(pkg *packages.Package, a *ast.AssignStmt, add func(token.Pos, Fact)) {
	if len(a.Lhs) != 1 || len(a.Rhs) != 1 {
		return
	}
	if a.Tok == token.ADD_ASSIGN && !subtractionChecks(pkg, a.TokPos, a.Lhs[0], a.Rhs[0]) {
		add(a.TokPos, InvalidSubtraction)
	}
	if (a.Tok == token.SHL_ASSIGN || a.Tok == token.SHR_ASSIGN) && !countAssignable(pkg.TypesInfo, a.Lhs[0], a.Rhs[0]) {
		add(a.TokPos, UnassignableShiftCount)
	}
}

// subtractionChecks reports whether x - y type-checks at pos, where x + y is
// in the source. The type checker evaluates the type sets of type parameters,
// so it also handles generic operands. It gives true when x + y alone does not
// type-check, for example with iota, because the result is then unknown and
// the spec keeps its earlier selection.
func subtractionChecks(pkg *packages.Package, pos token.Pos, x, y ast.Expr) bool {
	check := func(op token.Token) error {
		return types.CheckExpr(pkg.Fset, pkg.Types, pos, &ast.BinaryExpr{X: x, OpPos: pos, Op: op, Y: y}, nil)
	}
	if check(token.ADD) != nil {
		return true
	}

	return check(token.SUB) == nil
}

// countAssignable reports whether the shift count can be assigned to the left
// operand of a shift assignment. An untyped constant count, or a count of
// unknown type, gives true: the spec then keeps its earlier selection.
func countAssignable(info *types.Info, left, count ast.Expr) bool {
	tv, ok := info.Types[count]
	lt := info.TypeOf(left)
	if !ok || tv.Type == nil || lt == nil || isUntyped(tv.Type) {
		return true
	}

	return types.AssignableTo(tv.Type, lt)
}

// isUntyped reports whether t is the type of an untyped constant.
func isUntyped(t types.Type) bool {
	b, ok := t.(*types.Basic)

	return ok && b.Info()&types.IsUntyped != 0
}
