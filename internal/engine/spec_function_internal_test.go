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
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
)

// extremeSource has one function for each case of the eligibility rules of
// ADR 0012. It compiles. It imports a package that only one function body
// uses, so a mutant that removed the body would not compile.
const extremeSource = `package p

import "strings"

type named int

type T struct{}

func noResults(x int) {
	println(x)
}

func emptyBody() {}

func bareReturn() {
	return
}

func (t *T) method(x int) {
	println(t, x)
}

func generic[E any](e E) {
	println(e)
}

func usesImport(s string) {
	println(strings.ToUpper(s))
}

func boolResult(a, b int) bool {
	return a < b
}

func intAndError(a int) (int, error) {
	return a, nil
}

func nilables() (*T, []int, map[string]int, chan int, func(), interface{}, error, any) {
	return &T{}, []int{}, nil, nil, nil, nil, nil, nil
}

func namedResults() (s, u string) {
	s = "x"
	return s, s
}

func numbers() (int8, uint64, float32, complex128, byte, rune, uintptr) {
	return 2, 2, 2, 2, 2, 2, 2
}

func namedType() named {
	return 2
}

func structResult() T {
	return T{}
}

func arrayResult() [2]int {
	return [2]int{}
}

func typeParam[E any](e E) E {
	return e
}

func parenthesized() ((*T)) {
	return &T{}
}

func alreadyZero() (int, error) {
	return 0, nil
}

func alreadyTrue() bool {
	return true
}

func alreadyFalse() bool {
	return false
}

func zeroFloat() float64 {
	return 0.0
}

func oneString() string {
	return "A"
}

func namedBare() (n int) {
	return
}

func shadowNil(nil int) *T {
	println(nil)
	return &T{}
}

func shadowType[int any](v int) int {
	return v
}

func parenZero() int {
	return (0)
}

func parenTrue() bool {
	return (true)
}

func bodyless() int

var literal = func() int {
	return 2
}
`

// Names of the functions of extremeSource that several tests use.
const (
	fnNoResults    = "noResults"
	fnBoolResult   = "boolResult"
	fnIntAndError  = "intAndError"
	fnNamedResults = "namedResults"
)

// parseExtreme parses extremeSource. It returns the file set, the file, and
// the package-level names of the file.
func parseExtreme(t *testing.T) (*token.FileSet, *ast.File, pkgNames) {
	t.Helper()

	return parsePackage(t, extremeSource)
}

// parsePackage parses the files srcs of one package. It returns the file set,
// the first file, and the package-level names of all the files.
func parsePackage(t *testing.T, srcs ...string) (*token.FileSet, *ast.File, pkgNames) {
	t.Helper()
	set := token.NewFileSet()
	names := pkgNames{}
	var first *ast.File
	for i, src := range srcs {
		file, err := parser.ParseFile(set, fmt.Sprintf("p%d.go", i), src, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse file %d: %v", i, err)
		}
		addPackageNames(names, file)
		if first == nil {
			first = file
		}
	}

	return set, first, names
}

// funcCandidates returns the function candidates of file by function name.
func funcCandidates(file *ast.File, names pkgNames) map[string]*NodeToken {
	got := make(map[string]*NodeToken)
	inspectTokens(file, fileScope{pkg: names}, func(n *NodeToken) bool {
		if n.fn != nil {
			got[n.fn.decl.Name.Name] = n
		}

		return true
	})

	return got
}

