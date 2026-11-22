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
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/OpenRAE/go-rae-gremlins/internal/configuration"
	"github.com/OpenRAE/go-rae-gremlins/internal/engine"
	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
)

const plainFileB = "package main\n\nfunc g(a, b int) int {\n\treturn a + b\n}\n"

// ignoreSettings enables only the mutation types of the ignore tests, so the
// results do not depend on the defaults or on the order of the tests.
func ignoreSettings(disabled ...mutator.Type) map[string]any {
	enabled := []mutator.Type{mutator.ArithmeticBase, mutator.ConditionalsBoundary, mutator.ConditionalsNegation}
	settings := make(map[string]any, len(mutator.Types))
	for _, mt := range mutator.Types {
		settings[configuration.MutantTypeEnabledKey(mt)] = slices.Contains(enabled, mt) && !slices.Contains(disabled, mt)
	}

	return settings
}

// mutantKeys runs the engine on the files with full coverage and returns
// the sorted keys of the mutants that it creates.
func mutantKeys(t *testing.T, files map[string]string, settings map[string]any) []string {
	t.Helper()
	sys := fstest.MapFS{}
	names := make([]string, 0, len(files))
	for name, src := range files {
		sys[name] = &fstest.MapFile{Data: []byte(src)}
		names = append(names, name)
	}
	codeData := engine.CodeData{Cov: wholeFileProfile(names...)}

	res, err := runEquivalentEngine(t, sys, codeData, settings)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	keys := make([]string, 0, len(res.Mutants))
	for _, m := range res.Mutants {
		keys = append(keys, mutantKey(m))
	}
	slices.Sort(keys)

	return keys
}

