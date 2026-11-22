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

package diff

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

const (
	nestedModuleDir = "tools/verifier"
	siblingDir      = "tools/verifier-extra"
	sourceBefore    = "package p\n\nfunc ok(a, b int) bool {\n\treturn a != b\n}\n"
	sourceAfter     = "package p\n\nfunc ok(a, b int) bool {\n\treturn a != b && a > 0\n}\n"
)

// changedFiles are the repository-relative names of the files that diffRepo
// changes. Git quotes the name with a non-ASCII letter in its patch.
var changedFiles = []string{
	"root.go",
	nestedModuleDir + "/check/check.go",
	nestedModuleDir + "/check/with space.go",
	nestedModuleDir + "/check/\u00e9.go",
	siblingDir + "/check.go",
}

// TestFeatureDiffSelectsScannedDirectory proves that the Diff of a scanned
// directory names files relative to that directory, as the engine does, and
// holds no change outside it. The repository has changes in the root, in a
// nested module, and in a sibling directory with the same name prefix. Some
// file names have a space or a non-ASCII letter. The repository configuration
// asks for colored output without path prefixes.
func TestFeatureDiffSelectsScannedDirectory(t *testing.T) {
	repo := diffRepo(t)
	setDiffRef(t, "HEAD")

	cases := []struct {
		name    string
		dir     string
		changed []string
	}{
		{name: "repository root", dir: ".", changed: changedFiles},
		{name: "nested module", dir: nestedModuleDir, changed: []string{"check/check.go", "check/with space.go", "check/\u00e9.go"}},
		{name: "package directory", dir: nestedModuleDir + "/check", changed: []string{"check.go", "with space.go", "\u00e9.go"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := New(filepath.Join(repo, filepath.FromSlash(tc.dir)))
			if err != nil {
				t.Fatalf("New() error = %v, want nil", err)
			}
			want := Diff{}
			for _, name := range tc.changed {
				want[FileName(name)] = []Change{{StartLine: 4, EndLine: 4}}
			}
			if !reflect.DeepEqual(d, want) {
				t.Errorf("New(%q) = %v, want %v", tc.dir, d, want)
			}
		})
	}
}

// TestFeatureDiffRejectsOptionReference proves that Git reads the diff
// reference only as a revision. An option-like reference is an error and
// writes no file.
func TestFeatureDiffRejectsOptionReference(t *testing.T) {
	repo := diffRepo(t)
	setDiffRef(t, "--output=written")

	d, err := New(repo)
	if err == nil {
		t.Errorf("New() = %v, nil; want an error", d)
	}
	if _, statErr := os.Stat(filepath.Join(repo, "written")); statErr == nil {
		t.Error("git wrote the file that the option-like reference names")
	}
}

// diffRepo creates a Git repository with a root Go file, a Go module in
// nestedModuleDir, and a Go file in siblingDir. It commits them and then
// changes line 4 of each file. The repository uses no global or system Git
// configuration. Its local configuration asks for colored output without path
// prefixes, which the diff command must override.
func diffRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.invalid")

	// Inherited repository variables would redirect Git away from the
	// temporary repository, for example when a Git hook runs the tests.
	for _, name := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR"} {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, filepath.FromSlash(nestedModuleDir), "go.mod"), "module example.com/verifier\n")
	for _, name := range changedFiles {
		writeFile(t, filepath.Join(repo, filepath.FromSlash(name)), sourceBefore)
	}

	runIn(t, repo, exec.CommandContext(t.Context(), "git", "init", "--quiet"))
	runIn(t, repo, exec.CommandContext(t.Context(), "git", "config", "color.diff", "always"))
	runIn(t, repo, exec.CommandContext(t.Context(), "git", "config", "diff.noprefix", "true"))
	runIn(t, repo, exec.CommandContext(t.Context(), "git", "add", "."))
	runIn(t, repo, exec.CommandContext(t.Context(), "git", "commit", "--quiet", "-m", "initial"))

	for _, name := range changedFiles {
		writeFile(t, filepath.Join(repo, filepath.FromSlash(name)), sourceAfter)
	}

	return repo
}

func writeFile(t *testing.T, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// runIn runs cmd in dir and fails the test when cmd fails.
func runIn(t *testing.T, dir string, cmd *exec.Cmd) {
	t.Helper()
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %v\n%s", cmd.Args, err, out)
	}
}
