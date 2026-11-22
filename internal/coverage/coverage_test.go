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

package coverage_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/viper"

	"github.com/OpenRAE/go-rae-gremlins/internal/coverage"

	"github.com/OpenRAE/go-rae-gremlins/internal/configuration"
	"github.com/OpenRAE/go-rae-gremlins/internal/gomodule"
)

const (
	allPackages    = "./..."
	exampleModule  = "example.com"
	testBuildCache = "/gremlins-test/gocache"
)

type commandHolder struct {
	events []struct {
		cmd     *exec.Cmd
		command string
		args    []string
	}
}

func TestCoverageRun(t *testing.T) {
	testCases := []struct {
		name     string
		callPath string
		wantPath string
		intMode  bool
	}{
		{
			name:     "from root, normal mode",
			callPath: ".",
			wantPath: allPackages,
			intMode:  false,
		},
		{
			name:     "from folder, normal mode",
			callPath: "test/pkg",
			wantPath: "./test/pkg/...",
			intMode:  false,
		},
		{
			name:     "from root, integration mode",
			callPath: ".",
			wantPath: allPackages,
			intMode:  true,
		},
		{
			name:     "from folder, integration mode",
			callPath: "test/dir",
			wantPath: allPackages,
			intMode:  true,
		},
	}
	coverpkg := "./internal/log,./pkg/..."
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			viper.Set(configuration.UnleashTagsKey, "tag1 tag2")
			viper.Set(configuration.UnleashCoverPkgKey, coverpkg)
			viper.Set(configuration.UnleashIntegrationMode, tc.intMode)
			defer viper.Reset()

			wantWorkdir := filepath.Join("testdata", "valid")
			wantFilePath, err := filepath.Abs(filepath.Join(wantWorkdir, "coverage"))
			if err != nil {
				t.Fatal(err)
			}
			holder := &commandHolder{}
			mod := gomodule.GoModule{
				Name:       exampleModule,
				Root:       t.TempDir(),
				CallingDir: tc.callPath,
			}
			callerDir, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			cov := coverage.NewWithCmd(fakeExecCommandSuccess(t, holder), wantWorkdir, testBuildCache, mod)

			if _, err := cov.Run(t.Context()); err != nil {
				t.Fatalf("Run error = %v, want nil", err)
			}

			assertCallerDirKept(t, callerDir, holder, mod.Root)

			want := []string{
				"go mod download",
				"go list -buildvcs=false -export -deps -test -tags tag1 tag2 " + tc.wantPath,
				fmt.Sprintf("go test -tags tag1 tag2 -coverpkg %s -count=1 -cover -coverprofile %v %s",
					coverpkg, wantFilePath, tc.wantPath),
			}
			var got []string
			for _, e := range holder.events {
				got = append(got, fmt.Sprintf("go %v", strings.Join(e.args, " ")))
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("Run() commands mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFeatureCoverageRunUsesTemporaryBuildCache(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "user-cache"))
	workDir := t.TempDir()
	holder := &commandHolder{}
	mod := gomodule.GoModule{Name: exampleModule, Root: t.TempDir(), CallingDir: "."}
	if err := os.WriteFile(filepath.Join(workDir, "coverage"), []byte("mode: set\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cov := coverage.NewWithCmd(fakeExecCommandSuccess(t, holder), workDir, testBuildCache, mod)

	if _, err := cov.Run(t.Context()); err != nil {
		t.Fatalf("Run error = %v, want nil", err)
	}

	if len(holder.events) != 3 {
		t.Fatalf("Run() executed %d commands, want 3", len(holder.events))
	}
	for _, e := range holder.events {
		if got := lastValue(e.cmd.Env, "GOCACHE"); got != testBuildCache {
			t.Errorf("%s %v GOCACHE = %q, want %q", e.command, e.args, got, testBuildCache)
		}
		if got := lastValue(e.cmd.Env, "GO_TEST_PROCESS"); got != "1" {
			t.Errorf("%s %v GO_TEST_PROCESS = %q, want the command's own value %q", e.command, e.args, got, "1")
		}
	}
}

// lastValue returns the value of the last entry of key in env, because
// exec.Cmd uses the last value of a duplicate variable.
func lastValue(env []string, key string) string {
	var value string
	for _, v := range env {
		if k, val, ok := strings.Cut(v, "="); ok && k == key {
			value = val
		}
	}

	return value
}

// assertCallerDirKept checks that the working directory is still callerDir
// and that each recorded command ran in root.
func assertCallerDirKept(t *testing.T, callerDir string, holder *commandHolder, root string) {
	t.Helper()
	if got, err := os.Getwd(); err != nil || got != callerDir {
		t.Errorf("working directory after Run = %q, %v; want %q, nil", got, err, callerDir)
	}
	for _, e := range holder.events {
		if e.cmd.Dir != root {
			t.Errorf("%s %v ran in %q, want module root %q", e.command, e.args, e.cmd.Dir, root)
		}
	}
}

func TestCoverageRunFails(t *testing.T) {
	mod := gomodule.GoModule{
		Name:       exampleModule,
		CallingDir: allPackages,
	}

	t.Run("failure of: go mod download", func(t *testing.T) {
		cov := coverage.NewWithCmd(fakeExecCommandFailure(t, 0), "workdir", testBuildCache, mod)
		if _, err := cov.Run(t.Context()); err == nil {
			t.Error("Run() error = nil, want an error")
		}
	})

	t.Run("failure of: go list", func(t *testing.T) {
		cov := coverage.NewWithCmd(fakeExecCommandFailure(t, 1), "workdir", testBuildCache, mod)
		if _, err := cov.Run(t.Context()); err == nil {
			t.Error("Run() error = nil, want an error")
		}
	})

	t.Run("failure of: go test", func(t *testing.T) {
		cov := coverage.NewWithCmd(fakeExecCommandFailure(t, 2), "workdir", testBuildCache, mod)
		if _, err := cov.Run(t.Context()); err == nil {
			t.Error("Run() error = nil, want an error")
		}
	})
}

func TestFeatureCoverageRunStopsOnCancel(t *testing.T) {
	mod := gomodule.GoModule{
		Name:       exampleModule,
		Root:       ".",
		CallingDir: ".",
	}
	holder := &commandHolder{}
	cov := coverage.NewWithCmd(fakeExecCommandSuccess(t, holder), "workdir", testBuildCache, mod)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := cov.Run(ctx)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Run(cancelled context) error = %v, want %v", err, context.Canceled)
	}
	if len(holder.events) != 1 {
		t.Errorf("Run(cancelled context) built %d commands, want 1", len(holder.events))
	}
}

func TestCoverageParsesOutput(t *testing.T) {
	module := exampleModule
	mod := gomodule.GoModule{
		Name:       module,
		CallingDir: "path",
	}
	cov := coverage.NewWithCmd(fakeExecCommandSuccess(t, nil), "testdata/valid", testBuildCache, mod)
	profile := coverage.Profile{
		"file1.go": {
			{
				StartLine: 47,
				StartCol:  2,
				EndLine:   48,
				EndCol:    16,
			},
		},
		"file2.go": {
			{
				StartLine: 52,
				StartCol:  2,
				EndLine:   53,
				EndCol:    16,
			},
		},
		// A nested file keeps a slash-separated name on each platform.
		"sub/file3.go": {
			{
				StartLine: 10,
				StartCol:  2,
				EndLine:   11,
				EndCol:    16,
			},
		},
		// A directory with the name of the module stays in the path.
		"example.com/file4.go": {
			{
				StartLine: 20,
				StartCol:  2,
				EndLine:   21,
				EndCol:    16,
			},
		},
	}
	want := coverage.Result{
		Profile: profile,
	}

	got, err := cov.Run(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(want.Profile, got.Profile); diff != "" {
		t.Errorf("Run() profile mismatch (-want +got):\n%s", diff)
	}
	if got.Elapsed == 0 {
		t.Error("Run() Elapsed = 0, want a positive duration")
	}
	if got.Peak == 0 {
		t.Error("Run().Peak = 0, want the peak memory of the coverage process")
	}
}

func TestRegressionCoverageMatchesModuleAfterComment(t *testing.T) {
	root := t.TempDir()
	goMod := filepath.Join(root, "go.mod")
	if err := os.WriteFile(goMod, []byte("// license\nmodule example.com/m\n\ngo 1.25.0\n"), 0o600); err != nil {
		t.Fatalf("write %s: %v", goMod, err)
	}
	workdir := t.TempDir()
	profilePath := filepath.Join(workdir, "coverage")
	if err := os.WriteFile(profilePath, []byte("mode: set\nexample.com/m/m.go:3.2,5.3 1 1\n"), 0o600); err != nil {
		t.Fatalf("write %s: %v", profilePath, err)
	}
	mod, err := gomodule.Init(root)
	if err != nil {
		t.Fatalf("gomodule.Init(%q) error = %v, want nil", root, err)
	}
	cov := coverage.NewWithCmd(fakeExecCommandSuccess(t, nil), workdir, testBuildCache, mod)

	got, err := cov.Run(t.Context())
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	want := coverage.Profile{"m.go": {{StartLine: 3, StartCol: 2, EndLine: 5, EndCol: 3}}}
	if diff := cmp.Diff(want, got.Profile); diff != "" {
		t.Errorf("Run() profile mismatch (-want +got):\n%s", diff)
	}
}

func TestParseOutputFail(t *testing.T) {
	mod := gomodule.GoModule{
		Name:       exampleModule,
		CallingDir: allPackages,
	}
	cov := coverage.NewWithCmd(fakeExecCommandSuccess(t, nil), "testdata/invalid", testBuildCache, mod)

	if _, err := cov.Run(t.Context()); err == nil {
		t.Error("Run() with an invalid coverage profile error = nil, want an error")
	}
}

func TestCoverageProcessSuccess(_ *testing.T) {
	if os.Getenv("GO_TEST_PROCESS") != "1" {
		return
	}
	os.Exit(0) // skipcq: RVV-A0003
}

func TestCoverageProcessFailure(_ *testing.T) {
	if os.Getenv("GO_TEST_PROCESS") != "1" {
		return
	}
	os.Exit(1) // skipcq: RVV-A0003
}

type execContext = func(ctx context.Context, name string, args ...string) *exec.Cmd

func fakeExecCommandSuccess(t *testing.T, got *commandHolder) execContext {
	t.Helper()

	return func(ctx context.Context, command string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestCoverageProcessSuccess", "--", command}
		cs = append(cs, args...)
		// #nosec G204 G702 - We are in tests, we don't care
		cmd := exec.CommandContext(ctx, os.Args[0], cs...)
		cmd.Env = []string{"GO_TEST_PROCESS=1"}
		if got != nil {
			got.events = append(got.events, struct {
				cmd     *exec.Cmd
				command string
				args    []string
			}{cmd: cmd, command: command, args: args})
		}

		return cmd
	}
}

func fakeExecCommandFailure(t *testing.T, run int) execContext {
	t.Helper()
	var executed int

	return func(ctx context.Context, command string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestCoverageProcessSuccess", "--", command}
		if executed == run {
			cs = []string{"-test.run=TestCoverageProcessFailure", "--", command}
		}
		cs = append(cs, args...)
		// #nosec G204 G702 - We are in tests, we don't care
		cmd := exec.CommandContext(ctx, os.Args[0], cs...)
		cmd.Env = []string{"GO_TEST_PROCESS=1"}
		executed++

		return cmd
	}
}