func TestFeatureIgnoreDirectiveScopes(t *testing.T) {
	testCases := []struct {
		files map[string]string
		name  string
		want  []string
	}{
		{
			name: "after code it ignores all the types of its line",
			files: map[string]string{fileA: "package main\n\nfunc f(a, b int) int {\n" +
				"\tx := a + b //gremlins:ignore\n" +
				"\ty := a + b\n" +
				"\treturn x + y\n}\n"},
			want: []string{"a.go:5:9 ARITHMETIC_BASE", "a.go:6:11 ARITHMETIC_BASE"},
		},
		{
			name: "after code it ignores only the listed types of its line",
			files: map[string]string{fileA: "package main\n\nfunc f(a, b int) bool {\n" +
				"\tx := a + b //gremlins:ignore CONDITIONALS_BOUNDARY, ARITHMETIC_BASE\n" +
				"\treturn a < b //gremlins:ignore CONDITIONALS_BOUNDARY\n}\n"},
			want: []string{"a.go:5:11 CONDITIONALS_NEGATION"},
		},
		{
			name: "before a function it ignores the function",
			files: map[string]string{fileA: "package main\n\n" +
				"//gremlins:ignore\n" +
				"func f(a, b int) int {\n\treturn a + b\n}\n\n" +
				"func g(a, b int) int {\n\treturn a + b\n}\n"},
			want: []string{"a.go:9:11 ARITHMETIC_BASE"},
		},
		{
			name: "at the end of a documentation comment it ignores the function",
			files: map[string]string{fileA: "package main\n\n" +
				"// f adds.\n//\n//gremlins:ignore\n" +
				"func f(a, b int) int {\n\treturn a + b\n}\n\n" +
				"func g(a, b int) int {\n\treturn a + b\n}\n"},
			want: []string{"a.go:11:11 ARITHMETIC_BASE"},
		},
		{
			name: "before a statement it ignores the statement and its branches",
			files: map[string]string{fileA: "package main\n\nfunc f(a, b int) bool {\n" +
				"\t//gremlins:ignore CONDITIONALS_BOUNDARY\n" +
				"\tif a < b {\n" +
				"\t\treturn a + b > 0\n" +
				"\t} else if a > b {\n" +
				"\t\treturn a * b > 0\n" +
				"\t}\n" +
				"\treturn a < b\n}\n"},
			want: []string{
				"a.go:10:11 CONDITIONALS_BOUNDARY",
				"a.go:10:11 CONDITIONALS_NEGATION",
				"a.go:5:7 CONDITIONALS_NEGATION",
				"a.go:6:12 ARITHMETIC_BASE",
				"a.go:6:16 CONDITIONALS_NEGATION",
				"a.go:7:14 CONDITIONALS_NEGATION",
				"a.go:8:12 ARITHMETIC_BASE",
				"a.go:8:16 CONDITIONALS_NEGATION",
			},
		},
		{
			name: "before a line it ignores all the statements that start on that line",
			files: map[string]string{fileA: "package main\n\nfunc f(a, b int) int {\n" +
				"\t//gremlins:ignore\n" +
				"\tx := a + b; y := a + b\n" +
				"\treturn x + y\n}\n"},
			want: []string{"a.go:6:11 ARITHMETIC_BASE"},
		},
		{
			name: "before the package clause it ignores the file",
			files: map[string]string{
				fileA: "// Header.\n\n//gremlins:ignore\npackage main\n\n" +
					"func f(a, b int) int {\n\treturn a + b\n}\n",
				"b.go": plainFileB,
			},
			want: []string{"b.go:4:11 ARITHMETIC_BASE"},
		},
		{
			name: "a string is not a directive",
			files: map[string]string{fileA: "package main\n\nfunc f(b string) string {\n" +
				"\treturn \"//gremlins:ignore\" + b\n}\n"},
			want: []string{"a.go:4:29 ARITHMETIC_BASE"},
		},
		{
			name: "a line directive does not move a scope",
			files: map[string]string{fileA: "package main\n\nfunc f(a, b int) int {\n" +
				"//line other.go:40\n" +
				"\tx := a + b //gremlins:ignore\n" +
				"\t//gremlins:ignore\n" +
				"\ty := x + b\n" +
				"\treturn y + a\n}\n"},
			want: []string{"other.go:43:0 ARITHMETIC_BASE"},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := mutantKeys(t, tc.files, ignoreSettings())

			if !slices.Equal(got, tc.want) {
				t.Errorf("Run() mutants = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFeatureIgnoreDirectiveOfDisabledTypeIsValid(t *testing.T) {
	files := map[string]string{fileA: "package main\n\nfunc f(a, b int) bool {\n" +
		"\treturn a < b //gremlins:ignore CONDITIONALS_BOUNDARY\n}\n"}

	got := mutantKeys(t, files, ignoreSettings(mutator.ConditionalsBoundary))

	if want := []string{"a.go:4:11 CONDITIONALS_NEGATION"}; !slices.Equal(got, want) {
		t.Errorf("Run() mutants = %q, want %q", got, want)
	}
}

// TestFeatureIgnoreDirectiveKeepsValidEquivalentAnnotation checks that an
// equivalent annotation in an ignored scope stays valid, and that the
// ignored mutant is not created.
func TestFeatureIgnoreDirectiveKeepsValidEquivalentAnnotation(t *testing.T) {
	files := map[string]string{fileA: "//gremlins:ignore\npackage main\n\nfunc f(a, b int) bool {\n" +
		"\treturn a < b //gremlins:equivalent CONDITIONALS_BOUNDARY the bound is unreachable\n}\n"}

	if got := mutantKeys(t, files, ignoreSettings()); len(got) != 0 {
		t.Errorf("Run() mutants = %q, want none", got)
	}
}

// TestFeatureIgnoreDirectiveDoesNotHideInvalidEquivalentAnnotation checks
// that the engine binds equivalent annotations before it ignores mutants.
func TestFeatureIgnoreDirectiveDoesNotHideInvalidEquivalentAnnotation(t *testing.T) {
	src := "//gremlins:ignore\npackage main\n\nfunc f(a, b int) bool {\n" +
		"\treturn a < b //gremlins:equivalent CONDITIONALS_BOUNDARY@12 r\n}\n"
	sys := fstest.MapFS{fileBad: {Data: []byte(src)}}

	_, err := runEquivalentEngine(t, sys, engine.CodeData{}, ignoreSettings())

	if !errors.Is(err, engine.ErrEquivalentAnnotation) {
		t.Fatalf("Run() error = %v, want %v", err, engine.ErrEquivalentAnnotation)
	}
}

func TestFeatureIgnoreDirectiveErrors(t *testing.T) {
	const head = "package main\n\nfunc f(a, b int) int {\n"
	testCases := []struct {
		name     string
		src      string
		wantLine int
	}{
		{name: "unknown type", src: head + "\treturn a + b //gremlins:ignore BOUNDARY\n}\n", wantLine: 4},
		{name: "configuration name", src: head + "\treturn a + b //gremlins:ignore arithmetic-base\n}\n", wantLine: 4},
		{name: "trailing comma", src: head + "\treturn a + b //gremlins:ignore ARITHMETIC_BASE,\n}\n", wantLine: 4},
		{
			name:     "empty entry",
			src:      head + "\treturn a + b //gremlins:ignore ARITHMETIC_BASE,,CONDITIONALS_BOUNDARY\n}\n",
			wantLine: 4,
		},
		{
			name:     "types without a comma",
			src:      head + "\treturn a + b //gremlins:ignore ARITHMETIC_BASE CONDITIONALS_BOUNDARY\n}\n",
			wantLine: 4,
		},
		{name: "blank line after a directive", src: head + "\t//gremlins:ignore\n\n\treturn a + b\n}\n", wantLine: 4},
		{name: "closing brace after a directive", src: head + "\treturn a + b\n\t//gremlins:ignore\n}\n", wantLine: 5},
		{name: "end of file after a directive", src: head + "\treturn a + b\n}\n\n//gremlins:ignore\n", wantLine: 7},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sys := fstest.MapFS{fileBad: {Data: []byte(tc.src)}}

			res, err := runEquivalentEngine(t, sys, engine.CodeData{}, ignoreSettings())

			if !errors.Is(err, engine.ErrIgnoreDirective) {
				t.Fatalf("Run() error = %v, want %v", err, engine.ErrIgnoreDirective)
			}
			if want := fmt.Sprintf("\"bad.go:%d:", tc.wantLine); !strings.Contains(err.Error(), want) {
				t.Errorf("Run() error = %q, want it to name %q", err, want)
			}
			if len(res.Mutants) != 0 {
				t.Errorf("Run() returned %d mutants of the invalid file, want 0", len(res.Mutants))
			}
		})
	}
}

func TestFeatureUnknownDirectiveIsAnError(t *testing.T) {
	src := "package main\n\nfunc f(a, b int) int {\n\treturn a + b //gremlins:ignored\n}\n"
	sys := fstest.MapFS{fileBad: {Data: []byte(src)}}

	_, err := runEquivalentEngine(t, sys, engine.CodeData{}, ignoreSettings())

	if !errors.Is(err, engine.ErrDirective) {
		t.Fatalf("Run() error = %v, want %v", err, engine.ErrDirective)
	}
	if errors.Is(err, engine.ErrEquivalentAnnotation) || errors.Is(err, engine.ErrIgnoreDirective) {
		t.Errorf("Run() error = %v, want only %v", err, engine.ErrDirective)
	}
	if want := "\"bad.go:4:"; !strings.Contains(err.Error(), want) {
		t.Errorf("Run() error = %q, want it to name %q", err, want)
	}
}
