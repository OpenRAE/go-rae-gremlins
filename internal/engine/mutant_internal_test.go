/*
 * Copyright 2024 The Gremlins Authors
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
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
)

const sourceFileName = "sourceFile.go"

func TestExtractSnippet(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		content      []byte
		targetLine   int
		contextLines int
		want         []byte
	}{
		"should_clamp_start_to_zero_when_target_line_is_near_beginning": {
			content:      []byte("line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10"),
			targetLine:   1,
			contextLines: 3,
			want:         []byte("line1\nline2\nline3\nline4"),
		},
		"should_extract_lines_around_target_when_target_is_in_the_middle": {
			content:      []byte("line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10"),
			targetLine:   5,
			contextLines: 2,
			want:         []byte("line3\nline4\nline5\nline6\nline7"),
		},
		"should_clamp_end_to_content_length_when_target_line_is_near_end": {
			content:      []byte("line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10"),
			targetLine:   9,
			contextLines: 3,
			want:         []byte("line6\nline7\nline8\nline9\nline10"),
		},
		"should_return_all_content_when_context_is_larger_than_file": {
			content:      []byte("line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10"),
			targetLine:   5,
			contextLines: 20,
			want:         []byte("line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10"),
		},
		"should_handle_trailing_newline_in_content": {
			content:      []byte("line1\nline2\nline3\n"),
			targetLine:   2,
			contextLines: 1,
			want:         []byte("line1\nline2\nline3"),
		},
		"should_return_only_target_line_when_context_is_zero": {
			content:      []byte("line1\nline2\nline3"),
			targetLine:   2,
			contextLines: 0,
			want:         []byte("line2"),
		},
		"should_return_empty_when_content_is_empty": {
			content:      []byte(""),
			targetLine:   1,
			contextLines: 3,
			want:         []byte(""),
		},
		"should_return_empty_when_target_line_is_beyond_content": {
			content:      []byte("line1\nline2\nline3"),
			targetLine:   20,
			contextLines: 3,
			want:         []byte(""),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := extractSnippet(tc.content, tc.targetLine, tc.contextLines)
			if diff := cmp.Diff(string(tc.want), string(got)); diff != "" {
				t.Errorf("extractSnippet() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// parseSource parses src as the file name of a module.
func parseSource(t *testing.T, name, src string) *parsedFile {
	t.Helper()
	set := token.NewFileSet()
	f, err := parser.ParseFile(set, name, src, parser.ParseComments)
	if err != nil {
		t.Fatalf("ParseFile(%q) error = %v", name, err)
	}

	return &parsedFile{set: set, file: f, name: name}
}

// writeSource writes src to the file name in a new module copy and returns
// the directory of the copy.
func writeSource(t *testing.T, name, src string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}

	return dir
}

// operatorsOf returns the operators tok of pf, in source order.
func operatorsOf(t *testing.T, pf *parsedFile, tok token.Token) []*NodeToken {
	t.Helper()
	var nodes []*NodeToken
	inspectTokens(pf.file, fileScope{}, func(n *NodeToken) bool {
		if n.Tok() == tok {
			nodes = append(nodes, n)
		}

		return true
	})
	if len(nodes) == 0 {
		t.Fatalf("found no %s operator", tok)
	}

	return nodes
}

// checkFile checks that the file at path contains want.
func checkFile(t *testing.T, what, path, want string) {
	t.Helper()
	//nolint:gosec // test code reading test file
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, string(got)); diff != "" {
		t.Errorf("%s mismatch (-want +got):\n%s", what, diff)
	}
}

// checkAST checks that the printed AST of pf is want.
func checkAST(t *testing.T, what string, pf *parsedFile, want string) {
	t.Helper()
	got, err := pf.source()
	if err != nil {
		t.Fatalf("print() error = %v", err)
	}
	if diff := cmp.Diff(want, string(got)); diff != "" {
		t.Errorf("%s mismatch (-want +got):\n%s", what, diff)
	}
}

func TestFeatureMutantApplySetsSnippets(t *testing.T) {
	t.Parallel()

	const src = "package main\n\nfunc main() {\n\ta := 1 + 2\n}\n"
	workdir := writeSource(t, sourceFileName, src)
	pf := parseSource(t, sourceFileName, src)
	mut := builtins.newMutant(pf, "example.com/test", operatorsOf(t, pf, token.ADD)[0], mutator.ArithmeticBase)
	mut.SetWorkdir(workdir)

	if err := mut.Apply(); err != nil {
		t.Fatalf("Apply() error = %v, want nil", err)
	}

	if diff := cmp.Diff(src, string(mut.OrigSnippet())); diff != "" {
		t.Errorf("OrigSnippet() after Apply() mismatch (-want +got):\n%s", diff)
	}
	want := "package main\n\nfunc main() {\n\ta := 1 - 2\n}\n"
	if diff := cmp.Diff(want, string(mut.MutatedSnippet())); diff != "" {
		t.Errorf("MutatedSnippet() after Apply() mismatch (-want +got):\n%s", diff)
	}
}

func TestMutantApplyAndRollback(t *testing.T) {
	want := []string{
		"package main\n\nfunc main() {\n\ta := 1 - 2\n\tb := 1 - 2\n}\n",
		"package main\n\nfunc main() {\n\ta := 1 + 2\n\tb := 1 + 2\n}\n",
	}
	const rollbackWant = "package main\n\nfunc main() {\n\ta := 1 + 2\n\tb := 1 - 2\n}\n"
	workdir := writeSource(t, sourceFileName, rollbackWant)
	path := filepath.Join(workdir, sourceFileName)
	pf := parseSource(t, sourceFileName, rollbackWant)
	nodes := append(operatorsOf(t, pf, token.ADD), operatorsOf(t, pf, token.SUB)...)

	for i, node := range nodes {
		mut := builtins.newMutant(pf, "example.com/test", node, mutator.ArithmeticBase)
		mut.SetWorkdir(workdir)

		if err := mut.Apply(); err != nil {
			t.Fatalf("Apply() of node %d error = %v, want nil", i, err)
		}
		checkFile(t, fmt.Sprintf("source after Apply() of node %d", i), path, want[i])
		if err := mut.Rollback(); err != nil {
			t.Fatalf("Rollback() of node %d error = %v, want nil", i, err)
		}
		checkFile(t, fmt.Sprintf("source after Rollback() of node %d", i), path, rollbackWant)
	}
}

// TestRegressionTokenMutatorApplyWriteFailureRestoresAST proves that a failed
// write of the mutated file leaves no changed state for later mutants
// (issue #106). The AST is shared among the mutants of a file, so Apply must
// put back the original token when the write fails. A later mutant of the
// same AST must then write only its own mutation.
//
// The test makes the source file read-only to make the write fail. It needs
// a user that file permissions apply to, and it verifies that setup first.
func TestRegressionTokenMutatorApplyWriteFailureRestoresAST(t *testing.T) {
	const src = "package main\n\nfunc main() {\n\ta := 1 + 2\n\tb := 1 - 2\n}\n"
	const wantSecond = "package main\n\nfunc main() {\n\ta := 1 + 2\n\tb := 1 + 2\n}\n"
	workdir := writeSource(t, sourceFileName, src)
	path := filepath.Join(workdir, sourceFileName)
	if err := os.Chmod(path, 0o400); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(src), 0o400); err == nil {
		t.Skip("skipping: the current user can write a read-only file, so the write failure cannot be injected")
	}
	pf := parseSource(t, sourceFileName, src)
	add := operatorsOf(t, pf, token.ADD)[0]
	first := builtins.newMutant(pf, "example.com/test", add, mutator.ArithmeticBase)
	first.SetWorkdir(workdir)

	err := first.Apply()

	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("Apply() error = %v, want %v", err, fs.ErrPermission)
	}
	if got := add.Tok(); got != token.ADD {
		t.Errorf("AST token after failed Apply() = %v, want %v", got, token.ADD)
	}
	checkFile(t, "source after failed Apply()", path, src)

	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	second := builtins.newMutant(pf, "example.com/test", operatorsOf(t, pf, token.SUB)[0], mutator.ArithmeticBase)
	second.SetWorkdir(workdir)
	if err := second.Apply(); err != nil {
		t.Fatalf("second Apply() error = %v, want nil", err)
	}
	checkFile(t, "source after second Apply()", path, wantSecond)
}

// TestFeatureMutantApplyDoubleNegation checks that the INVERT_LOGICAL
// mutation of a unary ! adds a second ! in front of the operand. Apply must
// put back the AST before it returns, and Rollback must put back the file.
func TestFeatureMutantApplyDoubleNegation(t *testing.T) {
	t.Parallel()

	const doubleNot = "!!a"
	testCases := map[string]struct {
		expr    string
		want    string
		nthNode int
	}{
		"identifier":        {expr: "!a", want: doubleNot},
		"parenthesized":     {expr: "!(a || b)", want: "!!(a || b)"},
		"call":              {expr: "!f()", want: "!!f()"},
		"outer of a double": {expr: doubleNot, want: "!!!a"},
		"inner of a double": {expr: doubleNot, want: "!!!a", nthNode: 1},
		"comment after":     {expr: "!a\t/* c */", want: "!!a\t/* c */"},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			src := negationSrc(tc.expr)
			workdir := writeSource(t, sourceFileName, src)
			path := filepath.Join(workdir, sourceFileName)
			pf := parseSource(t, sourceFileName, src)
			mut := builtins.newMutant(pf, "example.com/test", operatorsOf(t, pf, token.NOT)[tc.nthNode], mutator.InvertLogical)
			mut.SetWorkdir(workdir)

			if err := mut.Apply(); err != nil {
				t.Fatalf("Apply() error = %v, want nil", err)
			}
			checkFile(t, "source after Apply()", path, negationSrc(tc.want))
			checkAST(t, "AST after Apply()", pf, src)
			if err := mut.Rollback(); err != nil {
				t.Fatalf("Rollback() error = %v, want nil", err)
			}
			checkFile(t, "source after Rollback()", path, src)
		})
	}
}

