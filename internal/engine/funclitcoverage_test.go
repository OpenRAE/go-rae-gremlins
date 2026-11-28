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
	"fmt"
	"testing"

	"github.com/OpenRAE/go-rae-gremlins/internal/configuration"
	"github.com/OpenRAE/go-rae-gremlins/internal/coverage"
	"github.com/OpenRAE/go-rae-gremlins/internal/engine"
	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
)

const funcLitFixture = "testdata/fixtures/funclit_go"

// funcLitLayouts hold the executed blocks of funclit_go that go test
// -coverprofile reports for a test that calls each function once. The second
// function literal of Simple did not run. The blocks are in the
// "startLine.startCol,endLine.endCol" form of the profile.
var funcLitLayouts = map[string][]string{
	layoutGo126: {
		"3.29,3.43", "5.44,6.11", "6.11,8.3",
		"12.21,13.26", "13.26,13.38", "13.47,15.3",
		"19.21,21.27", "21.27,21.39", "21.53,23.3", "24.2,24.10",
		"27.23,29.28", "29.28,29.40", "29.46,31.3", "32.2,32.10",
		"35.25,36.25", "36.25,36.37", "37.9,38.14",
		"43.24,44.32", "44.32,44.48",
		"47.25,50.27", "50.27,50.39", "50.53,52.12", "56.2,56.10",
	},
	layoutGo127: {
		"3.31,3.43", "6.2,6.11", "7.3,8.1",
		"13.2,13.26", "13.28,13.38", "14.3,15.1",
		"20.2,21.27", "21.29,21.39", "22.3,23.1", "24.2,24.10",
		"28.2,29.28", "29.30,29.40", "30.3,31.1", "32.2,32.10",
		"36.2,36.25", "36.27,36.37", "38.3,38.14",
		"44.2,44.32", "44.34,44.48",
		"48.2,50.27", "50.29,50.39", "51.3,52.12", "56.2,56.10",
	},
}

func TestFeatureFuncLitStatementCoverage(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name    string
		ran     func(b coverage.Block) bool
		mutants []clauseMutant
	}{
		{
			name: "a statement that ran proves its code after a function literal",
			ran:  func(coverage.Block) bool { return true },
			mutants: []clauseMutant{
				{line: 13, col: 43, mt: mutator.ConditionalsBoundary, want: mutator.Runnable},
				{line: 21, col: 44, mt: mutator.ConditionalsBoundary, want: mutator.Runnable},
				{line: 21, col: 50, mt: mutator.IncrementDecrement, want: mutator.Runnable},
				{line: 29, col: 42, mt: mutator.ArithmeticBase, want: mutator.Runnable},
				{line: 36, col: 39, mt: mutator.ArithmeticBase, want: mutator.Runnable},
				{line: 44, col: 43, mt: mutator.ArithmeticBase, want: mutator.Runnable},
				{line: 44, col: 79, mt: mutator.ArithmeticBase, want: mutator.Runnable},
				{line: 50, col: 44, mt: mutator.ConditionalsBoundary, want: mutator.Runnable},
				{line: 50, col: 50, mt: mutator.IncrementDecrement, want: mutator.Runnable},
			},
		},
		{
			name: "a function literal body that did not run is not covered",
			ran:  func(coverage.Block) bool { return true },
			mutants: []clauseMutant{
				{line: 44, col: 72, mt: mutator.ArithmeticBase, want: mutator.NotCovered},
			},
		},
		{
			name: "a statement that did not run proves no code after a function literal",
			ran:  func(b coverage.Block) bool { return b.StartLine < 12 || b.StartLine > 16 },
			mutants: []clauseMutant{
				{line: 13, col: 43, mt: mutator.ConditionalsBoundary, want: mutator.NotCovered},
			},
		},
	}
	for _, tc := range testCases {
		for layout, specs := range funcLitLayouts {
			t.Run(tc.name+"/"+layout, func(t *testing.T) {
				t.Parallel()
				viperSet(map[string]any{configuration.UnleashDryRunKey: true})
				defer viperReset()

				var blocks []coverage.Block
				for _, spec := range specs {
					if b := parseBlock(t, spec); tc.ran(b) {
						blocks = append(blocks, b)
					}
				}
				mapFS, mod := loadFixture(t, funcLitFixture, ".")
				profile := coverage.Profile{filenameFromFixture(funcLitFixture): blocks}

				mut := engine.New(mod, engine.CodeData{Cov: profile}, newJobDealerStub(t), engine.WithDirFs(mapFS))
				res := runEngine(t, mut)

				got := make(map[string]mutator.Status)
				for _, m := range res.Mutants {
					got[clauseMutantKey(m.Position().Line, m.Position().Column, m.Type())] = m.Status()
				}
				for _, w := range tc.mutants {
					key := clauseMutantKey(w.line, w.col, w.mt)
					if status, ok := got[key]; !ok {
						t.Errorf("no mutant %s found", key)
					} else if status != w.want {
						t.Errorf("mutant %s: got status %s, want %s", key, status, w.want)
					}
				}
			})
		}
	}
}

// parseBlock returns the coverage block of a profile range in the
// "startLine.startCol,endLine.endCol" form.
func parseBlock(t *testing.T, spec string) coverage.Block {
	t.Helper()
	var b coverage.Block
	if _, err := fmt.Sscanf(spec, "%d.%d,%d.%d", &b.StartLine, &b.StartCol, &b.EndLine, &b.EndCol); err != nil {
		t.Fatalf("parse block %q: %v", spec, err)
	}

	return b
}
