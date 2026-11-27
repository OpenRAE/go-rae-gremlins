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

const clausesFixture = "testdata/fixtures/clauses_go"

// clauseBlocks holds one coverage block for each clause header and body of
// clauses_go. A profile holds only the blocks that ran.
type clauseBlocks struct {
	classifyHeader coverage.Block
	classifyBig    coverage.Block
	classifySmall  coverage.Block
	eitherHeader   coverage.Block
	eitherBody     coverage.Block
	fallHeader     coverage.Block
	fallFirst      coverage.Block
	fallSecond     coverage.Block
	defaultHeader  coverage.Block
	defaultBody    coverage.Block
	defaultCase    coverage.Block
	sendHeader     coverage.Block
	sendBody       coverage.Block
	sendDefault    coverage.Block
	emptyHeader    coverage.Block
	emptyFirst     coverage.Block
	emptySecond    coverage.Block
}

// go126Blocks are the clauses_go blocks that go test -coverprofile reports on
// Go 1.26 and earlier. A clause body block starts immediately after the colon.
var go126Blocks = clauseBlocks{
	classifyHeader: coverage.Block{StartLine: 3, StartCol: 26, EndLine: 4, EndCol: 9},
	classifyBig:    coverage.Block{StartLine: 5, StartCol: 14, EndLine: 6, EndCol: 11},
	classifySmall:  coverage.Block{StartLine: 7, StartCol: 13, EndLine: 8, EndCol: 11},
	eitherHeader:   coverage.Block{StartLine: 13, StartCol: 24, EndLine: 14, EndCol: 9},
	eitherBody:     coverage.Block{StartLine: 15, StartCol: 23, EndLine: 16, EndCol: 11},
	fallHeader:     coverage.Block{StartLine: 21, StartCol: 22, EndLine: 23, EndCol: 9},
	fallFirst:      coverage.Block{StartLine: 24, StartCol: 14, EndLine: 26, EndCol: 14},
	fallSecond:     coverage.Block{StartLine: 27, StartCol: 13, EndLine: 28, EndCol: 6},
	defaultHeader:  coverage.Block{StartLine: 33, StartCol: 25, EndLine: 34, EndCol: 9},
	defaultBody:    coverage.Block{StartLine: 35, StartCol: 10, EndLine: 36, EndCol: 11},
	defaultCase:    coverage.Block{StartLine: 37, StartCol: 14, EndLine: 38, EndCol: 11},
	sendHeader:     coverage.Block{StartLine: 42, StartCol: 36, EndLine: 43, EndCol: 9},
	sendBody:       coverage.Block{StartLine: 44, StartCol: 19, EndLine: 45, EndCol: 14},
	sendDefault:    coverage.Block{StartLine: 46, StartCol: 10, EndLine: 47, EndCol: 15},
	emptyHeader:    coverage.Block{StartLine: 51, StartCol: 23, EndLine: 53, EndCol: 9},
	emptyFirst:     coverage.Block{StartLine: 54, StartCol: 13, EndLine: 54, EndCol: 13},
	emptySecond:    coverage.Block{StartLine: 55, StartCol: 13, EndLine: 56, EndCol: 8},
}

