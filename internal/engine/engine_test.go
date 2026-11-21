/*
 * Copyright 2022 The Gremlins Authors
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
	"context"
	"errors"
	"fmt"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"testing/synctest"

	"github.com/google/go-cmp/cmp"

	"github.com/OpenRAE/go-rae-gremlins/internal/configuration"
	"github.com/OpenRAE/go-rae-gremlins/internal/coverage"
	"github.com/OpenRAE/go-rae-gremlins/internal/diff"
	"github.com/OpenRAE/go-rae-gremlins/internal/engine"
	"github.com/OpenRAE/go-rae-gremlins/internal/engine/workerpool"
	"github.com/OpenRAE/go-rae-gremlins/internal/gomodule"
	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
	"github.com/OpenRAE/go-rae-gremlins/internal/report"
)

const (
	defaultFixture = "testdata/fixtures/gtr_go"
	expectedModule = "example.com"
)

var testCodeData = engine.CodeData{Cov: coveredPosition(defaultFixture).Profile}

func coveredPosition(fixture string) coverage.Result {
	fn := filenameFromFixture(fixture)
	p := coverage.Profile{fn: {{StartLine: 6, EndLine: 7, StartCol: 8, EndCol: 9}}}

	return coverage.Result{Profile: p, Elapsed: 10}
}

func notCoveredPosition(fixture string) coverage.Result {
	fn := filenameFromFixture(fixture)
	p := coverage.Profile{fn: {{StartLine: 9, EndLine: 9, StartCol: 8, EndCol: 9}}}

	return coverage.Result{Profile: p, Elapsed: 10}
}

type mutationsTest struct {
	name       string
	fixture    string
	covResult  coverage.Result
	mutantType mutator.Type
	token      token.Token
	mutStatus  mutator.Status
}

var mutationsTests = []mutationsTest{
	// CONDITIONAL BOUNDARIES
	{
		name:       "it recognizes CONDITIONAL_BOUNDARY with GTR",
		fixture:    "testdata/fixtures/gtr_go",
		mutantType: mutator.ConditionalsBoundary,
		token:      token.GTR,
		covResult:  coveredPosition("testdata/fixtures/gtr_go"),
		mutStatus:  mutator.Runnable,
	},
	{
		name:       "it recognizes CONDITIONAL_BOUNDARY with LSS",
		fixture:    "testdata/fixtures/lss_go",
		mutantType: mutator.ConditionalsBoundary,
		token:      token.LSS,
		covResult:  notCoveredPosition("testdata/fixtures/lss_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes CONDITIONAL_BOUNDARY with LEQ",
		fixture:    "testdata/fixtures/leq_go",
		mutantType: mutator.ConditionalsBoundary,
		token:      token.LEQ,
		covResult:  notCoveredPosition("testdata/fixtures/leq_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes CONDITIONAL_BOUNDARY with GEQ",
		fixture:    "testdata/fixtures/geq_go",
		mutantType: mutator.ConditionalsBoundary,
		token:      token.GEQ,
		covResult:  notCoveredPosition("testdata/fixtures/geq_go"),
		mutStatus:  mutator.NotCovered,
	},
	// INCREMENT_DECREMENT
	{
		name:       "it recognizes INCREMENT_DECREMENT with INC",
		fixture:    "testdata/fixtures/inc_go",
		mutantType: mutator.IncrementDecrement,
		token:      token.INC,
		covResult:  notCoveredPosition("testdata/fixtures/inc_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes INCREMENT_DECREMENT with DEC",
		fixture:    "testdata/fixtures/dec_go",
		mutantType: mutator.IncrementDecrement,
		token:      token.DEC,
		covResult:  notCoveredPosition("testdata/fixtures/dec_go"),
		mutStatus:  mutator.NotCovered,
	},
	// CONDITIONAL_NEGATION
	{
		name:       "it recognizes CONDITIONAL_NEGATION with EQL",
		fixture:    "testdata/fixtures/eql_go",
		mutantType: mutator.ConditionalsNegation,
		token:      token.EQL,
		covResult:  notCoveredPosition("testdata/fixtures/eql_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes CONDITIONAL_NEGATION with NEQ",
		fixture:    "testdata/fixtures/neq_go",
		mutantType: mutator.ConditionalsNegation,
		token:      token.NEQ,
		covResult:  notCoveredPosition("testdata/fixtures/neq_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes CONDITIONAL_NEGATION with LEQ",
		fixture:    "testdata/fixtures/leq_go",
		mutantType: mutator.ConditionalsNegation,
		token:      token.LEQ,
		covResult:  notCoveredPosition("testdata/fixtures/leq_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes CONDITIONAL_NEGATION with GTR",
		fixture:    "testdata/fixtures/gtr_go",
		mutantType: mutator.ConditionalsNegation,
		token:      token.GTR,
		covResult:  notCoveredPosition("testdata/fixtures/gtr_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes CONDITIONAL_NEGATION with GEQ",
		fixture:    "testdata/fixtures/geq_go",
		mutantType: mutator.ConditionalsNegation,
		token:      token.GEQ,
		covResult:  notCoveredPosition("testdata/fixtures/geq_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes CONDITIONAL_NEGATION with LSS",
		fixture:    "testdata/fixtures/lss_go",
		mutantType: mutator.ConditionalsNegation,
		token:      token.LSS,
		covResult:  notCoveredPosition("testdata/fixtures/lss_go"),
		mutStatus:  mutator.NotCovered,
	},
	// INVERT_NEGATIVES
	{
		name:       "it recognizes INVERT_NEGATIVE with SUB",
		fixture:    "testdata/fixtures/negative_sub_go",
		mutantType: mutator.InvertNegatives,
		token:      token.SUB,
		covResult:  notCoveredPosition("testdata/fixtures/negative_sub_go"),
		mutStatus:  mutator.NotCovered,
	},
	// ARITHMETIC_BASIC
	{
		name:       "it recognizes ARITHMETIC_BASIC with ADD",
		fixture:    "testdata/fixtures/add_go",
		mutantType: mutator.ArithmeticBase,
		token:      token.ADD,
		covResult:  notCoveredPosition("testdata/fixtures/add_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes ARITHMETIC_BASIC with SUB",
		fixture:    "testdata/fixtures/sub_go",
		mutantType: mutator.ArithmeticBase,
		token:      token.SUB,
		covResult:  notCoveredPosition("testdata/fixtures/sub_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes ARITHMETIC_BASIC with MUL",
		fixture:    "testdata/fixtures/mul_go",
		mutantType: mutator.ArithmeticBase,
		token:      token.MUL,
		covResult:  notCoveredPosition("testdata/fixtures/mul_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes ARITHMETIC_BASIC with QUO",
		fixture:    "testdata/fixtures/quo_go",
		mutantType: mutator.ArithmeticBase,
		token:      token.QUO,
		covResult:  notCoveredPosition("testdata/fixtures/quo_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes ARITHMETIC_BASIC with REM",
		fixture:    "testdata/fixtures/rem_go",
		mutantType: mutator.ArithmeticBase,
		token:      token.REM,
		covResult:  notCoveredPosition("testdata/fixtures/rem_go"),
		mutStatus:  mutator.NotCovered,
	},
	// INVERT_LOGICAL
	{
		name:       "it recognizes INVERT_LOGICAL with LAND",
		fixture:    "testdata/fixtures/land_go",
		mutantType: mutator.InvertLogical,
		token:      token.LAND,
		covResult:  notCoveredPosition("testdata/fixtures/land_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes INVERT_LOGICAL with LOR",
		fixture:    "testdata/fixtures/lor_go",
		mutantType: mutator.InvertLogical,
		token:      token.LOR,
		covResult:  notCoveredPosition("testdata/fixtures/lor_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes INVERT_LOGICAL with NOT",
		fixture:    "testdata/fixtures/not_go",
		mutantType: mutator.InvertLogical,
		token:      token.NOT,
		covResult:  notCoveredPosition("testdata/fixtures/not_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes INVERT_LOOPCTRL with CONTINUE",
		fixture:    "testdata/fixtures/loop_continue_go",
		mutantType: mutator.InvertLoopCtrl,
		token:      token.CONTINUE,
		covResult:  notCoveredPosition("testdata/fixtures/loop_continue_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes INVERT_LOOPCTRL with BREAK",
		fixture:    "testdata/fixtures/loop_break_go",
		mutantType: mutator.InvertLoopCtrl,
		token:      token.BREAK,
		covResult:  notCoveredPosition("testdata/fixtures/loop_break_go"),
		mutStatus:  mutator.NotCovered,
	},
	// INVERT_ASSIGNMENTS
	{
		name:       "it recognizes INVERT_ASSIGNMENTS with ADD_ASSIGN",
		fixture:    "testdata/fixtures/add_assign_go",
		mutantType: mutator.InvertAssignments,
		token:      token.ADD_ASSIGN,
		covResult:  notCoveredPosition("testdata/fixtures/add_assign_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes INVERT_ASSIGNMENTS with SUB_ASSIGN",
		fixture:    "testdata/fixtures/sub_assign_go",
		mutantType: mutator.InvertAssignments,
		token:      token.SUB_ASSIGN,
		covResult:  notCoveredPosition("testdata/fixtures/sub_assign_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes INVERT_ASSIGNMENTS with MUL_ASSIGN",
		fixture:    "testdata/fixtures/mul_assign_go",
		mutantType: mutator.InvertAssignments,
		token:      token.MUL_ASSIGN,
		covResult:  notCoveredPosition("testdata/fixtures/mul_assign_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes INVERT_ASSIGNMENTS with QUO_ASSIGN",
		fixture:    "testdata/fixtures/quo_assign_go",
		mutantType: mutator.InvertAssignments,
		token:      token.QUO_ASSIGN,
		covResult:  notCoveredPosition("testdata/fixtures/quo_assign_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes INVERT_ASSIGNMENTS with REM_ASSIGN",
		fixture:    "testdata/fixtures/rem_assign_go",
		mutantType: mutator.InvertAssignments,
		token:      token.REM_ASSIGN,
		covResult:  notCoveredPosition("testdata/fixtures/rem_assign_go"),
		mutStatus:  mutator.NotCovered,
	},
	// INVERT_BITWISE
	{
		name:       "it recognizes INVERT_BITWISE with AND",
		fixture:    "testdata/fixtures/and_go",
		mutantType: mutator.InvertBitwise,
		token:      token.AND,
		covResult:  notCoveredPosition("testdata/fixtures/and_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes INVERT_BITWISE with OR",
		fixture:    "testdata/fixtures/or_go",
		mutantType: mutator.InvertBitwise,
		token:      token.OR,
		covResult:  notCoveredPosition("testdata/fixtures/or_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes INVERT_BITWISE with XOR",
		fixture:    "testdata/fixtures/xor_go",
		mutantType: mutator.InvertBitwise,
		token:      token.XOR,
		covResult:  notCoveredPosition("testdata/fixtures/xor_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes INVERT_BITWISE with AND_NOT",
		fixture:    "testdata/fixtures/and_not_go",
		mutantType: mutator.InvertBitwise,
		token:      token.AND_NOT,
		covResult:  notCoveredPosition("testdata/fixtures/and_not_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes INVERT_BITWISE with SHL",
		fixture:    "testdata/fixtures/shl_go",
		mutantType: mutator.InvertBitwise,
		token:      token.SHL,
		covResult:  notCoveredPosition("testdata/fixtures/shl_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes INVERT_BITWISE with SHR",
		fixture:    "testdata/fixtures/shr_go",
		mutantType: mutator.InvertBitwise,
		token:      token.SHR,
		covResult:  notCoveredPosition("testdata/fixtures/shr_go"),
		mutStatus:  mutator.NotCovered,
	},
	// REMOVE_SELF_ASSIGNMENTS
	{
		name:       "it recognizes REMOVE_SELF_ASSIGNMENTS with ADD_ASSIGN",
		fixture:    "testdata/fixtures/add_assign_go",
		mutantType: mutator.RemoveSelfAssignments,
		token:      token.ADD_ASSIGN,
		covResult:  notCoveredPosition("testdata/fixtures/add_assign_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes REMOVE_SELF_ASSIGNMENTS with SUB_ASSIGN",
		fixture:    "testdata/fixtures/sub_assign_go",
		mutantType: mutator.RemoveSelfAssignments,
		token:      token.SUB_ASSIGN,
		covResult:  notCoveredPosition("testdata/fixtures/sub_assign_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes REMOVE_SELF_ASSIGNMENTS with MUL_ASSIGN",
		fixture:    "testdata/fixtures/mul_assign_go",
		mutantType: mutator.RemoveSelfAssignments,
		token:      token.MUL_ASSIGN,
		covResult:  notCoveredPosition("testdata/fixtures/mul_assign_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes REMOVE_SELF_ASSIGNMENTS with QUO_ASSIGN",
		fixture:    "testdata/fixtures/quo_assign_go",
		mutantType: mutator.RemoveSelfAssignments,
		token:      token.QUO_ASSIGN,
		covResult:  notCoveredPosition("testdata/fixtures/quo_assign_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes REMOVE_SELF_ASSIGNMENTS with REM_ASSIGN",
		fixture:    "testdata/fixtures/rem_assign_go",
		mutantType: mutator.RemoveSelfAssignments,
		token:      token.REM_ASSIGN,
		covResult:  notCoveredPosition("testdata/fixtures/rem_assign_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes REMOVE_SELF_ASSIGNMENTS with AND_ASSIGN",
		fixture:    "testdata/fixtures/and_assign_go",
		mutantType: mutator.RemoveSelfAssignments,
		token:      token.AND_ASSIGN,
		covResult:  notCoveredPosition("testdata/fixtures/and_assign_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes REMOVE_SELF_ASSIGNMENTS with OR_ASSIGN",
		fixture:    "testdata/fixtures/or_assign_go",
		mutantType: mutator.RemoveSelfAssignments,
		token:      token.OR_ASSIGN,
		covResult:  notCoveredPosition("testdata/fixtures/or_assign_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes REMOVE_SELF_ASSIGNMENTS with XOR_ASSIGN",
		fixture:    "testdata/fixtures/xor_assign_go",
		mutantType: mutator.RemoveSelfAssignments,
		token:      token.XOR_ASSIGN,
		covResult:  notCoveredPosition("testdata/fixtures/xor_assign_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes REMOVE_SELF_ASSIGNMENTS with SHL_ASSIGN",
		fixture:    "testdata/fixtures/shl_assign_go",
		mutantType: mutator.RemoveSelfAssignments,
		token:      token.SHL_ASSIGN,
		covResult:  notCoveredPosition("testdata/fixtures/shl_assign_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes REMOVE_SELF_ASSIGNMENTS with SHR_ASSIGN",
		fixture:    "testdata/fixtures/shr_assign_go",
		mutantType: mutator.RemoveSelfAssignments,
		token:      token.SHR_ASSIGN,
		covResult:  notCoveredPosition("testdata/fixtures/shr_assign_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes REMOVE_SELF_ASSIGNMENTS with AND_NOT_ASSIGN",
		fixture:    "testdata/fixtures/and_not_assign_go",
		mutantType: mutator.RemoveSelfAssignments,
		token:      token.AND_NOT_ASSIGN,
		covResult:  notCoveredPosition("testdata/fixtures/and_not_assign_go"),
		mutStatus:  mutator.NotCovered,
	},
	// INVERT_BWASSIGN
	{
		name:       "it recognizes INVERT_BWASSIGN with AND_ASSIGN",
		fixture:    "testdata/fixtures/and_assign_go",
		mutantType: mutator.InvertBitwiseAssignments,
		token:      token.AND_ASSIGN,
		covResult:  notCoveredPosition("testdata/fixtures/and_assign_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes INVERT_BWASSIGN with OR_ASSIGN",
		fixture:    "testdata/fixtures/or_assign_go",
		mutantType: mutator.InvertBitwiseAssignments,
		token:      token.OR_ASSIGN,
		covResult:  notCoveredPosition("testdata/fixtures/or_assign_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes INVERT_BWASSIGN with XOR_ASSIGN",
		fixture:    "testdata/fixtures/xor_assign_go",
		mutantType: mutator.InvertBitwiseAssignments,
		token:      token.XOR_ASSIGN,
		covResult:  notCoveredPosition("testdata/fixtures/xor_assign_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes INVERT_BWASSIGN with AND_NOT_ASSIGN",
		fixture:    "testdata/fixtures/and_not_assign_go",
		mutantType: mutator.InvertBitwiseAssignments,
		token:      token.AND_NOT_ASSIGN,
		covResult:  notCoveredPosition("testdata/fixtures/and_not_assign_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes INVERT_BWASSIGN with SHL_ASSIGN",
		fixture:    "testdata/fixtures/shl_assign_go",
		mutantType: mutator.InvertBitwiseAssignments,
		token:      token.SHL_ASSIGN,
		covResult:  notCoveredPosition("testdata/fixtures/shl_assign_go"),
		mutStatus:  mutator.NotCovered,
	},
	{
		name:       "it recognizes INVERT_BWASSIGN with SHR_ASSIGN",
		fixture:    "testdata/fixtures/shr_assign_go",
		mutantType: mutator.InvertBitwiseAssignments,
		token:      token.SHR_ASSIGN,
		covResult:  notCoveredPosition("testdata/fixtures/shr_assign_go"),
		mutStatus:  mutator.NotCovered,
	},
	// Common behaviours
	{
		name:       "it works with recursion",
		fixture:    "testdata/fixtures/geq_land_true_go",
		mutantType: mutator.ConditionalsBoundary,
		token:      token.GEQ,
		covResult:  notCoveredPosition("testdata/fixtures/geq_go"),
		mutStatus:  mutator.NotCovered,
	},
}

func TestMutations(t *testing.T) {
	t.Parallel()
	for _, tc := range mutationsTests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			viperSet(map[string]any{configuration.UnleashDryRunKey: true})
			defer viperReset()

			mapFS, mod := loadFixture(t, tc.fixture, ".")

			mut := engine.New(mod, engine.CodeData{Cov: tc.covResult.Profile}, newJobDealerStub(t), engine.WithDirFs(mapFS))
			res := runEngine(t, mut)
			got := res.Mutants

			if res.Module != expectedModule {
				t.Errorf("Run() Module = %q, want %q", res.Module, expectedModule)
			}

			for _, g := range got {
				if g.Type() == tc.mutantType && g.Status() == tc.mutStatus && g.Pos() > 0 {
					// PASS
					return
				}
			}

			t.Errorf("Run() mutants = %v, want a %v mutant with status %v and a position", got, tc.mutantType, tc.mutStatus)
		})
	}
}

func TestMutantSkipDisabled(t *testing.T) {
	t.Parallel()
	for _, mt := range mutator.Types {
		mt := mt
		t.Run(mt.String(), func(t *testing.T) {
			t.Parallel()
			mapFS, mod := loadFixture(t, defaultFixture, ".")

			viperSet(map[string]any{
				configuration.UnleashDryRunKey:         true,
				configuration.MutantTypeEnabledKey(mt): false,
			})
			defer viperReset()

			mut := engine.New(mod, testCodeData, newJobDealerStub(t), engine.WithDirFs(mapFS))
			res := runEngine(t, mut)
			got := res.Mutants

			for _, m := range got {
				if m.Type() == mt {
					t.Fatalf("Run() with %v disabled returned a %v mutant, want none", mt, mt)
				}
			}
		})
	}
}

func TestDisablingOneMutantTypeDoesNotDisableOthersForSameToken(t *testing.T) {
	t.Parallel()
	mapFS, mod := loadFixture(t, defaultFixture, ".")

	viperSet(map[string]any{
		configuration.UnleashDryRunKey:                                   true,
		configuration.MutantTypeEnabledKey(mutator.ConditionalsBoundary): false,
		configuration.MutantTypeEnabledKey(mutator.ConditionalsNegation): true,
	})
	defer viperReset()

	mut := engine.New(mod, testCodeData, newJobDealerStub(t), engine.WithDirFs(mapFS))
	res := runEngine(t, mut)

	var hasNegation, hasBoundary bool
	for _, m := range res.Mutants {
		if m.Type() == mutator.ConditionalsNegation {
			hasNegation = true
		} else if m.Type() == mutator.ConditionalsBoundary {
			hasBoundary = true
		}
	}

	if !hasNegation {
		t.Fatal("Run() returned no ConditionalsNegation mutants, want at least one")
	}
	if hasBoundary {
		t.Fatal("Run() returned a ConditionalsBoundary mutant, want none")
	}
}

func TestSkipTestAndNonGoFiles(t *testing.T) {
	t.Parallel()
	file := readFixture(t, "testdata/fixtures/geq_go")

	sys := fstest.MapFS{
		"file_test.go": {Data: file},
		"folder1/file": {Data: file},
	}
	mod := gomodule.GoModule{
		Name:       expectedModule,
		Root:       ".",
		CallingDir: ".",
	}
	viperSet(map[string]any{configuration.UnleashDryRunKey: true})
	defer viperReset()
	mut := engine.New(mod, engine.CodeData{}, newJobDealerStub(t), engine.WithDirFs(sys))
	res := runEngine(t, mut)

	if got := res.Mutants; len(got) != 0 {
		t.Error("should not receive results")
	}
}

func TestFeatureSkipTestdataVendorAndNestedModuleTrees(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile("testdata/fixtures/geq_go")
	if err != nil {
		t.Fatal(err)
	}

	sys := fstest.MapFS{
		"pkg/file.go":         {Data: src},
		"testdata/fixture.go": {Data: src},
		"vendor/dep/file.go":  {Data: src},
		"nested/go.mod":       {Data: []byte("module nested\n\ngo 1.25\n")},
		"nested/file.go":      {Data: src},
	}
	mod := gomodule.GoModule{
		Name:       expectedModule,
		Root:       ".",
		CallingDir: ".",
	}
	viperSet(map[string]any{configuration.UnleashDryRunKey: true})
	defer viperReset()
	mut := engine.New(mod, engine.CodeData{}, newJobDealerStub(t), engine.WithDirFs(sys))
	res := runEngine(t, mut)

	files := make(map[string]int)
	for _, m := range res.Mutants {
		files[m.Position().Filename]++
	}
	if files["pkg/file.go"] == 0 {
		t.Errorf("got no mutants of pkg/file.go, want mutants; mutants by file: %v", files)
	}
	for _, excluded := range []string{"testdata/fixture.go", "vendor/dep/file.go", "nested/file.go"} {
		if files[excluded] != 0 {
			t.Errorf("got mutants of %s, want none; mutants by file: %v", excluded, files)
		}
	}
}

// TestRegressionSkipFilesTheGoToolIgnores checks that Run mutates no file
// that the go tool ignores: a name that starts with "." or "_", or a file in
// such a directory, for example a Go file under .git.
func TestRegressionSkipFilesTheGoToolIgnores(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile("testdata/fixtures/geq_go")
	if err != nil {
		t.Fatal(err)
	}

	const mutated = "pkg/file.go"
	ignored := []string{".git/review/probe.go", "_tools/tool.go", "pkg/.hidden.go", "pkg/_ignored.go"}
	sys := fstest.MapFS{mutated: {Data: src}}
	for _, name := range ignored {
		sys[name] = &fstest.MapFile{Data: src}
	}
	mod := gomodule.GoModule{
		Name:       expectedModule,
		Root:       ".",
		CallingDir: ".",
	}
	viperSet(map[string]any{configuration.UnleashDryRunKey: true})
	defer viperReset()
	mut := engine.New(mod, engine.CodeData{}, newJobDealerStub(t), engine.WithDirFs(sys))
	res := runEngine(t, mut)

	files := make(map[string]int)
	for _, m := range res.Mutants {
		files[m.Position().Filename]++
	}
	if files[mutated] == 0 {
		t.Errorf("got no mutants of %s, want mutants; mutants by file: %v", mutated, files)
	}
	for _, name := range ignored {
		if files[name] != 0 {
			t.Errorf("got mutants of %s, want none; mutants by file: %v", name, files)
		}
	}
}

const (
	goodFile   = "good.go"
	lockedFile = "locked.go"
)

// errInjected is the failure that failingFS returns.
var errInjected = errors.New("injected failure")

// failingFS is a fstest.MapFS that fails to open the file failOpen and to
// read the directory failReadDir.
type failingFS struct {
	fstest.MapFS
	failOpen    string
	failReadDir string
}

func (f failingFS) Open(name string) (fs.File, error) {
	if name == f.failOpen {
		return nil, &fs.PathError{Op: "open", Path: name, Err: errInjected}
	}

	return f.MapFS.Open(name)
}

func (f failingFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name == f.failReadDir {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: errInjected}
	}

	return f.MapFS.ReadDir(name)
}

func TestFeatureRunReturnsDiscoveryErrors(t *testing.T) {
	src, err := os.ReadFile("testdata/fixtures/geq_go")
	if err != nil {
		t.Fatal(err)
	}
	broken := []byte("package main\n\nfunc main() {\n  if 1 >= 2 {\n")
	illegal, err := os.ReadFile("testdata/fixtures/illegal_go")
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name     string
		fsys     fs.FS
		wantIs   error
		wantPath string
	}{
		{
			name:     "malformed Go source",
			fsys:     fstest.MapFS{goodFile: {Data: src}, "broken.go": {Data: broken}},
			wantPath: "broken.go",
		},
		{
			name:     "illegal token",
			fsys:     fstest.MapFS{"illegal.go": {Data: illegal}},
			wantPath: "illegal.go",
		},
		{
			name: "file open failure",
			fsys: failingFS{
				MapFS:    fstest.MapFS{goodFile: {Data: src}, lockedFile: {Data: src}},
				failOpen: lockedFile,
			},
			wantIs:   errInjected,
			wantPath: lockedFile,
		},
		{
			name: "directory read failure",
			fsys: failingFS{
				MapFS:       fstest.MapFS{goodFile: {Data: src}, "pkg/file.go": {Data: src}},
				failReadDir: "pkg",
			},
			wantIs:   errInjected,
			wantPath: "pkg",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mod := gomodule.GoModule{Name: expectedModule, Root: ".", CallingDir: "."}
			viperSet(map[string]any{configuration.UnleashDryRunKey: true})
			defer viperReset()
			mut := engine.New(mod, engine.CodeData{}, newJobDealerStub(t), engine.WithDirFs(tc.fsys))

			_, err := mut.Run(context.Background())

			if err == nil {
				t.Fatal("Run() error = nil, want a discovery error")
			}
			if tc.wantIs != nil && !errors.Is(err, tc.wantIs) {
				t.Errorf("Run() error = %v, want it to wrap %v", err, tc.wantIs)
			}
			if !strings.Contains(err.Error(), tc.wantPath) {
				t.Errorf("Run() error = %v, want it to name %q", err, tc.wantPath)
			}
		})
	}
}

func TestFeatureRunWalksDirectoryNamedAsGoFile(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile("testdata/fixtures/geq_go")
	if err != nil {
		t.Fatal(err)
	}

	sys := fstest.MapFS{
		goodFile:            {Data: src},
		"directory.go/x.go": {Data: src},
	}
	mod := gomodule.GoModule{Name: expectedModule, Root: ".", CallingDir: "."}
	viperSet(map[string]any{configuration.UnleashDryRunKey: true})
	defer viperReset()
	mut := engine.New(mod, engine.CodeData{}, newJobDealerStub(t), engine.WithDirFs(sys))
	res := runEngine(t, mut)

	files := make(map[string]int)
	for _, m := range res.Mutants {
		files[m.Position().Filename]++
	}
	if files[goodFile] == 0 || files["directory.go/x.go"] == 0 {
		t.Errorf("got no mutants of good.go or directory.go/x.go, want mutants; mutants by file: %v", files)
	}
	if files["directory.go"] != 0 {
		t.Errorf("got mutants of the directory directory.go, want none; mutants by file: %v", files)
	}
}

// runEngine runs mut and fails the test when Run returns an error.
func runEngine(t *testing.T, mut engine.Engine) report.Results {
	t.Helper()
	res, err := mut.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	return res
}

func TestSkipNotDiffMutants(t *testing.T) {
	t.Parallel()
	file := readFixture(t, "testdata/fixtures/geq_go")

	sys := fstest.MapFS{
		"file.go": {Data: file},
	}
	mod := gomodule.GoModule{
		Name:       expectedModule,
		Root:       ".",
		CallingDir: ".",
	}
	viperSet(map[string]any{configuration.UnleashDryRunKey: true})
	defer viperReset()

	codeData := engine.CodeData{Diff: diff.Diff{
		"file.go": nil,
	}}
	mut := engine.New(mod, codeData, newJobDealerStub(t), engine.WithDirFs(sys))
	res := runEngine(t, mut)

	if got := res.Mutants; len(got) == 0 {
		t.Error("should receive mutants")
	}

	for _, mutant := range res.Mutants {
		if mutant.Status() != mutator.Skipped {
			t.Errorf("all mutants should be skipped")
		}
	}
}

// TestRegressionSkipMutantsThatCoverageCannotObserve checks that a mutant in
// a package-level constant declaration, or in a file that the host build
// excludes, is skipped and not reported as not covered. The other mutants
// follow the coverage profile, with or without coverage blocks.
func TestRegressionSkipMutantsThatCoverageCannotObserve(t *testing.T) {
	otherOS := "windows"
	if runtime.GOOS == otherOS {
		otherOS = "linux"
	}
	hostFile := "host_" + runtime.GOOS + ".go"
	otherFile := "other_" + otherOS + ".go"
	const constFile = "consts.go"
	fn := []byte("package p\n\nfunc g(a int) bool { return a > 0 }\n")
	sys := fstest.MapFS{
		constFile: {Data: []byte("package p\n\nconst scaled = 5 * 2\n\n" +
			"func f(a int) int {\n\tconst local = 3 * 4\n\n\treturn a * local\n}\n")},
		hostFile:     {Data: fn},
		otherFile:    {Data: fn},
		"ignored.go": {Data: append([]byte("//go:build ignore\n\n"), fn...)},
		"tagged.go":  {Data: append([]byte("//go:build gremlinstag\n\n"), fn...)},
	}
	full := coverage.Profile{}
	for name := range sys {
		full[name] = []coverage.Block{{StartLine: 1, StartCol: 1, EndLine: 100, EndCol: 1}}
	}
	// observable maps each line with mutants to whether coverage can observe it.
	observable := map[mutantLine]bool{
		{constFile, 3}:    false,
		{constFile, 6}:    true,
		{constFile, 8}:    true,
		{hostFile, 3}:     true,
		{otherFile, 3}:    false,
		{"ignored.go", 5}: false,
		{"tagged.go", 5}:  true,
	}
	tests := []struct {
		name          string
		profile       coverage.Profile
		observedState mutator.Status
	}{
		{name: "every line covered", profile: full, observedState: mutator.Runnable},
		{name: "no coverage blocks", profile: coverage.Profile{}, observedState: mutator.NotCovered},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mod := gomodule.GoModule{Name: expectedModule, Root: ".", CallingDir: "."}
			viperSet(map[string]any{
				configuration.UnleashDryRunKey: true,
				configuration.UnleashTagsKey:   "other,gremlinstag",
			})
			defer viperReset()

			mut := engine.New(mod, engine.CodeData{Cov: tc.profile}, newJobDealerStub(t), engine.WithDirFs(sys))
			res := runEngine(t, mut)

			checkObservableStatuses(t, res.Mutants, observable, tc.observedState)
		})
	}
}

// mutantLine is a source line of a mutant.
type mutantLine struct {
	file string
	line int
}

// checkObservableStatuses checks that each mutant is on a line of observable
// and that each line of observable has a mutant. A mutant on an observable
// line must have the status observedState. The other mutants must be skipped.
func checkObservableStatuses(
	t *testing.T, mutants []mutator.Mutator, observable map[mutantLine]bool, observedState mutator.Status,
) {
	t.Helper()
	want := make(map[mutantLine]mutator.Status, len(observable))
	for at, obs := range observable {
		want[at] = mutator.Skipped
		if obs {
			want[at] = observedState
		}
	}
	checkLineStatuses(t, mutants, want)
}

// checkLineStatuses checks that each mutant is on a line of want and has the
// status of that line, and that each line of want has a mutant.
func checkLineStatuses(t *testing.T, mutants []mutator.Mutator, want map[mutantLine]mutator.Status) {
	t.Helper()
	found := make(map[mutantLine]bool)
	for _, m := range mutants {
		at := mutantLine{m.Position().Filename, m.Position().Line}
		wantStatus, ok := want[at]
		if !ok {
			t.Errorf("got %s mutant at %s, want no mutant on that line", m.Type(), m.Position())

			continue
		}
		found[at] = true
		if m.Status() != wantStatus {
			t.Errorf("%s mutant at %s: status = %s, want %s", m.Type(), m.Position(), m.Status(), wantStatus)
		}
	}
	for at := range want {
		if !found[at] {
			t.Errorf("got no mutant at %s:%d, want at least one", at.file, at.line)
		}
	}
}

// TestRegressionPackageVarInitializerCoverage checks the status of a mutant
// in a package-level variable initializer. The profile has blocks only for
// function bodies, as a real profile does. An executed block in any file of
// the package proves that the initializers ran, so the mutant is runnable.
// Without such a block the profile cannot show the initializer, so the mutant
// is skipped. A function literal in an initializer follows its own blocks.
func TestRegressionPackageVarInitializerCoverage(t *testing.T) {
	const (
		ranFile  = "ran/vars.go"
		onlyFile = "ran/only.go"
		idleFile = "idle/vars.go"
	)
	sys := fstest.MapFS{
		ranFile: {Data: []byte("package ran\n\nvar delay = 5 * 2\n\n" +
			"var check = func(a int) bool { return a > 0 }\n\n" +
			"func F() int { return delay * 3 }\n")},
		onlyFile: {Data: []byte("package ran\n\nvar size = 2 + 3\n")},
		idleFile: {Data: []byte("package idle\n\nvar size = 2 + 3\n\nfunc G(a int) int { return a - 1 }\n")},
	}
	// The block is the body of F. No test ran check or G.
	profile := coverage.Profile{ranFile: {{StartLine: 7, StartCol: 14, EndLine: 7, EndCol: 34}}}
	want := map[mutantLine]mutator.Status{
		{ranFile, 3}:  mutator.Runnable,
		{ranFile, 5}:  mutator.NotCovered,
		{ranFile, 7}:  mutator.Runnable,
		{onlyFile, 3}: mutator.Runnable,
		{idleFile, 3}: mutator.Skipped,
		{idleFile, 5}: mutator.NotCovered,
	}
	mod := gomodule.GoModule{Name: expectedModule, Root: ".", CallingDir: "."}
	viperSet(map[string]any{configuration.UnleashDryRunKey: true})
	defer viperReset()

	mut := engine.New(mod, engine.CodeData{Cov: profile}, newJobDealerStub(t), engine.WithDirFs(sys))
	res := runEngine(t, mut)

	checkLineStatuses(t, res.Mutants, want)
}

// TestRegressionSkipTypeDeclarationsAndSignatures checks that a mutant in a
// package-level type declaration or in the signature of a function, a method,
// or a function literal of a package-level variable is skipped. The compiler
// evaluates these expressions, so no test run can cover them. The mutants in
// the function bodies follow the coverage profile.
func TestRegressionSkipTypeDeclarationsAndSignatures(t *testing.T) {
	const sigFile = "sig.go"
	sys := fstest.MapFS{
		sigFile: {Data: []byte("package p\n\n" +
			"type Buf [4 * 2]byte\n\n" +
			"var check = func(b [2 + 1]int) int {\n\treturn len(b) * 2\n}\n\n" +
			"type T struct{}\n\n" +
			"func (T) M(b [1 + 1]int) (r [3 - 1]int) {\n\treturn [2]int{b[0] * 2, b[1]}\n}\n")},
	}
	// observable maps each line with mutants to whether coverage can observe it.
	observable := map[mutantLine]bool{
		{sigFile, 3}:  false,
		{sigFile, 5}:  false,
		{sigFile, 6}:  true,
		{sigFile, 11}: false,
		{sigFile, 12}: true,
	}
	full := coverage.Profile{sigFile: {{StartLine: 1, StartCol: 1, EndLine: 100, EndCol: 1}}}
	tests := []struct {
		name          string
		profile       coverage.Profile
		observedState mutator.Status
	}{
		{name: "every line covered", profile: full, observedState: mutator.Runnable},
		{name: "no coverage blocks", profile: coverage.Profile{}, observedState: mutator.NotCovered},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mod := gomodule.GoModule{Name: expectedModule, Root: ".", CallingDir: "."}
			viperSet(map[string]any{configuration.UnleashDryRunKey: true})
			defer viperReset()

			mut := engine.New(mod, engine.CodeData{Cov: tc.profile}, newJobDealerStub(t), engine.WithDirFs(sys))
			res := runEngine(t, mut)

			checkObservableStatuses(t, res.Mutants, observable, tc.observedState)
		})
	}
}

// TestRegressionSkipTypeConstraintOperators checks that the operators of the
// type constraints of interfaces and type parameters produce no mutant. The |
// of a type union is a type term and not a bitwise operation. The | in the
// function body is a bitwise operation and produces mutants.
func TestRegressionSkipTypeConstraintOperators(t *testing.T) {
	const genFile = "gen.go"
	sys := fstest.MapFS{
		genFile: {Data: []byte("package p\n\n" +
			"type Number interface {\n\t~int | ~int64\n\tString() string\n}\n\n" +
			"type Pair[K ~int | ~string, V any] struct{ k K }\n\n" +
			"func Or[T ~int | ~uint](a, b T) T {\n\treturn a | b\n}\n\n" +
			"var f = func(a, b int) int { return a | b }\n")},
	}
	mod := gomodule.GoModule{Name: expectedModule, Root: ".", CallingDir: "."}
	viperSet(map[string]any{
		configuration.UnleashDryRunKey:                            true,
		configuration.MutantTypeEnabledKey(mutator.InvertBitwise): true,
	})
	defer viperReset()

	full := coverage.Profile{genFile: {{StartLine: 1, StartCol: 1, EndLine: 100, EndCol: 1}}}
	mut := engine.New(mod, engine.CodeData{Cov: full}, newJobDealerStub(t), engine.WithDirFs(sys))
	res := runEngine(t, mut)

	lines := make(map[int]bool)
	for _, m := range res.Mutants {
		lines[m.Position().Line] = true
	}
	want := map[int]bool{11: true, 14: true}
	if !maps.Equal(lines, want) {
		t.Errorf("Run() mutant lines = %v, want %v", lines, want)
	}
}

// lifecycleFS is a module file with eight mutants: each comparison has a
// conditionals-boundary and a conditionals-negation mutant.
var lifecycleFS = fstest.MapFS{
	"lifecycle.go": {Data: []byte("package lifecycle\n\n" +
		"func f(a, b int) (bool, bool, bool, bool) { return a > b, a < b, a >= b, a <= b }\n")},
}

// The synctest bubbles below fail if Run leaves a goroutine of the engine,
// the discovery, or the worker pool blocked after it returns.

func TestFeatureEngineRunReleasesWorkersOnCompletion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		viperSet(map[string]any{configuration.UnleashWorkersKey: 2})
		defer viperReset()
		mod := gomodule.GoModule{Name: expectedModule, Root: ".", CallingDir: "."}
		mut := engine.New(mod, engine.CodeData{}, newJobDealerStub(t), engine.WithDirFs(lifecycleFS))

		res, err := mut.Run(context.Background())
		if err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}

		if got, want := len(res.Mutants), 8; got != want {
			t.Errorf("Run reported %d mutants, want %d", got, want)
		}
	})
}

func TestStopsOnCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mod := gomodule.GoModule{Name: expectedModule, Root: ".", CallingDir: "."}
		mut := engine.New(mod, engine.CodeData{}, newJobDealerStub(t), engine.WithDirFs(lifecycleFS))

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		res, err := mut.Run(ctx)
		if err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}

		if len(res.Mutants) > 0 {
			t.Errorf("Run() returned %d mutants, want 0", len(res.Mutants))
		}
	})
}

func TestFeatureEngineRunCancelReleasesBlockedSubmission(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		viperSet(map[string]any{configuration.UnleashWorkersKey: 1})
		defer viperReset()
		mod := gomodule.GoModule{Name: expectedModule, Root: ".", CallingDir: "."}
		dealer := &blockingDealerStub{}
		mut := engine.New(mod, engine.CodeData{}, dealer, engine.WithDirFs(lifecycleFS))
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		type runResult struct {
			err error
			res report.Results
		}
		done := make(chan runResult, 1)
		go func() {
			res, err := mut.Run(ctx)
			done <- runResult{err: err, res: res}
		}()
		synctest.Wait()

		// The worker holds the first executor and the queue holds the second,
		// so the third submission and the discovery are blocked.
		if got, want := dealer.created.Load(), int32(3); got != want {
			t.Fatalf("executors created before cancellation = %d, want %d", got, want)
		}
		cancel()
		got := <-done

		if got.err != nil {
			t.Errorf("Run after cancellation error = %v, want nil", got.err)
		}
		if res := got.res; len(res.Mutants) > 0 {
			t.Errorf("Run after cancellation reported %d mutants, want 0", len(res.Mutants))
		}
	})
}

// TestFeatureRunReturnsExecutorError proves that a failed executor fails
// the run instead of leaving a missing mutant in successful results
// (issue #106).
func TestFeatureRunReturnsExecutorError(t *testing.T) {
	mapFS, mod := loadFixture(t, defaultFixture, ".")

	errExecutor := errors.New("executor stub error")
	var failed mutator.Mutator
	jds := newJobDealerStub(t)
	jds.errFor = func(mut mutator.Mutator) error {
		if failed != nil {
			return nil
		}
		failed = mut

		return errExecutor
	}
	mut := engine.New(mod, testCodeData, jds, engine.WithDirFs(mapFS))

	res, err := mut.Run(context.Background())

	if !errors.Is(err, errExecutor) {
		t.Fatalf("Run() error = %v, want %v", err, errExecutor)
	}
	for _, m := range res.Mutants {
		if m == failed {
			t.Errorf("Run() results contain the failed mutant at %s", m.Position())
		}
	}
}

func TestPackageDiscovery(t *testing.T) {
	testCases := []struct {
		name     string
		fromPkg  string
		wantPath string
		intMode  bool
	}{
		{
			name:     "from root, normal mode",
			fromPkg:  ".",
			intMode:  false,
			wantPath: "example.com/fixtures",
		},
		{
			name:     "from subpackage, normal mode",
			fromPkg:  "testdata/main/fixture",
			intMode:  false,
			wantPath: "example.com/testdata/main/fixture/fixtures",
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			viperSet(map[string]any{
				configuration.UnleashIntegrationMode: tc.intMode,
				configuration.UnleashTagsKey:         "tag1 tag2",
			})
			defer viperReset()
			mapFS, mod := loadFixture(t, defaultFixture, tc.fromPkg)

			jds := newJobDealerStub(t)
			mut := engine.New(mod, testCodeData, jds, engine.WithDirFs(mapFS))

			runEngine(t, mut)

			got := jds.gotMutants[0].Pkg()

			if got != tc.wantPath {
				t.Errorf("mutant Pkg() = %q, want %q", got, tc.wantPath)
			}
		})
	}
}

// applyingDealerStub creates executors that apply and roll back each mutant
// in its own copy of src while the discovery runs, as the real executors do.
type applyingDealerStub struct {
	base     string
	fileName string
	src      []byte
}

func (d *applyingDealerStub) NewExecutor(
	mut mutator.Mutator, outCh chan<- engine.ExecutorResult, wg *sync.WaitGroup,
) workerpool.Executor {
	return &applyingExecutorStub{mut: mut, dealer: d, outCh: outCh, wg: wg}
}

type applyingExecutorStub struct {
	mut    mutator.Mutator
	dealer *applyingDealerStub
	outCh  chan<- engine.ExecutorResult
	wg     *sync.WaitGroup
}

func (e *applyingExecutorStub) Start(_ context.Context, _ *workerpool.Worker) {
	defer e.wg.Done()
	e.outCh <- engine.ExecutorResult{Mutant: e.mut, Err: e.applyAndRollback()}
}

func (e *applyingExecutorStub) applyAndRollback() error {
	workdir, err := os.MkdirTemp(e.dealer.base, "mutant")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(workdir, e.dealer.fileName), e.dealer.src, 0o600); err != nil {
		return err
	}
	e.mut.SetWorkdir(workdir)
	if err := e.mut.Apply(); err != nil {
		return err
	}

	return e.mut.Rollback()
}

// TestFeatureRunDiscoversOnlyOriginalNegations checks that the discovery of
// a file never sees the ! that a running INVERT_LOGICAL mutant adds to the
// shared AST. Each original ! gives exactly one mutant. Run the test with the
// race detector to check that discovery and Apply do not share the AST.
func TestFeatureRunDiscoversOnlyOriginalNegations(t *testing.T) {
	const src = "package main\n\nfunc f(a, b bool) bool {\n\tx := !a\n\ty := !!b\n\tz := !(a && !b)\n\n\treturn !x && !y && !z\n}\n"
	const fileName = "negations.go"
	want := []string{"4:7", "5:7", "5:8", "6:14", "6:7", "8:15", "8:21", "8:9"}

	enabled := make(map[string]any, len(mutator.Types))
	for _, mt := range mutator.Types {
		enabled[configuration.MutantTypeEnabledKey(mt)] = mt == mutator.InvertLogical
	}
	viperSet(enabled)
	defer viperReset()
	cov := coverage.Profile{fileName: {{StartLine: 1, EndLine: 9, StartCol: 1, EndCol: 2}}}
	mod := gomodule.GoModule{Name: expectedModule, Root: ".", CallingDir: "."}
	mapFS := fstest.MapFS{fileName: {Data: []byte(src)}}
	dealer := &applyingDealerStub{base: t.TempDir(), fileName: fileName, src: []byte(src)}

	mut := engine.New(mod, engine.CodeData{Cov: cov}, dealer, engine.WithDirFs(mapFS))
	res := runEngine(t, mut)

	var got []string
	for _, m := range res.Mutants {
		pos := m.Position()
		if m.Type() != mutator.InvertLogical {
			t.Errorf("mutant at %s has type %s, want %s", pos, m.Type(), mutator.InvertLogical)
		}
		if src[pos.Offset] == '!' {
			got = append(got, fmt.Sprintf("%d:%d", pos.Line, pos.Column))
		}
	}
	slices.Sort(got)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Run() ! mutant positions mismatch (-want +got):\n%s", diff)
	}
}