// TestFeatureExtremeSpecEligibility checks the extreme types that each
// function of extremeSource can produce, in registration order. The expected
// types come from the eligibility table of ADR 0012.
func TestFeatureExtremeSpecEligibility(t *testing.T) {
	t.Parallel()

	void := []mutator.Type{mutator.ExtremeVoid}
	both := []mutator.Type{mutator.ExtremeZero, mutator.ExtremeConstant}
	zero := []mutator.Type{mutator.ExtremeZero}
	constant := []mutator.Type{mutator.ExtremeConstant}
	want := map[string][]mutator.Type{
		fnNoResults:     void,
		"emptyBody":     nil,
		"bareReturn":    nil,
		"method":        void,
		"generic":       void,
		"usesImport":    void,
		fnBoolResult:    both,
		fnIntAndError:   both,
		"nilables":      zero,
		fnNamedResults:  both,
		"numbers":       both,
		"namedType":     nil,
		"structResult":  nil,
		"arrayResult":   nil,
		"typeParam":     nil,
		"parenthesized": zero,
		"alreadyZero":   constant,
		"alreadyTrue":   zero,
		"alreadyFalse":  constant,
		"zeroFloat":     constant,
		"oneString":     zero,
		"namedBare":     constant,
		"shadowNil":     nil,
		"shadowType":    nil,
		"parenZero":     constant,
		"parenTrue":     zero,
	}

	_, file, names := parseExtreme(t)
	candidates := funcCandidates(file, names)
	for name, wantTypes := range want {
		n, ok := candidates[name]
		if !ok {
			t.Errorf("function %s has no candidate, want one", name)

			continue
		}
		var got []mutator.Type
		for mt := range builtins.eligibleTypes(n) {
			got = append(got, mt)
		}
		if diff := cmp.Diff(wantTypes, got); diff != "" {
			t.Errorf("eligible types of %s mismatch (-want +got):\n%s", name, diff)
		}
	}
	for name := range candidates {
		if _, ok := want[name]; !ok {
			t.Errorf("function %s has a candidate, want none: a declaration without a body is not a candidate", name)
		}
	}
}

// TestFeatureExtremeSpecPackageShadowing checks that a package-level name in
// another file of the package hides a predeclared identifier: float32 is
// then not a basic type, and EXTREME_CONSTANT cannot return true.
func TestFeatureExtremeSpecPackageShadowing(t *testing.T) {
	t.Parallel()

	const src = `package p

func shadowType() float32 {
	return 2
}

func shadowTrue() bool {
	return 2 > 1
}

func notShadowed() string {
	return "x"
}
`
	const shadow = `package p

type float32 struct{}

var true = 2 > 1
`
	want := map[string][]mutator.Type{
		"shadowType":  nil,
		"shadowTrue":  {mutator.ExtremeZero},
		"notShadowed": {mutator.ExtremeZero, mutator.ExtremeConstant},
	}
	_, file, names := parsePackage(t, src, shadow)
	candidates := funcCandidates(file, names)
	for name, wantTypes := range want {
		var got []mutator.Type
		for mt := range builtins.eligibleTypes(candidates[name]) {
			got = append(got, mt)
		}
		if diff := cmp.Diff(wantTypes, got); diff != "" {
			t.Errorf("eligible types of %s mismatch (-want +got):\n%s", name, diff)
		}
	}
}

// TestFeatureExtremeSpecImportShadowing checks that an import name of the
// file hides a predeclared identifier: EXTREME_ZERO cannot return false when
// the file imports a package as false. The EXTREME_CONSTANT mutant of the
// same function type-checks.
func TestFeatureExtremeSpecImportShadowing(t *testing.T) {
	t.Parallel()

	const src = `package p

import false "fmt"

func ok() bool {
	return false.Sprint() == ""
}
`
	set, file, names := parsePackage(t, src)
	n := funcCandidates(file, fileNames(names, file))["ok"]
	var got []mutator.Type
	for mt := range builtins.eligibleTypes(n) {
		got = append(got, mt)
	}
	if diff := cmp.Diff([]mutator.Type{mutator.ExtremeConstant}, got); diff != "" {
		t.Fatalf("eligible types of ok mismatch (-want +got):\n%s", diff)
	}

	restore, err := builtins.byType[mutator.ExtremeConstant].mutate(n)
	if err != nil {
		t.Fatalf("EXTREME_CONSTANT mutate(ok) error = %v, want nil", err)
	}
	var b bytes.Buffer
	err = printer.Fprint(&b, set, file)
	restore()
	if err != nil {
		t.Fatalf("print mutant: %v", err)
	}
	checkSet := token.NewFileSet()
	if err := typeCheck(checkSet, importer.ForCompiler(checkSet, "source", nil), b.String()); err != nil {
		t.Errorf("EXTREME_CONSTANT mutant of ok does not type-check: %v\n%s", err, b.String())
	}
}

