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
	allPackages   = "./..."
	exampleModule = "example.com"
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
			cov := coverage.NewWithCmd(fakeExecCommandSuccess(t, holder), wantWorkdir, mod)

			if _, err := cov.Run(); err != nil {
				t.Fatalf("Run error = %v, want nil", err)
			}

			if got, err := os.Getwd(); err != nil || got != callerDir {
				t.Errorf("working directory after Run = %q, %v; want %q, nil", got, err, callerDir)
			}
			for _, e := range holder.events {
				if e.cmd.Dir != mod.Root {
					t.Errorf("%s %v ran in %q, want module root %q", e.command, e.args, e.cmd.Dir, mod.Root)
				}
			}

			firstWant := "go mod download"
			secondWant := fmt.Sprintf("go test -tags tag1 tag2 -coverpkg %s -cover -coverprofile %v %s",
				coverpkg, wantFilePath, tc.wantPath)

			if len(holder.events) != 2 {
				t.Fatal("expected two commands to be executed")
			}
			firstGot := fmt.Sprintf("go %v", strings.Join(holder.events[0].args, " "))
			secondGot := fmt.Sprintf("go %v", strings.Join(holder.events[1].args, " "))

			if !cmp.Equal(firstGot, firstWant) {
				t.Error(cmp.Diff(firstGot, firstWant))
			}
			if !cmp.Equal(secondGot, secondWant) {
				t.Error(cmp.Diff(secondGot, secondWant))
			}
		})
	}
}

func TestCoverageRunFails(t *testing.T) {
	mod := gomodule.GoModule{
		Name:       exampleModule,
		CallingDir: allPackages,
	}

	t.Run("failure of: go mod download", func(t *testing.T) {
		cov := coverage.NewWithCmd(fakeExecCommandFailure(t, 0), "workdir", mod)
		if _, err := cov.Run(); err == nil {
			t.Error("expected run to report an error")
		}
	})

	t.Run("failure of: go test", func(t *testing.T) {
		cov := coverage.NewWithCmd(fakeExecCommandFailure(t, 1), "workdir", mod)
		if _, err := cov.Run(); err == nil {
			t.Error("expected run to report an error")
		}
	})
}

func TestCoverageParsesOutput(t *testing.T) {
	module := exampleModule
	mod := gomodule.GoModule{
		Name:       module,
		CallingDir: "path",
	}
	cov := coverage.NewWithCmd(fakeExecCommandSuccess(t, nil), "testdata/valid", mod)
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

	got, err := cov.Run()
	if err != nil {
		t.Fatal(err)
	}

	if !cmp.Equal(got.Profile, want.Profile) {
		t.Error(cmp.Diff(got, want))
	}
	if got.Elapsed == 0 {
		t.Error("expected elapsed time to be greater than 0")
	}
}

func TestParseOutputFail(t *testing.T) {
	mod := gomodule.GoModule{
		Name:       exampleModule,
		CallingDir: allPackages,
	}
	cov := coverage.NewWithCmd(fakeExecCommandSuccess(t, nil), "testdata/invalid", mod)

	if _, err := cov.Run(); err == nil {
		t.Error("expected an error")
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

type execContext = func(name string, args ...string) *exec.Cmd

func fakeExecCommandSuccess(t *testing.T, got *commandHolder) execContext {
	t.Helper()

	return func(command string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestCoverageProcessSuccess", "--", command}
		cs = append(cs, args...)
		// #nosec G204 G702 - We are in tests, we don't care
		cmd := exec.CommandContext(t.Context(), os.Args[0], cs...)
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

	return func(command string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestCoverageProcessSuccess", "--", command}
		if executed == run {
			cs = []string{"-test.run=TestCoverageProcessFailure", "--", command}
		}
		cs = append(cs, args...)
		// #nosec G204 G702 - We are in tests, we don't care
		cmd := exec.CommandContext(t.Context(), os.Args[0], cs...)
		cmd.Env = []string{"GO_TEST_PROCESS=1"}
		executed++

		return cmd
	}
}
