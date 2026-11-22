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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/OpenRAE/go-rae-gremlins/internal/configuration"
	"github.com/OpenRAE/go-rae-gremlins/internal/coverage"
	"github.com/OpenRAE/go-rae-gremlins/internal/diff"
	"github.com/OpenRAE/go-rae-gremlins/internal/engine"
	"github.com/OpenRAE/go-rae-gremlins/internal/gomodule"
	"github.com/OpenRAE/go-rae-gremlins/internal/log"
	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
	"github.com/OpenRAE/go-rae-gremlins/internal/report"
)

// annotatedSrc has a same-line annotation of the first < on line 4 and a
// next-line annotation of the < on line 9. In both lines, the first < is at
// column 11. The second < of line 4 is at column 20.
const annotatedSrc = `package main

func f(a, b, c, d int) bool {
	return a < b && c < d //gremlins:equivalent CONDITIONALS_BOUNDARY@11 the bound is unreachable
}

func g(a, b int) bool {
	//gremlins:equivalent CONDITIONALS_NEGATION the result does not reach the caller
	return a < b
}
`

// plainSrc has the same operators as annotatedSrc and no annotation.
const plainSrc = `package main

func h(a, b, c, d int) bool {
	return a < b && c < d
}
`

const (
	boundaryReason = "the bound is unreachable"
	negationReason = "the result does not reach the caller"

	fileA            = "a.go"
	fileBad          = "bad.go"
	boundaryAt4Col11 = "a.go:4:11 CONDITIONALS_BOUNDARY"
	negationAt4Col11 = "a.go:4:11 CONDITIONALS_NEGATION"
)

var equivalentModule = gomodule.GoModule{Name: expectedModule, Root: ".", CallingDir: "."}

// wholeFileProfile covers every line of the named files.
func wholeFileProfile(files ...string) coverage.Profile {
	p := coverage.Profile{}
	for _, f := range files {
		p[f] = []coverage.Block{{StartLine: 1, StartCol: 1, EndLine: 100, EndCol: 1}}
	}

	return p
}

func mutantKey(m mutator.Mutator) string {
	pos := m.Position()

	return fmt.Sprintf("%s:%d:%d %s", pos.Filename, pos.Line, pos.Column, m.Type())
}

// runEquivalentEngine runs the engine on sys with the settings and returns
// the results and the error of the run.
func runEquivalentEngine(
	t *testing.T, sys fstest.MapFS, codeData engine.CodeData, settings map[string]any,
) (report.Results, error) {
	t.Helper()
	settings[configuration.UnleashDryRunKey] = true
	viperSet(settings)
	defer viperReset()

	mut := engine.New(equivalentModule, codeData, newJobDealerStub(t), engine.WithDirFs(sys))

	return mut.Run(context.Background())
}

// mutantsByKey indexes mutants by file, line, column, and type, and fails
// the test when two mutants have the same key.
func mutantsByKey(t *testing.T, mutants []mutator.Mutator) map[string]mutator.Mutator {
	t.Helper()
	byKey := make(map[string]mutator.Mutator, len(mutants))
	for _, m := range mutants {
		k := mutantKey(m)
		if _, dup := byKey[k]; dup {
			t.Fatalf("Run() returned two mutants for %s", k)
		}
		byKey[k] = m
	}

	return byKey
}

type wantMutant struct {
	status mutator.Status
	reason string
}

func checkMutants(t *testing.T, got map[string]mutator.Mutator, want map[string]wantMutant) {
	t.Helper()
	for k, w := range want {
		m, ok := got[k]
		if !ok {
			t.Errorf("Run() has no mutant %s, want one", k)

			continue
		}
		if m.Status() != w.status {
			t.Errorf("mutant %s Status() = %v, want %v", k, m.Status(), w.status)
		}
		if m.EquivalentReason() != w.reason {
			t.Errorf("mutant %s EquivalentReason() = %q, want %q", k, m.EquivalentReason(), w.reason)
		}
	}
}