// TestFeatureUnknownPackageNamesHidePredeclaredIdentifiers checks that a
// nil set of package names, which means that the names are not known, lets
// no predeclared identifier count. Only EXTREME_VOID, which uses no name,
// stays eligible.
func TestFeatureUnknownPackageNamesHidePredeclaredIdentifiers(t *testing.T) {
	t.Parallel()

	_, file, _ := parseExtreme(t)
	candidates := funcCandidates(file, nil)
	for name, wantVoid := range map[string]bool{fnNoResults: true, fnBoolResult: false, fnIntAndError: false} {
		var got []mutator.Type
		for mt := range builtins.eligibleTypes(candidates[name]) {
			got = append(got, mt)
		}
		var want []mutator.Type
		if wantVoid {
			want = []mutator.Type{mutator.ExtremeVoid}
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("eligible types of %s with unknown names mismatch (-want +got):\n%s", name, diff)
		}
	}
}

// TestFeatureExtremeSpecIgnoresFunctionLiterals checks that a function
// literal is not a candidate of an extreme type.
func TestFeatureExtremeSpecIgnoresFunctionLiterals(t *testing.T) {
	t.Parallel()

	_, file, names := parseExtreme(t)
	var lit *ast.FuncLit
	ast.Inspect(file, func(n ast.Node) bool {
		if l, ok := n.(*ast.FuncLit); ok {
			lit = l
		}

		return lit == nil
	})
	if lit == nil {
		t.Fatal("extremeSource has no function literal")
	}
	inspectTokens(lit, fileScope{pkg: names}, func(n *NodeToken) bool {
		if n.fn != nil {
			t.Errorf("function literal at %d has a function candidate, want none", lit.Pos())
		}

		return true
	})
}

// printDecl returns the source of the declaration of n.
func printDecl(t *testing.T, set *token.FileSet, n *NodeToken) string {
	t.Helper()
	var b bytes.Buffer
	if err := printer.Fprint(&b, set, n.fn.decl); err != nil {
		t.Fatalf("print %s: %v", n.fn.decl.Name.Name, err)
	}

	return b.String()
}

// TestFeatureExtremeSpecMutatesAndRestores checks the added return statement
// of each extreme type and the restoration of the original statements.
func TestFeatureExtremeSpecMutatesAndRestores(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		fn   string
		want string
		mt   mutator.Type
	}{
		{
			name: "void", fn: fnNoResults, mt: mutator.ExtremeVoid,
			want: "func noResults(x int) {\n\treturn\n\tprintln(x)\n}",
		},
		{
			name: "zero of int and error", fn: fnIntAndError, mt: mutator.ExtremeZero,
			want: "func intAndError(a int) (int, error) {\n\treturn 0, nil\n\treturn a, nil\n}",
		},
		{
			name: "constant of int and error", fn: fnIntAndError, mt: mutator.ExtremeConstant,
			want: "func intAndError(a int) (int, error) {\n\treturn 1, nil\n\treturn a, nil\n}",
		},
		{
			name: "zero of bool", fn: fnBoolResult, mt: mutator.ExtremeZero,
			want: "func boolResult(a, b int) bool {\n\treturn false\n\treturn a < b\n}",
		},
		{
			name: "constant of bool", fn: fnBoolResult, mt: mutator.ExtremeConstant,
			want: "func boolResult(a, b int) bool {\n\treturn true\n\treturn a < b\n}",
		},
		{
			name: "zero of named strings", fn: fnNamedResults, mt: mutator.ExtremeZero,
			want: "func namedResults() (s, u string) {\n\treturn \"\", \"\"\n\ts = \"x\"\n\treturn s, s\n}",
		},
		{
			name: "constant of named strings", fn: fnNamedResults, mt: mutator.ExtremeConstant,
			want: "func namedResults() (s, u string) {\n\treturn \"A\", \"A\"\n\ts = \"x\"\n\treturn s, s\n}",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			set, file, names := parseExtreme(t)
			n := funcCandidates(file, names)[tc.fn]
			if n == nil {
				t.Fatalf("function %s has no candidate", tc.fn)
			}
			orig := printDecl(t, set, n)
			origList := n.fn.decl.Body.List

			restore, err := builtins.byType[tc.mt].mutate(n)
			if err != nil {
				t.Fatalf("%s mutate(%s) error = %v, want nil", tc.mt, tc.fn, err)
			}
			if got := printDecl(t, set, n); got != tc.want {
				t.Errorf("%s mutate(%s) =\n%s\nwant\n%s", tc.mt, tc.fn, got, tc.want)
			}
			restore()
			if got := printDecl(t, set, n); got != orig {
				t.Errorf("%s restore(%s) =\n%s\nwant\n%s", tc.mt, tc.fn, got, orig)
			}
			if !slices.Equal(n.fn.decl.Body.List, origList) {
				t.Errorf("%s restore(%s) did not put back the original statement list", tc.mt, tc.fn)
			}
		})
	}
}