// go127Blocks are the clauses_go blocks that go test -coverprofile reports on
// Go 1.27 and later. A non-empty clause body block starts at its first
// statement, not immediately after the colon (issue #189).
var go127Blocks = clauseBlocks{
	classifyHeader: coverage.Block{StartLine: 4, StartCol: 2, EndLine: 4, EndCol: 9},
	classifyBig:    coverage.Block{StartLine: 6, StartCol: 3, EndLine: 6, EndCol: 11},
	classifySmall:  coverage.Block{StartLine: 8, StartCol: 3, EndLine: 8, EndCol: 11},
	eitherHeader:   coverage.Block{StartLine: 14, StartCol: 2, EndLine: 14, EndCol: 9},
	eitherBody:     coverage.Block{StartLine: 16, StartCol: 3, EndLine: 16, EndCol: 11},
	fallHeader:     coverage.Block{StartLine: 22, StartCol: 2, EndLine: 23, EndCol: 9},
	fallFirst:      coverage.Block{StartLine: 25, StartCol: 3, EndLine: 26, EndCol: 14},
	fallSecond:     coverage.Block{StartLine: 28, StartCol: 3, EndLine: 28, EndCol: 6},
	defaultHeader:  coverage.Block{StartLine: 34, StartCol: 2, EndLine: 34, EndCol: 9},
	defaultBody:    coverage.Block{StartLine: 36, StartCol: 3, EndLine: 36, EndCol: 11},
	defaultCase:    coverage.Block{StartLine: 38, StartCol: 3, EndLine: 38, EndCol: 11},
	sendHeader:     coverage.Block{StartLine: 43, StartCol: 2, EndLine: 43, EndCol: 9},
	sendBody:       coverage.Block{StartLine: 45, StartCol: 3, EndLine: 45, EndCol: 14},
	sendDefault:    coverage.Block{StartLine: 47, StartCol: 3, EndLine: 47, EndCol: 15},
	emptyHeader:    coverage.Block{StartLine: 52, StartCol: 2, EndLine: 53, EndCol: 9},
	emptyFirst:     coverage.Block{StartLine: 54, StartCol: 13, EndLine: 54, EndCol: 13},
	emptySecond:    coverage.Block{StartLine: 56, StartCol: 3, EndLine: 56, EndCol: 8},
}

var clauseLayouts = map[string]clauseBlocks{
	"go126": go126Blocks,
	"go127": go127Blocks,
}

// clauseMutant names a mutant position in clauses_go and its expected status.
type clauseMutant struct {
	line, col int
	mt        mutator.Type
	want      mutator.Status
}