// negationSrc returns a file whose function g returns expr.
func negationSrc(expr string) string {
	return "package main\n\nfunc f() bool {\n\treturn true\n}\n\n" +
		"func g(a, b bool) bool {\n\treturn " + expr + "\n}\n"
}

// TestFeatureMutantApplyRejectsMissingMutation checks that Apply returns an
// error and leaves the file and the AST unchanged when the mutation type has
// no mutation for the operator.
func TestFeatureMutantApplyRejectsMissingMutation(t *testing.T) {
	t.Parallel()

	const src = "package main\n\nfunc main() {\n\ta := 1 + 2\n}\n"
	testCases := map[string]mutator.Type{
		"type without the operator": mutator.InvertLogical,
		"unregistered type":         mutator.Type(len(mutator.Types)),
	}
	for name, mt := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			workdir := writeSource(t, sourceFileName, src)
			pf := parseSource(t, sourceFileName, src)
			node := operatorsOf(t, pf, token.ADD)[0]
			mut := builtins.newMutant(pf, "example.com/test", node, mt)
			mut.SetWorkdir(workdir)

			err := mut.Apply()

			if !errors.Is(err, ErrNoMutation) {
				t.Errorf("Apply() error = %v, want %v", err, ErrNoMutation)
			}
			if got := node.Tok(); got != token.ADD {
				t.Errorf("AST token after Apply() = %v, want %v", got, token.ADD)
			}
			checkFile(t, "source after Apply()", filepath.Join(workdir, sourceFileName), src)
		})
	}
}

