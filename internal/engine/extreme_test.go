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
	"context"
	"fmt"
	"io/fs"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/google/go-cmp/cmp"

	"github.com/OpenRAE/go-rae-gremlins/internal/configuration"
	"github.com/OpenRAE/go-rae-gremlins/internal/coverage"
	"github.com/OpenRAE/go-rae-gremlins/internal/diff"
	"github.com/OpenRAE/go-rae-gremlins/internal/engine"
	"github.com/OpenRAE/go-rae-gremlins/internal/gomodule"
)

// extremeFile is the file name of extremeEngineSource.
const extremeFile = "p.go"

// extremeEngineSource has four functions with extreme mutants. The tests
// covered the body lines 4, 13, 18, and 19, and not line 8.
const extremeEngineSource = `package p

func covered(a int) int {
	return a * 2
}

func uncovered(a int) int {
	return a + 1
}

//gremlins:ignore EXTREME_ZERO
func ignored(a int) int {
	return a - 1
}

//gremlins:equivalent EXTREME_CONSTANT the caller ignores the result
func annotated() int {
	x := 1
	return x
}
`

// extremeEngineProfile covers the body lines of extremeEngineSource except
// line 8.
var extremeEngineProfile = coverage.Profile{extremeFile: {
	{StartLine: 4, StartCol: 2, EndLine: 4, EndCol: 14},
	{StartLine: 13, StartCol: 2, EndLine: 13, EndCol: 14},
	{StartLine: 18, StartCol: 2, EndLine: 19, EndCol: 10},
}}

// runExtremeEngine runs a dry run of extremeEngineSource with the Descartes
// group and returns each mutant as "line:column TYPE STATUS", sorted.
func runExtremeEngine(t *testing.T, d diff.Diff) []string {
	t.Helper()
	viperSet(map[string]any{
		configuration.UnleashDryRunKey:    true,
		configuration.UnleashDescartesKey: true,
	})
	defer viperReset()
	mod := gomodule.GoModule{Name: expectedModule, Root: ".", CallingDir: "."}
	sys := fstest.MapFS{extremeFile: {Data: []byte(extremeEngineSource)}}
	mut := engine.New(mod, engine.CodeData{Cov: extremeEngineProfile, Diff: d}, newJobDealerStub(t), engine.WithDirFs(sys))
	res := runEngine(t, mut)

	got := make([]string, 0, len(res.Mutants))
	for _, m := range res.Mutants {
		got = append(got, fmt.Sprintf("%d:%d %s %s", m.Position().Line, m.Position().Column, m.Type(), m.Status()))
	}
	slices.Sort(got)

	return got
}

// TestFeatureRunFindsExtremeMutants checks the extreme mutants of a run with
// the Descartes group (ADR 0012). The group runs no other type, although the
// test configuration enables all the per-type keys. A mutant is at the func
// keyword. The coverage of the first body statement gives its status. The
// ignore and equivalent directives apply to it.
func TestFeatureRunFindsExtremeMutants(t *testing.T) {
	want := []string{
		"12:1 EXTREME_CONSTANT RUNNABLE",
		"17:1 EXTREME_CONSTANT EQUIVALENT",
		"17:1 EXTREME_ZERO RUNNABLE",
		"3:1 EXTREME_CONSTANT RUNNABLE",
		"3:1 EXTREME_ZERO RUNNABLE",
		"7:1 EXTREME_CONSTANT NOT COVERED",
		"7:1 EXTREME_ZERO NOT COVERED",
	}

	got := runExtremeEngine(t, nil)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Run() extreme mutants mismatch (-want +got):\n%s", diff)
	}
}

// TestFeatureRunSelectsExtremeMutantsByDeclarationLines checks that a change
// on a body line selects the extreme mutants of its function, and that the
// mutants of the other functions are skipped.
func TestFeatureRunSelectsExtremeMutantsByDeclarationLines(t *testing.T) {
	want := []string{
		"12:1 EXTREME_CONSTANT RUNNABLE",
		"17:1 EXTREME_CONSTANT SKIPPED",
		"17:1 EXTREME_ZERO SKIPPED",
		"3:1 EXTREME_CONSTANT SKIPPED",
		"3:1 EXTREME_ZERO SKIPPED",
		"7:1 EXTREME_CONSTANT SKIPPED",
		"7:1 EXTREME_ZERO SKIPPED",
	}

	got := runExtremeEngine(t, diff.Diff{extremeFile: {{StartLine: 13, EndLine: 13}}})
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Run() extreme mutants with a diff of line 13 mismatch (-want +got):\n%s", diff)
	}
}

