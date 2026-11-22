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

package cmd

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/OpenRAE/go-rae-gremlins/internal/configuration"
	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
)

const (
	diffPkgSource  = "package pkg\n\nfunc Less(a, b int) bool { return a < b }\n\nfunc More(a, b int) bool { return a > b }\n"
	diffPkgChanged = "package pkg\n\nfunc Less(a, b int) bool { return a <= b }\n\nfunc More(a, b int) bool { return a > b }\n"
)

// diffReport is the part of the JSON report that the diff selection test reads.
type diffReport struct {
	Files []struct {
		Filename  string `json:"file_name"`
		Mutations []struct {
			Status string `json:"status"`
			Line   int    `json:"line"`
		} `json:"mutations"`
	} `json:"files"`
}

// TestRegressionUnleashDiffSelectsNestedModule runs unleash --diff from the
// root of a Git repository on a module in a subdirectory. The mutant on the
// changed line must be runnable, and the mutant on the unchanged line must
// stay skipped.
func TestRegressionUnleashDiffSelectsNestedModule(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the go and git commands on a temporary repository")
	}
	workspace := newUnleashWorkspace(t)
	pkgFile := filepath.Join(workspace, "mod", "pkg", "pkg.go")
	writeTestFile(t, pkgFile, diffPkgSource)
	commitWorkspace(t, workspace)
	writeTestFile(t, pkgFile, diffPkgChanged)
	t.Chdir(workspace)

	configuration.Reset()
	t.Cleanup(configuration.Reset)
	if err := configuration.Init([]string{t.TempDir()}); err != nil {
		t.Fatalf("configuration.Init error = %v, want nil", err)
	}
	outDir := t.TempDir()
	configuration.Set(configuration.UnleashDryRunKey, true)
	configuration.Set(configuration.MutantTypeEnabledKey(mutator.ConditionalsBoundary), true)
	configuration.Set(configuration.UnleashDiffRef, "HEAD")
	configuration.Set(configuration.UnleashOutputKey, filepath.Join(outDir, "report.json"))

	if err := runUnleash(context.Background())(nil, []string{"mod"}); err != nil {
		t.Fatalf("runUnleash(mod) error = %v, want nil", err)
	}

	got := mutantStatuses(t, outDir, "pkg/pkg.go")
	want := map[int]string{3: "RUNNABLE", 5: "SKIPPED"}
	for line, status := range want {
		if got[line] != status {
			t.Errorf("pkg/pkg.go:%d status = %q, want %q (all: %v)", line, got[line], status, got)
		}
	}
}

// mutantStatuses reads the JSON report file report.json in dir and returns
// the status of each mutant of fileName, by line.
func mutantStatuses(t *testing.T, dir, fileName string) map[int]string {
	t.Helper()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatalf("open report directory: %v", err)
	}
	b, err := root.ReadFile("report.json")
	if closeErr := root.Close(); closeErr != nil {
		t.Fatalf("close report directory: %v", closeErr)
	}
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var r diffReport
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	statuses := map[int]string{}
	for _, f := range r.Files {
		if f.Filename != fileName {
			continue
		}
		for _, m := range f.Mutations {
			statuses[m.Line] = m.Status
		}
	}

	return statuses
}

// commitWorkspace creates a Git repository in dir and commits every file. The
// repository uses no global or system Git configuration.
func commitWorkspace(t *testing.T, dir string) {
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
	for _, cmd := range []*exec.Cmd{
		exec.CommandContext(t.Context(), "git", "init", "--quiet"),
		exec.CommandContext(t.Context(), "git", "add", "."),
		exec.CommandContext(t.Context(), "git", "commit", "--quiet", "-m", "initial"),
	} {
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", cmd.Args, err, out)
		}
	}
}

func writeTestFile(t *testing.T, name, content string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
		t.Fatalf("os.WriteFile error = %v", err)
	}
}
