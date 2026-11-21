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

package engine_test

import (
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/OpenRAE/go-rae-gremlins/internal/engine"
	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
)

const sourceFileName = "sourceFile.go"

func TestTokenMutator_Apply_SetsSnippets(t *testing.T) {
	t.Parallel()

	src := "package main\n\nfunc main() {\n\ta := 1 + 2\n}\n"

	testCases := map[string]struct {
		assertFunc func(t *testing.T, mut *engine.TokenMutator, err error)
	}{
		"should_set_orig_snippet_when_apply_is_called": {
			assertFunc: func(t *testing.T, mut *engine.TokenMutator, err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
				want := []byte("package main\n\nfunc main() {\n\ta := 1 + 2\n}\n")
				if diff := cmp.Diff(string(want), string(mut.OrigSnippet())); diff != "" {
					t.Fatalf("OrigSnippet() after Apply() mismatch (-want +got):\n%s", diff)
				}
			},
		},
		"should_set_mutated_snippet_when_apply_is_called": {
			assertFunc: func(t *testing.T, mut *engine.TokenMutator, err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
				want := []byte("package main\n\nfunc main() {\n\ta := 1 - 2\n}\n")
				if diff := cmp.Diff(string(want), string(mut.MutatedSnippet())); diff != "" {
					t.Fatalf("MutatedSnippet() after Apply() mismatch (-want +got):\n%s", diff)
				}
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			workdir := t.TempDir()
			filePath := sourceFileName
			fileFullPath := filepath.Join(workdir, filePath)

			err := os.WriteFile(fileFullPath, []byte(src), 0o600)
			if err != nil {
				t.Fatal(err)
			}

			set := token.NewFileSet()
			f, err := parser.ParseFile(set, filePath, src, parser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}

			var node *ast.BinaryExpr
			ast.Inspect(f, func(n ast.Node) bool {
				if b, ok := n.(*ast.BinaryExpr); ok && node == nil {
					node = b
				}

				return true
			})
			if node == nil {
				t.Fatal("binary expression node must be found")
			}

			n, ok := engine.NewTokenNode(node)
			if !ok {
				t.Fatal("token node must be created")
			}

			mut := engine.NewTokenMutant("example.com/test", set, f, n)
			mut.SetType(mutator.ArithmeticBase)
			mut.SetStatus(mutator.Runnable)
			mut.SetWorkdir(workdir)

			applyErr := mut.Apply()

			tc.assertFunc(t, mut, applyErr)
		})
	}
}