// siblingSource is a file with one bool function. Its extreme mutants depend
// on the names that the other files of the package declare.
const siblingSource = `package p

func ok(a int) bool {
	return a > 0
}
`

// runSiblings runs a dry run of siblingSource in p.go with the extra files of
// sys and the Descartes group. It returns the types of the mutants of p.go.
func runSiblings(t *testing.T, sys fstest.MapFS) []string {
	t.Helper()
	viperSet(map[string]any{
		configuration.UnleashDryRunKey:    true,
		configuration.UnleashDescartesKey: true,
	})
	defer viperReset()
	sys[extremeFile] = &fstest.MapFile{Data: []byte(siblingSource)}
	mod := gomodule.GoModule{Name: expectedModule, Root: ".", CallingDir: "."}
	mut := engine.New(mod, engine.CodeData{}, newJobDealerStub(t), engine.WithDirFs(sys))
	res, err := mut.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v, want nil: a sibling file must not stop the run", err)
	}

	var got []string
	for _, m := range res.Mutants {
		got = append(got, m.Type().String())
	}
	slices.Sort(got)

	return got
}

// TestRegressionPackageNamesOfSiblingFiles checks the package-level names
// that the extreme specs read from the other files of the directory. Names of
// another package do not count. A sibling that cannot be read or parsed does
// not stop the run; its names are not known, so no predeclared identifier
// counts. A sibling that is not a regular file is not read.
func TestRegressionPackageNamesOfSiblingFiles(t *testing.T) {
	both := []string{"EXTREME_CONSTANT", "EXTREME_ZERO"}
	testCases := map[string]struct {
		sys  fstest.MapFS
		want []string
	}{
		"no siblings": {sys: fstest.MapFS{}, want: both},
		"same package shadows true": {
			sys:  fstest.MapFS{"names.go": {Data: []byte("package p\n\nvar true = 2 > 1\n")}},
			want: []string{"EXTREME_ZERO"},
		},
		"external test package": {
			sys:  fstest.MapFS{"p_test.go": {Data: []byte("package p_test\n\nconst true = false\n")}},
			want: both,
		},
		"malformed test file": {
			sys:  fstest.MapFS{"broken_test.go": {Data: []byte("package p\n\nfunc {\n")}},
			want: nil,
		},
		"symbolic link": {
			sys:  fstest.MapFS{"link_test.go": {Data: []byte("package p\n\nvar x = 1\n"), Mode: fs.ModeSymlink}},
			want: nil,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(tc.want, runSiblings(t, tc.sys)); diff != "" {
				t.Errorf("extreme mutants of p.go mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestRegressionDiffSelectionUsesPhysicalLines checks that the diff
// selection of a function and of an operator uses the physical lines of the
// file. A //line directive in the function changes only the reported
// position of the operator.
func TestRegressionDiffSelectionUsesPhysicalLines(t *testing.T) {
	const src = "package p\n\nfunc f(a int) int {\n//line other.go:100\n\treturn a * 2\n}\n"
	viperSet(map[string]any{configuration.UnleashDryRunKey: true})
	defer viperReset()
	mod := gomodule.GoModule{Name: expectedModule, Root: ".", CallingDir: "."}
	sys := fstest.MapFS{extremeFile: {Data: []byte(src)}}
	codeData := engine.CodeData{
		Cov:  coverage.Profile{"other.go": {{StartLine: 99, StartCol: 1, EndLine: 101, EndCol: 1}}},
		Diff: diff.Diff{extremeFile: {{StartLine: 5, EndLine: 5}}},
	}
	mut := engine.New(mod, codeData, newJobDealerStub(t), engine.WithDirFs(sys))
	res := runEngine(t, mut)

	got := make([]string, 0, len(res.Mutants))
	for _, m := range res.Mutants {
		got = append(got, fmt.Sprintf("%s %s", m.Type(), m.Status()))
	}
	slices.Sort(got)
	want := []string{"ARITHMETIC_BASE RUNNABLE", "EXTREME_CONSTANT RUNNABLE", "EXTREME_ZERO RUNNABLE"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Run() mutants with a diff of physical line 5 mismatch (-want +got):\n%s", diff)
	}
}