func TestFeatureClauseExpressionCoverage(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name    string
		blocks  func(b clauseBlocks) []coverage.Block
		mutants []clauseMutant
	}{
		{
			name:   "a later case body proves that the earlier case expressions ran",
			blocks: func(b clauseBlocks) []coverage.Block { return []coverage.Block{b.classifyHeader, b.classifySmall} },
			mutants: []clauseMutant{
				{line: 5, col: 9, mt: mutator.ConditionalsBoundary, want: mutator.Runnable},
				{line: 7, col: 9, mt: mutator.ConditionalsBoundary, want: mutator.Runnable},
			},
		},
		{
			name:   "a matched first case does not prove the later case expressions",
			blocks: func(b clauseBlocks) []coverage.Block { return []coverage.Block{b.classifyHeader, b.classifyBig} },
			mutants: []clauseMutant{
				{line: 5, col: 9, mt: mutator.ConditionalsBoundary, want: mutator.Runnable},
				{line: 7, col: 9, mt: mutator.ConditionalsBoundary, want: mutator.NotCovered},
			},
		},
		{
			name:   "a reached switch with no clause body run proves no case expression",
			blocks: func(b clauseBlocks) []coverage.Block { return []coverage.Block{b.classifyHeader} },
			mutants: []clauseMutant{
				{line: 5, col: 9, mt: mutator.ConditionalsBoundary, want: mutator.NotCovered},
				{line: 7, col: 9, mt: mutator.ConditionalsBoundary, want: mutator.NotCovered},
			},
		},
		{
			name:   "a matched case proves only its first expression",
			blocks: func(b clauseBlocks) []coverage.Block { return []coverage.Block{b.eitherHeader, b.eitherBody} },
			mutants: []clauseMutant{
				{line: 15, col: 9, mt: mutator.ConditionalsBoundary, want: mutator.Runnable},
				{line: 15, col: 17, mt: mutator.ConditionalsBoundary, want: mutator.NotCovered},
				{line: 15, col: 19, mt: mutator.InvertNegatives, want: mutator.NotCovered},
			},
		},
		{
			name: "a body that fallthrough can reach does not prove its case expression",
			blocks: func(b clauseBlocks) []coverage.Block {
				return []coverage.Block{b.fallHeader, b.fallFirst, b.fallSecond}
			},
			mutants: []clauseMutant{
				{line: 24, col: 9, mt: mutator.ConditionalsBoundary, want: mutator.Runnable},
				{line: 27, col: 9, mt: mutator.ConditionalsBoundary, want: mutator.NotCovered},
			},
		},
		{
			name:   "a body after a fallthrough clause that did not run proves its case expression",
			blocks: func(b clauseBlocks) []coverage.Block { return []coverage.Block{b.fallHeader, b.fallSecond} },
			mutants: []clauseMutant{
				{line: 24, col: 9, mt: mutator.ConditionalsBoundary, want: mutator.Runnable},
				{line: 27, col: 9, mt: mutator.ConditionalsBoundary, want: mutator.Runnable},
			},
		},
		{
			name:   "a selected default clause proves the later case expressions",
			blocks: func(b clauseBlocks) []coverage.Block { return []coverage.Block{b.defaultHeader, b.defaultBody} },
			mutants: []clauseMutant{
				{line: 37, col: 9, mt: mutator.ConditionalsBoundary, want: mutator.Runnable},
			},
		},
		{
			name:   "a matched case after the default clause proves its expression",
			blocks: func(b clauseBlocks) []coverage.Block { return []coverage.Block{b.defaultHeader, b.defaultCase} },
			mutants: []clauseMutant{
				{line: 37, col: 9, mt: mutator.ConditionalsBoundary, want: mutator.Runnable},
			},
		},
		{
			name:   "a select clause body proves the send expression",
			blocks: func(b clauseBlocks) []coverage.Block { return []coverage.Block{b.sendHeader, b.sendDefault} },
			mutants: []clauseMutant{
				{line: 44, col: 15, mt: mutator.ArithmeticBase, want: mutator.Runnable},
			},
		},
		{
			name:   "a select with its own clause run proves the send expression",
			blocks: func(b clauseBlocks) []coverage.Block { return []coverage.Block{b.sendHeader, b.sendBody} },
			mutants: []clauseMutant{
				{line: 44, col: 15, mt: mutator.ArithmeticBase, want: mutator.Runnable},
			},
		},
		{
			name:   "a reached select with no clause body run proves no send expression",
			blocks: func(b clauseBlocks) []coverage.Block { return []coverage.Block{b.sendHeader} },
			mutants: []clauseMutant{
				{line: 44, col: 15, mt: mutator.ArithmeticBase, want: mutator.NotCovered},
			},
		},
		{
			name:   "a matched empty case body proves its expression",
			blocks: func(b clauseBlocks) []coverage.Block { return []coverage.Block{b.emptyHeader, b.emptyFirst} },
			mutants: []clauseMutant{
				{line: 54, col: 9, mt: mutator.ConditionalsBoundary, want: mutator.Runnable},
				{line: 55, col: 9, mt: mutator.ConditionalsBoundary, want: mutator.NotCovered},
			},
		},
	}
	for _, tc := range testCases {
		for _, layout := range []string{"go126", "go127"} {
			t.Run(tc.name+"/"+layout, func(t *testing.T) {
				t.Parallel()
				viperSet(map[string]any{configuration.UnleashDryRunKey: true})
				defer viperReset()

				mapFS, mod := loadFixture(t, clausesFixture, ".")
				profile := coverage.Profile{filenameFromFixture(clausesFixture): tc.blocks(clauseLayouts[layout])}

				mut := engine.New(mod, engine.CodeData{Cov: profile}, newJobDealerStub(t), engine.WithDirFs(mapFS))
				res := runEngine(t, mut)

				got := make(map[string]mutator.Status)
				for _, m := range res.Mutants {
					got[clauseMutantKey(m.Position().Line, m.Position().Column, m.Type())] = m.Status()
				}
				for _, w := range tc.mutants {
					key := clauseMutantKey(w.line, w.col, w.mt)
					status, ok := got[key]
					if !ok {
						t.Errorf("no mutant %s found", key)

						continue
					}
					if status != w.want {
						t.Errorf("mutant %s: got status %s, want %s", key, status, w.want)
					}
				}
			})
		}
	}
}

func clauseMutantKey(line, col int, mt mutator.Type) string {
	return fmt.Sprintf("%s at %d:%d", mt, line, col)
}