// TestFeatureMutantApplySharesTheLockOfItsFile applies all the mutants of
// two files at the same time, each in its own module copy. Each copy must
// contain only the mutation of its mutant, and the AST of each file must be
// unchanged at the end. Run the test with the race detector.
func TestFeatureMutantApplySharesTheLockOfItsFile(t *testing.T) {
	const srcA = "package main\n\nfunc a(x, y int) int {\n\treturn x + y + x + y\n}\n"
	const srcB = "package main\n\nfunc b(p, q bool) bool {\n\treturn !p && !q && !p\n}\n"
	files := []struct {
		pf          *parsedFile
		src         string
		replacement string
		tok         token.Token
		mt          mutator.Type
	}{
		{pf: parseSource(t, "a.go", srcA), src: srcA, tok: token.ADD, replacement: "-", mt: mutator.ArithmeticBase},
		{pf: parseSource(t, "sub/b.go", srcB), src: srcB, tok: token.NOT, replacement: "!!", mt: mutator.InvertLogical},
	}
	type job struct {
		mut  *mutant
		path string
		want string
	}
	var jobs []job
	for _, f := range files {
		for _, n := range operatorsOf(t, f.pf, f.tok) {
			workdir := writeSource(t, f.pf.name, f.src)
			mut := builtins.newMutant(f.pf, "example.com/test", n, f.mt)
			mut.SetWorkdir(workdir)
			off := mut.Position().Offset
			jobs = append(jobs, job{
				mut:  mut,
				path: filepath.Join(workdir, filepath.FromSlash(f.pf.name)),
				want: f.src[:off] + f.replacement + f.src[off+1:],
			})
		}
	}

	errs := make([]error, len(jobs))
	var wg sync.WaitGroup
	for i, j := range jobs {
		wg.Go(func() { errs[i] = j.mut.Apply() })
	}
	wg.Wait()

	for i, j := range jobs {
		if errs[i] != nil {
			t.Fatalf("Apply() of mutant %d error = %v, want nil", i, errs[i])
		}
		checkFile(t, fmt.Sprintf("source of mutant %d at %s", i, j.mut.Position()), j.path, j.want)
	}
	for _, f := range files {
		checkAST(t, "AST of "+f.pf.name+" after Apply()", f.pf, f.src)
	}
}