// TestFeatureExtremeSpecRejectsIneligibleFunction checks that mutate returns
// ErrNoMutation and changes nothing for a function that the type cannot
// mutate, and for an operator.
func TestFeatureExtremeSpecRejectsIneligibleFunction(t *testing.T) {
	t.Parallel()

	set, file, names := parseExtreme(t)
	n := funcCandidates(file, names)["structResult"]
	orig := printDecl(t, set, n)
	for _, mt := range []mutator.Type{mutator.ExtremeVoid, mutator.ExtremeZero, mutator.ExtremeConstant} {
		restore, err := builtins.byType[mt].mutate(n)
		if !errors.Is(err, ErrNoMutation) || restore != nil {
			t.Errorf("%s mutate(structResult) = (restore %t, %v), want (no restore, ErrNoMutation)", mt, restore != nil, err)
		}
		if got := printDecl(t, set, n); got != orig {
			t.Errorf("%s mutate(structResult) changed the function:\n%s", mt, got)
		}

		op := &NodeToken{tok: new(token.Token), TokPos: 1}
		*op.tok = token.ADD
		if builtins.byType[mt].eligible(op) {
			t.Errorf("%s eligible(+) = true, want false", mt)
		}
	}
}

// TestFeatureExtremeMutantsTypeCheck checks that each extreme mutant of
// extremeSource gives a file that the Go type checker accepts. The unused
// import case proves that the mutant keeps the original statements.
func TestFeatureExtremeMutantsTypeCheck(t *testing.T) {
	t.Parallel()

	set, file, names := parseExtreme(t)
	checkSet := token.NewFileSet()
	imp := importer.ForCompiler(checkSet, "source", nil)
	mutants := 0
	for name, n := range funcCandidates(file, names) {
		for mt := range builtins.eligibleTypes(n) {
			restore, err := builtins.byType[mt].mutate(n)
			if err != nil {
				t.Fatalf("%s mutate(%s) error = %v, want nil", mt, name, err)
			}
			var b bytes.Buffer
			err = printer.Fprint(&b, set, file)
			restore()
			if err != nil {
				t.Fatalf("print %s mutant of %s: %v", mt, name, err)
			}
			if err := typeCheck(checkSet, imp, b.String()); err != nil {
				t.Errorf("%s mutant of %s does not type-check: %v\n%s", mt, name, err, b.String())
			}
			mutants++
		}
	}
	if mutants == 0 {
		t.Fatal("extremeSource gave no extreme mutant")
	}
}

// typeCheck type-checks src as package p. It drops the declaration without a
// body, which the type checker rejects without an assembly file.
func typeCheck(set *token.FileSet, imp types.Importer, src string) error {
	file, err := parser.ParseFile(set, "p.go", src, 0)
	if err != nil {
		return err
	}
	file.Decls = slices.DeleteFunc(file.Decls, func(d ast.Decl) bool {
		f, ok := d.(*ast.FuncDecl)

		return ok && f.Body == nil
	})
	conf := types.Config{Importer: imp}
	_, err = conf.Check("p", set, []*ast.File{file}, nil)

	return err
}

// TestFeatureFunctionCandidateLocations checks the anchor, the coverage
// position, and the diff range of a function candidate.
func TestFeatureFunctionCandidateLocations(t *testing.T) {
	t.Parallel()

	set, file, names := parseExtreme(t)
	n := funcCandidates(file, names)[fnNamedResults]
	pos := func(p token.Pos) string { return set.Position(p).String() }
	start, end := n.span()
	got := []string{pos(n.TokPos), pos(n.coverPos()), pos(start), pos(end)}
	want := []string{"p0.go:43:1", "p0.go:44:2", "p0.go:43:1", "p0.go:46:2"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("anchor, coverage, start, end of namedResults mismatch (-want +got):\n%s", diff)
	}
}