func TestFeatureEquivalentAnnotationSelectsOneMutant(t *testing.T) {
	sys := fstest.MapFS{fileA: {Data: []byte(annotatedSrc)}, "b.go": {Data: []byte(plainSrc)}}
	codeData := engine.CodeData{Cov: wholeFileProfile(fileA, "b.go")}

	res, err := runEquivalentEngine(t, sys, codeData, map[string]any{})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	checkMutants(t, mutantsByKey(t, res.Mutants), map[string]wantMutant{
		// The annotated mutants.
		boundaryAt4Col11:                  {status: mutator.Equivalent, reason: boundaryReason},
		"a.go:9:11 CONDITIONALS_NEGATION": {status: mutator.Equivalent, reason: negationReason},
		// The other type of the same operator.
		negationAt4Col11:                  {status: mutator.Runnable},
		"a.go:9:11 CONDITIONALS_BOUNDARY": {status: mutator.Runnable},
		// The other operators of the same line.
		"a.go:4:15 INVERT_LOGICAL":        {status: mutator.Runnable},
		"a.go:4:20 CONDITIONALS_BOUNDARY": {status: mutator.Runnable},
		"a.go:4:20 CONDITIONALS_NEGATION": {status: mutator.Runnable},
		// The same operators in another file.
		"b.go:4:11 CONDITIONALS_BOUNDARY": {status: mutator.Runnable},
		"b.go:4:11 CONDITIONALS_NEGATION": {status: mutator.Runnable},
	})
}

// TestRegressionEquivalentAnnotationIgnoresTypeConstraintOperators checks
// that an annotation without a column selects the one bitwise | of its line.
// The | of the type union on the same line is not a mutant.
func TestRegressionEquivalentAnnotationIgnoresTypeConstraintOperators(t *testing.T) {
	src := "package main\n\n" +
		"func Or[T ~int | ~uint](a, b T) T { return a | b } //gremlins:equivalent INVERT_BITWISE no effect\n"
	sys := fstest.MapFS{fileA: {Data: []byte(src)}}
	settings := map[string]any{configuration.MutantTypeEnabledKey(mutator.InvertBitwise): true}

	res, err := runEquivalentEngine(t, sys, engine.CodeData{Cov: wholeFileProfile(fileA)}, settings)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	got := mutantsByKey(t, res.Mutants)
	want := map[string]wantMutant{"a.go:3:46 INVERT_BITWISE": {status: mutator.Equivalent, reason: "no effect"}}
	checkMutants(t, got, want)
	if len(got) != len(want) {
		t.Errorf("Run() returned %d mutants, want %d", len(got), len(want))
	}
}

func TestFeatureEquivalentAnnotationMarksUncoveredMutant(t *testing.T) {
	sys := fstest.MapFS{fileA: {Data: []byte(annotatedSrc)}}

	res, err := runEquivalentEngine(t, sys, engine.CodeData{}, map[string]any{})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	checkMutants(t, mutantsByKey(t, res.Mutants), map[string]wantMutant{
		boundaryAt4Col11: {status: mutator.Equivalent, reason: boundaryReason},
		negationAt4Col11: {status: mutator.NotCovered},
	})
}

func TestFeatureEquivalentAnnotationKeepsSkippedStatus(t *testing.T) {
	sys := fstest.MapFS{fileA: {Data: []byte(annotatedSrc)}}
	codeData := engine.CodeData{Cov: wholeFileProfile(fileA), Diff: diff.Diff{fileA: nil}}

	res, err := runEquivalentEngine(t, sys, codeData, map[string]any{})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	checkMutants(t, mutantsByKey(t, res.Mutants), map[string]wantMutant{
		boundaryAt4Col11: {status: mutator.Skipped, reason: boundaryReason},
		negationAt4Col11: {status: mutator.Skipped},
	})
}

