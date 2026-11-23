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
			const fileName = "parser.go"
			enabled := make(map[string]any, len(mutator.Types))
			for _, mt := range mutator.Types {
				enabled[configuration.MutantTypeEnabledKey(mt)] = mt == mutator.ArithmeticBase
			}
			viperSet(enabled)
			defer viperReset()
			mod := gomodule.GoModule{Name: expectedModule, Root: ".", CallingDir: "."}
			mapFS := fstest.MapFS{fileName: {Data: []byte(src)}}
			base := t.TempDir()
			dealer := &applyingDealerStub{base: base, fileName: fileName, src: []byte(src)}

			mut := engine.New(mod, engine.CodeData{Cov: coverage.Profile{}}, dealer, engine.WithDirFs(mapFS))
			res, err := mut.Run(t.Context())
			if err != nil {
				t.Fatalf("Run() error = %v, want nil", err)
			}

			if len(res.Mutants) != 1 {
				t.Fatalf("Run() found %d mutants, want 1", len(res.Mutants))
			}
			m := res.Mutants[0]
			if got := m.Position().Filename; got != tc.wantDisplay {
				t.Errorf("Position().Filename = %q, want %q", got, tc.wantDisplay)
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
		})
	}
}