func TestMutantApplyAndRollback(t *testing.T) {
	want := []string{
		"package main\n\nfunc main() {\n\ta := 1 - 2\n\tb := 1 - 2\n}\n",
		"package main\n\nfunc main() {\n\ta := 1 + 2\n\tb := 1 + 2\n}\n",
	}
	rollbackWant := "package main\n\nfunc main() {\n\ta := 1 + 2\n\tb := 1 - 2\n}\n"

	workdir := t.TempDir()
	filePath := sourceFileName
	fileFullPath := filepath.Join(workdir, filePath)

	err := os.WriteFile(fileFullPath, []byte(rollbackWant), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	set := token.NewFileSet()
	f, err := parser.ParseFile(set, filePath, rollbackWant, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	var nodes []*ast.BinaryExpr
	ast.Inspect(f, func(n ast.Node) bool {
		if n, ok := n.(*ast.BinaryExpr); ok {
			nodes = append(nodes, n)
		}

		return true
	})

	for i, node := range nodes {
		n, ok := engine.NewTokenNode(node)
		if !ok {
			t.Fatalf("NewTokenNode(node %d) ok = false, want true", i)
		}
		mut := engine.NewTokenMutant("example.com/test", set, f, n)
		mut.SetType(mutator.ArithmeticBase)
		mut.SetStatus(mutator.Runnable)
		mut.SetWorkdir(workdir)

		err = mut.Apply()
		if err != nil {
			t.Fatal(err)
		}

		//nolint:gosec // test code reading test file
		got, err := os.ReadFile(fileFullPath)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(want[i], string(got)); diff != "" {
			t.Fatalf("source after Apply() of node %d mismatch (-want +got):\n%s", i, diff)
		}

		err = mut.Rollback()
		if err != nil {
			t.Fatal(err)
		}

		//nolint:gosec // test code reading test file
		got, err = os.ReadFile(fileFullPath)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(rollbackWant, string(got)); diff != "" {
			t.Fatalf("source after Rollback() of node %d mismatch (-want +got):\n%s", i, diff)
		}
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

	workdir := t.TempDir()
	filePath := sourceFileName
	fileFullPath := filepath.Join(workdir, filePath)
	if err := os.WriteFile(fileFullPath, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(fileFullPath, 0o400); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fileFullPath, []byte(src), 0o400); err == nil {
		t.Skip("skipping: the current user can write a read-only file, so the write failure cannot be injected")
	}

	set := token.NewFileSet()
	f, err := parser.ParseFile(set, filePath, src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	var nodes []*engine.NodeToken
	ast.Inspect(f, func(n ast.Node) bool {
		if be, ok := n.(*ast.BinaryExpr); ok {
			tn, ok := engine.NewTokenNode(be)
			if !ok {
				t.Fatal("new token node should be created")
			}
			nodes = append(nodes, tn)
		}

		return true
	})
	if len(nodes) != 2 {
		t.Fatalf("found %d binary expressions, want 2", len(nodes))
	}

	first := engine.NewTokenMutant("example.com/test", set, f, nodes[0])
	first.SetType(mutator.ArithmeticBase)
	first.SetWorkdir(workdir)

	err = first.Apply()

	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("Apply() error = %v, want %v", err, fs.ErrPermission)
	}
	if got := nodes[0].Tok(); got != token.ADD {
		t.Errorf("AST token after failed Apply() = %v, want %v", got, token.ADD)
	}
	//nolint:gosec // test code reading test file
	got, err := os.ReadFile(fileFullPath)
	if err != nil {
		t.Fatal(err)
	}
	if !cmp.Equal(string(got), src) {
		t.Errorf("source after failed Apply() differs (-want +got):\n%s", cmp.Diff(src, string(got)))
	}

	if err := os.Chmod(fileFullPath, 0o600); err != nil {
		t.Fatal(err)
	}
	second := engine.NewTokenMutant("example.com/test", set, f, nodes[1])
	second.SetType(mutator.ArithmeticBase)
	second.SetWorkdir(workdir)
	if err := second.Apply(); err != nil {
		t.Fatalf("second Apply() error = %v, want nil", err)
	}
	//nolint:gosec // test code reading test file
	got, err = os.ReadFile(fileFullPath)
	if err != nil {
		t.Fatal(err)
	}
	if !cmp.Equal(string(got), wantSecond) {
		t.Errorf("source after second Apply() differs (-want +got):\n%s", cmp.Diff(wantSecond, string(got)))
	}
}

// TestFeatureTokenMutatorApplyDoubleNegation checks that the INVERT_LOGICAL
// mutation of a unary ! adds a second ! in front of the operand. Apply must
// put back the AST before it returns, and Rollback must put back the file.
func TestFeatureTokenMutatorApplyDoubleNegation(t *testing.T) {
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
			workdir := t.TempDir()
			fileFullPath := filepath.Join(workdir, sourceFileName)
			if err := os.WriteFile(fileFullPath, []byte(src), 0o600); err != nil {
				t.Fatal(err)
			}
			set := token.NewFileSet()
			f, err := parser.ParseFile(set, sourceFileName, src, parser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}
			mut := engine.NewTokenMutant("example.com/test", set, f, nthNegation(t, f, tc.nthNode))
			mut.SetType(mutator.InvertLogical)
			mut.SetWorkdir(workdir)

			if err := mut.Apply(); err != nil {
				t.Fatalf("Apply() error = %v, want nil", err)
			}
			checkFile(t, "source after Apply()", fileFullPath, negationSrc(tc.want))
			var printed bytes.Buffer
			if err := printer.Fprint(&printed, set, f); err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(src, printed.String()); diff != "" {
				t.Errorf("AST after Apply() mismatch (-want +got):\n%s", diff)
			}
			if err := mut.Rollback(); err != nil {
				t.Fatalf("Rollback() error = %v, want nil", err)
			}
			checkFile(t, "source after Rollback()", fileFullPath, src)
		})
	}
}

// negationSrc returns a file whose function g returns expr.
func negationSrc(expr string) string {
	return "package main\n\nfunc f() bool {\n\treturn true\n}\n\n" +
		"func g(a, b bool) bool {\n\treturn " + expr + "\n}\n"
}

// nthNegation returns the token node of the nth unary ! of f.
func nthNegation(t *testing.T, f *ast.File, nth int) *engine.NodeToken {
	t.Helper()
	var nots []*ast.UnaryExpr
	ast.Inspect(f, func(n ast.Node) bool {
		if u, ok := n.(*ast.UnaryExpr); ok && u.Op == token.NOT {
			nots = append(nots, u)
		}

		return true
	})
	if len(nots) <= nth {
		t.Fatalf("found %d ! operators, want more than %d", len(nots), nth)
	}
	node, ok := engine.NewTokenNode(nots[nth])
	if !ok {
		t.Fatal("NewTokenNode(!) ok = false, want true")
	}

	return node
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

// TestFeatureTokenMutatorApplyRejectsMissingMutation checks that Apply
// returns an error and leaves the file unchanged when the mutation type has
// no mutation for the token.
func TestFeatureTokenMutatorApplyRejectsMissingMutation(t *testing.T) {
	t.Parallel()

	const src = "package main\n\nfunc main() {\n\ta := 1 + 2\n}\n"
	workdir := t.TempDir()
	fileFullPath := filepath.Join(workdir, sourceFileName)
	if err := os.WriteFile(fileFullPath, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	set := token.NewFileSet()
	f, err := parser.ParseFile(set, sourceFileName, src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	var node *engine.NodeToken
	ast.Inspect(f, func(n ast.Node) bool {
		if be, ok := n.(*ast.BinaryExpr); ok {
			node, _ = engine.NewTokenNode(be)
		}

		return true
	})
	if node == nil {
		t.Fatal("binary expression node must be found")
	}
	mut := engine.NewTokenMutant("example.com/test", set, f, node)
	mut.SetType(mutator.InvertLogical)
	mut.SetWorkdir(workdir)

	err = mut.Apply()

	if !errors.Is(err, engine.ErrNoMutation) {
		t.Errorf("Apply() error = %v, want %v", err, engine.ErrNoMutation)
	}
	if got := node.Tok(); got != token.ADD {
		t.Errorf("AST token after Apply() = %v, want %v", got, token.ADD)
	}
	checkFile(t, "source after Apply()", fileFullPath, src)
}
