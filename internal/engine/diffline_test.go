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
	"testing"
	"testing/fstest"

	"github.com/OpenRAE/go-rae-gremlins/internal/configuration"
	"github.com/OpenRAE/go-rae-gremlins/internal/coverage"
	"github.com/OpenRAE/go-rae-gremlins/internal/diff"
	"github.com/OpenRAE/go-rae-gremlins/internal/engine"
	"github.com/OpenRAE/go-rae-gremlins/internal/gomodule"
	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
)

// TestRegressionOperatorDiffSelectionUsesPhysicalLines checks that the diff
// selection of an operator uses the physical line of the file. The diff of
// git has physical lines, and a //line directive changes only the reported
// position. Before the fix, the operator after the directive was skipped.
func TestRegressionOperatorDiffSelectionUsesPhysicalLines(t *testing.T) {
	const fileName = "p.go"
	const src = "package p\n\nfunc f(a int) int {\n//line other.go:100\n\treturn a * 2\n}\n"
	viperSet(map[string]any{configuration.UnleashDryRunKey: true})
	defer viperReset()
	mod := gomodule.GoModule{Name: expectedModule, Root: ".", CallingDir: "."}
	sys := fstest.MapFS{fileName: {Data: []byte(src)}}
	codeData := engine.CodeData{
		Cov:  coverage.Profile{"other.go": {{StartLine: 99, StartCol: 1, EndLine: 101, EndCol: 1}}},
		Diff: diff.Diff{fileName: {{StartLine: 5, EndLine: 5}}},
	}
	mut := engine.New(mod, codeData, newJobDealerStub(t), engine.WithDirFs(sys))
	res := runEngine(t, mut)

	found := false
	for _, m := range res.Mutants {
		if m.Type() != mutator.ArithmeticBase {
			continue
		}
		found = true
		if m.Status() != mutator.Runnable {
			t.Errorf("ARITHMETIC_BASE at %s status = %s, want %s: physical line 5 changed", m.Position(), m.Status(), mutator.Runnable)
		}
	}
	if !found {
		t.Error("Run() gave no ARITHMETIC_BASE mutant, want one at physical line 5")
	}
}