func TestFeatureEquivalentAnnotationOfDisabledTypeIsValid(t *testing.T) {
	sys := fstest.MapFS{fileA: {Data: []byte(annotatedSrc)}}
	codeData := engine.CodeData{Cov: wholeFileProfile(fileA)}

	res, err := runEquivalentEngine(t, sys, codeData, map[string]any{
		configuration.MutantTypeEnabledKey(mutator.ConditionalsNegation): false,
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	got := mutantsByKey(t, res.Mutants)
	if m, ok := got["a.go:9:11 CONDITIONALS_NEGATION"]; ok {
		t.Errorf("Run() returned disabled mutant %s with status %v, want none", mutantKey(m), m.Status())
	}
	checkMutants(t, got, map[string]wantMutant{
		boundaryAt4Col11:                  {status: mutator.Equivalent, reason: boundaryReason},
		"a.go:9:11 CONDITIONALS_BOUNDARY": {status: mutator.Runnable},
	})
}

// TestFeatureEquivalentAnnotationSelectsNegation checks that a column
// selects the INVERT_LOGICAL mutant of a ! on a line that also has a &&.
func TestFeatureEquivalentAnnotationSelectsNegation(t *testing.T) {
	const src = "package main\n\nfunc f(a, b bool) bool {\n" +
		"\treturn !a && b //gremlins:equivalent INVERT_LOGICAL@9 a is always true here\n}\n"
	sys := fstest.MapFS{fileA: {Data: []byte(src)}}
	codeData := engine.CodeData{Cov: wholeFileProfile(fileA)}

	res, err := runEquivalentEngine(t, sys, codeData, map[string]any{})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	checkMutants(t, mutantsByKey(t, res.Mutants), map[string]wantMutant{
		"a.go:4:9 INVERT_LOGICAL":  {status: mutator.Equivalent, reason: "a is always true here"},
		"a.go:4:12 INVERT_LOGICAL": {status: mutator.Runnable},
	})
}

// lineFourSrc returns a file whose line 4 has a < at column 11, a && at
// column 15, and a < at column 20, followed by the comment.
func lineFourSrc(comment string) string {
	return "package main\n\nfunc f(a, b, c, d int) bool {\n\treturn a < b && c < d " + comment + "\n}\n"
}

func TestFeatureEquivalentAnnotationErrors(t *testing.T) {
	testCases := []struct {
		name     string
		src      string
		wantLine int
	}{
		{name: "no arguments", src: lineFourSrc("//gremlins:equivalent"), wantLine: 4},
		{name: "missing reason", src: lineFourSrc("//gremlins:equivalent CONDITIONALS_BOUNDARY@11"), wantLine: 4},
		{name: "blank reason", src: lineFourSrc("//gremlins:equivalent CONDITIONALS_BOUNDARY@11 \t "), wantLine: 4},
		{name: "unknown type", src: lineFourSrc("//gremlins:equivalent BOUNDARY@11 r"), wantLine: 4},
		{name: "lowercase type", src: lineFourSrc("//gremlins:equivalent conditionals_boundary@11 r"), wantLine: 4},
		{name: "empty column", src: lineFourSrc("//gremlins:equivalent CONDITIONALS_BOUNDARY@ r"), wantLine: 4},
		{name: "text column", src: lineFourSrc("//gremlins:equivalent CONDITIONALS_BOUNDARY@x r"), wantLine: 4},
		{name: "zero column", src: lineFourSrc("//gremlins:equivalent CONDITIONALS_BOUNDARY@0 r"), wantLine: 4},
		{name: "signed column", src: lineFourSrc("//gremlins:equivalent CONDITIONALS_BOUNDARY@+11 r"), wantLine: 4},
		{name: "stale column", src: lineFourSrc("//gremlins:equivalent CONDITIONALS_BOUNDARY@12 r"), wantLine: 4},
		{name: "type not on line", src: lineFourSrc("//gremlins:equivalent INVERT_LOOPCTRL r"), wantLine: 4},
		{name: "ambiguous without column", src: lineFourSrc("//gremlins:equivalent CONDITIONALS_BOUNDARY r"), wantLine: 4},
		{
			name: "ambiguous logical without column",
			src: "package main\n\nfunc f(a, b bool) bool {\n" +
				"\treturn !a && b //gremlins:equivalent INVERT_LOGICAL r\n}\n",
			wantLine: 4,
		},
		{
			name: "duplicate target",
			src: "package main\n\nfunc f(a, b int) bool {\n\t//gremlins:equivalent CONDITIONALS_BOUNDARY r\n" +
				"\treturn a < b //gremlins:equivalent CONDITIONALS_BOUNDARY@11 r\n}\n",
			wantLine: 5,
		},
		{
			name: "next line has no target",
			src: "package main\n\nfunc f(a, b int) bool {\n\t//gremlins:equivalent CONDITIONALS_BOUNDARY r\n" +
				"\t_ = a\n\treturn a < b\n}\n",
			wantLine: 4,
		},
		{
			name: "line directive moves the target",
			src: "package main\n\nfunc f(a, b int) bool {\n//line other.go:40\n" +
				"\treturn a < b //gremlins:equivalent CONDITIONALS_BOUNDARY r\n}\n",
			wantLine: 5,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sys := fstest.MapFS{fileBad: {Data: []byte(tc.src)}}

			_, err := runEquivalentEngine(t, sys, engine.CodeData{}, map[string]any{})

			if !errors.Is(err, engine.ErrEquivalentAnnotation) {
				t.Fatalf("Run() error = %v, want %v", err, engine.ErrEquivalentAnnotation)
			}
			if want := fmt.Sprintf("\"bad.go:%d:", tc.wantLine); !strings.Contains(err.Error(), want) {
				t.Errorf("Run() error = %q, want it to name %q", err, want)
			}
		})
	}
}

// TestFeatureEquivalentAnnotationErrorSendsNoMutantOfTheFile checks that the
// engine binds the annotations of a file before it sends its mutants, so an
// invalid annotation never lets a mutant of that file reach the results.
func TestFeatureEquivalentAnnotationErrorSendsNoMutantOfTheFile(t *testing.T) {
	sys := fstest.MapFS{fileBad: {Data: []byte(lineFourSrc("//gremlins:equivalent CONDITIONALS_BOUNDARY@12 r"))}}

	res, err := runEquivalentEngine(t, sys, engine.CodeData{}, map[string]any{})

	if !errors.Is(err, engine.ErrEquivalentAnnotation) {
		t.Fatalf("Run() error = %v, want %v", err, engine.ErrEquivalentAnnotation)
	}
	if len(res.Mutants) != 0 {
		t.Errorf("Run() returned %d mutants of the invalid file, want 0", len(res.Mutants))
	}
}

// TestFeatureEquivalentReasonReachesTheJSONReport checks that the reason of
// an annotation goes from discovery through the engine results to the JSON
// report.
func TestFeatureEquivalentReasonReachesTheJSONReport(t *testing.T) {
	sys := fstest.MapFS{fileA: {Data: []byte(annotatedSrc)}}
	output := filepath.Join(t.TempDir(), "report.json")
	res, err := runEquivalentEngine(t, sys, engine.CodeData{Cov: wholeFileProfile(fileA)}, map[string]any{})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	log.Init(&bytes.Buffer{}, &bytes.Buffer{})
	t.Cleanup(log.Reset)

	viperSet(map[string]any{configuration.UnleashDryRunKey: true, configuration.UnleashOutputKey: output})
	err = report.Do(res)
	viperReset()
	if err != nil {
		t.Fatalf("report.Do() error = %v, want nil", err)
	}

	//nolint:gosec // test code reading its own report file
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("os.ReadFile(report) error = %v, want nil", err)
	}
	var got struct {
		Files []struct {
			Mutations []struct {
				Type             string `json:"type"`
				Status           string `json:"status"`
				EquivalentReason string `json:"equivalent_reason"`
				Line             int    `json:"line"`
				Column           int    `json:"column"`
			} `json:"mutations"`
		} `json:"files"`
		MutantsEquivalent int `json:"mutants_equivalent"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal(report) error = %v, want nil", err)
	}
	reasons := map[string]string{}
	for _, f := range got.Files {
		for _, m := range f.Mutations {
			if m.EquivalentReason != "" {
				reasons[fmt.Sprintf("%d:%d %s %s", m.Line, m.Column, m.Type, m.Status)] = m.EquivalentReason
			}
		}
	}
	want := map[string]string{
		"4:11 CONDITIONALS_BOUNDARY EQUIVALENT": boundaryReason,
		"9:11 CONDITIONALS_NEGATION EQUIVALENT": negationReason,
	}
	if fmt.Sprint(reasons) != fmt.Sprint(want) {
		t.Errorf("report reasons = %v, want %v", reasons, want)
	}
	if got.MutantsEquivalent != len(want) {
		t.Errorf("report mutants_equivalent = %d, want %d", got.MutantsEquivalent, len(want))
	}
}
