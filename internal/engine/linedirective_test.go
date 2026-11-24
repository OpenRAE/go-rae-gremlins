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

package engine_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/OpenRAE/go-rae-gremlins/internal/configuration"
	"github.com/OpenRAE/go-rae-gremlins/internal/coverage"
	"github.com/OpenRAE/go-rae-gremlins/internal/engine"
	"github.com/OpenRAE/go-rae-gremlins/internal/gomodule"
	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
	"github.com/OpenRAE/go-rae-gremlins/internal/report"
)

// TestRegressionApplyWritesThePhysicalFileOfALineDirective checks that a
// //line directive changes only the reported position of a mutant. Apply and
// Rollback must use the parsed file in the worker copy, and the snippets must
// use its physical lines.
func TestRegressionApplyWritesThePhysicalFileOfALineDirective(t *testing.T) {
	testCases := []struct {
		name        string
		directive   string
		wantDisplay string
	}{
		{name: "other file name", directive: "//line generated.y:40", wantDisplay: "generated.y"},
		{name: "parent path", directive: "//line ../escape.go:40", wantDisplay: "../escape.go"},
		{name: "empty file name", directive: "//line :40", wantDisplay: ""},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			src := "package main\n\nfunc f(a, b int) int {\n" + tc.directive + "\n\treturn a + b\n}\n"
			base := t.TempDir()

			res, err := runArithmeticBase(t, base, "parser.go", src)

			if err != nil {
				t.Fatalf("Run() error = %v, want nil", err)
			}
			checkPhysicalMutant(t, res, base, filepath.FromSlash(tc.wantDisplay))
		})
	}
}

// runArithmeticBase runs the engine with only ARITHMETIC_BASE enabled on the
// module with the file fileName. Each mutant applies and rolls back in its own
// copy of the module under base.
func runArithmeticBase(t *testing.T, base, fileName, src string) (report.Results, error) {
	t.Helper()
	enabled := make(map[string]any, len(mutator.Types))
	for _, mt := range mutator.Types {
		enabled[configuration.MutantTypeEnabledKey(mt)] = mt == mutator.ArithmeticBase
	}
	viperSet(enabled)
	defer viperReset()
	mod := gomodule.GoModule{Name: expectedModule, Root: ".", CallingDir: "."}
	mapFS := fstest.MapFS{fileName: {Data: []byte(src)}}
	dealer := &applyingDealerStub{base: base, fileName: fileName, src: []byte(src)}

	mut := engine.New(mod, engine.CodeData{Cov: coverage.Profile{}}, dealer, engine.WithDirFs(mapFS))

	return mut.Run(t.Context())
}

// checkPhysicalMutant checks that res has one mutant of a + b, reported at
// the file wantDisplay, with snippets of the physical lines. It also checks
// that no file escape.go exists in base, outside the module copies.
func checkPhysicalMutant(t *testing.T, res report.Results, base, wantDisplay string) {
	t.Helper()
	if len(res.Mutants) != 1 {
		t.Fatalf("Run() found %d mutants, want 1", len(res.Mutants))
	}
	m := res.Mutants[0]
	if got := m.Position().Filename; got != wantDisplay {
		t.Errorf("Position().Filename = %q, want %q", got, wantDisplay)
	}
	if got := string(m.OrigSnippet()); !strings.Contains(got, "return a + b") {
		t.Errorf("OrigSnippet() = %q, want the line with a + b", got)
	}
	if got := string(m.MutatedSnippet()); !strings.Contains(got, "return a - b") {
		t.Errorf("MutatedSnippet() = %q, want the line with a - b", got)
	}
	if _, err := os.Stat(filepath.Join(base, "escape.go")); !os.IsNotExist(err) {
		t.Errorf("Stat(escape.go outside the worker copy) error = %v, want not exist", err)
	}
}