// TestFeatureParsedFileConfinesWorkFilesToTheCopy checks that the file
// access of a mutant stays in its module copy, also for a file name that
// points out of it.
func TestFeatureParsedFileConfinesWorkFilesToTheCopy(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	workdir := filepath.Join(parent, "copy")
	if err := os.Mkdir(workdir, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(parent, "escape.go")
	if err := os.WriteFile(outside, []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	pf := &parsedFile{name: "../escape.go"}

	if _, err := pf.readWork(workdir); err == nil {
		t.Error("readWork(../escape.go) error = nil, want an error")
	}
	if err := pf.writeWork(workdir, []byte("package mutated\n")); err == nil {
		t.Error("writeWork(../escape.go) error = nil, want an error")
	}
	checkFile(t, "file outside the copy", outside, "package main\n")
}

// TestFeatureMutantApplyReportsRestorationFailure checks that Apply returns
// both the write error and the restoration error when it can neither write
// the mutated file nor write back the original file.
//
// The test makes the source file read-only, so both writes fail. It needs a
// user that file permissions apply to, and it verifies that setup first.
func TestFeatureMutantApplyReportsRestorationFailure(t *testing.T) {
	const src = "package main\n\nfunc main() {\n\ta := 1 + 2\n}\n"
	workdir := writeSource(t, sourceFileName, src)
	path := filepath.Join(workdir, sourceFileName)
	if err := os.Chmod(path, 0o400); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(src), 0o400); err == nil {
		t.Skip("skipping: the current user can write a read-only file, so the write failure cannot be injected")
	}
	pf := parseSource(t, sourceFileName, src)
	mut := builtins.newMutant(pf, "example.com/test", operatorsOf(t, pf, token.ADD)[0], mutator.ArithmeticBase)
	mut.SetWorkdir(workdir)

	err := mut.Apply()

	var joined interface{ Unwrap() []error }
	if !errors.As(err, &joined) || len(joined.Unwrap()) != 2 {
		t.Fatalf("Apply() error = %v, want the write error joined with the restoration error", err)
	}
	for i, e := range joined.Unwrap() {
		if !errors.Is(e, fs.ErrPermission) {
			t.Errorf("Apply() error %d = %v, want %v", i, e, fs.ErrPermission)
		}
	}
}

// TestRegressionExtremeSnippetShowsTheAddedReturn checks that the snippets of
// an extreme mutant of a function with a long signature show the body. The
// mutant is at the func keyword, which is more than three lines before the
// added return statement.
func TestRegressionExtremeSnippetShowsTheAddedReturn(t *testing.T) {
	t.Parallel()

	const src = "package main\n\nfunc f(\n\ta int,\n\tb int,\n\tc int,\n\td int,\n) {\n\tprintln(a, b, c, d)\n}\n"
	workdir := writeSource(t, sourceFileName, src)
	pf := parseSource(t, sourceFileName, src)
	var fn *NodeToken
	inspectTokens(pf.file, fileScope{pkg: pkgNames{}}, func(n *NodeToken) bool {
		if n.fn != nil {
			fn = n
		}

		return true
	})
	if fn == nil {
		t.Fatal("found no function candidate")
	}
	mut := builtins.newMutant(pf, "example.com/test", fn, mutator.ExtremeVoid)
	mut.SetWorkdir(workdir)

	if err := mut.Apply(); err != nil {
		t.Fatalf("Apply() error = %v, want nil", err)
	}

	if got := string(mut.MutatedSnippet()); !strings.Contains(got, "\treturn\n") {
		t.Errorf("MutatedSnippet() = %q, want the added return statement", got)
	}
	if got := string(mut.OrigSnippet()); !strings.Contains(got, "println(a, b, c, d)") {
		t.Errorf("OrigSnippet() = %q, want the first body statement", got)
	}
}
