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

package typeinfo_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/OpenRAE/go-rae-gremlins/internal/typeinfo"
)

// operandsSource has one operator on each line that the comment names. The
// comment gives the expected fact, or "none".
const operandsSource = `package m

func f[T ~int | ~string, N ~int](s, t string, a, b int, u uint, x, y T, n, k N) {
	_ = s + t // InvalidSubtraction
	_ = "a" + "b" // InvalidSubtraction
	_ = a + b // none
	_ = x + y // InvalidSubtraction
	_ = n + k // none
	s += t // InvalidSubtraction
	a += b // none
	a <<= u // UnassignableShiftCount
	a <<= b // none
	a >>= 2 // none
	a >>= u // UnassignableShiftCount
}

type numeric interface {
	~int | ~string
	~int
}

func g[T interface{ ~int | ~string; ~int }, U numeric](x, y T, u, v U, a int64) {
	_ = x + y // none
	_ = u + v // none
	x += y // none
	const n uint = 2
	a <<= n // UnassignableShiftCount
	a >>= n // UnassignableShiftCount
	const m = 2
	a <<= m // none
}

const (
	first = iota + 1 // none
)
`

// wantFacts returns the facts that the comments of src name, by the byte
// offset of the operator, which is the first operator token of each line.
func wantFacts(t *testing.T, src string) map[int]typeinfo.Fact {
	t.Helper()
	facts := map[string]typeinfo.Fact{
		"InvalidSubtraction":     typeinfo.InvalidSubtraction,
		"UnassignableShiftCount": typeinfo.UnassignableShiftCount,
	}
	want := make(map[int]typeinfo.Fact)
	offset := 0
	for line := range strings.SplitAfterSeq(src, "\n") {
		code, comment, ok := strings.Cut(line, "// ")
		if ok && strings.HasPrefix(line, "\t") {
			name := strings.TrimSpace(comment)
			if f, known := facts[name]; known {
				want[offset+operatorIndex(t, code)] = f
			} else if name != "none" {
				t.Fatalf("unknown fact %q in the source", name)
			}
		}
		offset += len(line)
	}

	return want
}

// operatorIndex returns the index of the operator in a line of
// operandsSource: the token after the first operand.
func operatorIndex(t *testing.T, code string) int {
	t.Helper()
	for _, op := range []string{" <<= ", " >>= ", " += ", " + "} {
		if i := strings.Index(code, op); i >= 0 {
			return i + 1
		}
	}
	t.Fatalf("no operator in %q", code)

	return 0
}

// newModule writes a module with the files to a new directory and returns
// the directory.
func newModule(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files["go.mod"] = "module example.com/m\n\ngo 1.25\n"
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	return root
}

// buildCache returns the Go build cache of the test environment, so the
// load reuses compiled standard packages.
func buildCache(t *testing.T) string {
	t.Helper()
	out, err := exec.CommandContext(t.Context(), "go", "env", "GOCACHE").Output()
	if err != nil {
		t.Fatalf("go env GOCACHE: %v", err)
	}

	return strings.TrimSpace(string(out))
}

// TestFeatureLoadFindsOperandFacts checks the facts of each operator kind
// that the specs need, by module-relative file name and byte offset.
func TestFeatureLoadFindsOperandFacts(t *testing.T) {
	root := newModule(t, map[string]string{"sub/ops.go": operandsSource})

	got, err := typeinfo.Load(t.Context(), root, "./...", "", buildCache(t))
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	if diff := cmp.Diff(wantFacts(t, operandsSource), got.File("sub/ops.go")); diff != "" {
		t.Errorf("Load() facts of sub/ops.go mismatch (-want +got):\n%s", diff)
	}
	if f := got.File("ops.go"); f != nil {
		t.Errorf("Load() facts of ops.go = %v, want none", f)
	}
}

// TestFeatureLoadAppliesBuildTags checks that the load reads only the files
// of the host build with the given tags.
func TestFeatureLoadAppliesBuildTags(t *testing.T) {
	tagged := "//go:build mytag\n\npackage m\n\nfunc g(s, t string) string {\n\treturn s + t\n}\n"
	root := newModule(t, map[string]string{"tagged.go": tagged})

	testCases := map[string]struct {
		tags string
		want bool
	}{
		"without the tag": {tags: "", want: false},
		"with the tag":    {tags: "mytag", want: true},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := typeinfo.Load(t.Context(), root, "./...", tc.tags, buildCache(t))
			if err != nil {
				t.Fatalf("Load() error = %v, want nil", err)
			}
			if has := got.File("tagged.go") != nil; has != tc.want {
				t.Errorf("Load(tags %q) has facts of tagged.go = %t, want %t", tc.tags, has, tc.want)
			}
		})
	}
}

// TestFeatureLoadGivesNoFactsForAPackageWithTypeErrors checks that a package
// that does not type-check gives no facts and no error, so the specs keep
// their earlier selection.
func TestFeatureLoadGivesNoFactsForAPackageWithTypeErrors(t *testing.T) {
	broken := "package bad\n\nfunc g(s, t string) string {\n\treturn s + t + undefined\n}\n"
	good := "package good\n\nfunc g(s, t string) string {\n\treturn s + t\n}\n"
	root := newModule(t, map[string]string{"bad/bad.go": broken, "good/good.go": good})

	got, err := typeinfo.Load(t.Context(), root, "./...", "", buildCache(t))
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	if f := got.File("bad/bad.go"); f != nil {
		t.Errorf("Load() facts of bad/bad.go = %v, want none", f)
	}
	if f := got.File("good/good.go"); len(f) != 1 {
		t.Errorf("Load() facts of good/good.go = %v, want one fact", f)
	}
}

// TestFeatureLoadReportsALoadFailure checks that Load returns an error when
// the go command cannot list the packages.
func TestFeatureLoadReportsALoadFailure(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")

	_, err := typeinfo.Load(t.Context(), missing, "./...", "", buildCache(t))

	if err == nil {
		t.Fatal("Load() error = nil, want an error for a directory that does not exist")
	}
}

// TestFeatureLoadStopsWhenTheContextIsDone checks that a cancelled context
// stops the load with an error.
func TestFeatureLoadStopsWhenTheContextIsDone(t *testing.T) {
	root := newModule(t, map[string]string{"ops.go": operandsSource})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := typeinfo.Load(ctx, root, "./...", "", buildCache(t))

	if err == nil {
		t.Fatal("Load() error = nil, want an error for a cancelled context")
	}
}

func TestFeatureFactHas(t *testing.T) {
	f := typeinfo.InvalidSubtraction

	if !f.Has(typeinfo.InvalidSubtraction) {
		t.Error("InvalidSubtraction.Has(InvalidSubtraction) = false, want true")
	}
	if f.Has(typeinfo.UnassignableShiftCount) {
		t.Error("InvalidSubtraction.Has(UnassignableShiftCount) = true, want false")
	}
}
