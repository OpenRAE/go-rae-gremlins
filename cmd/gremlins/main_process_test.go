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

package main

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	// mainHelperEnv makes the test binary run main with its arguments.
	mainHelperEnv = "GO_TEST_MAIN_HELPER"
	// mainHelperLimit bounds the lifetime of one helper process.
	mainHelperLimit = 2 * time.Minute
	unleashCmd      = "unleash"
	// oneLivedOutput is the report line of a run with one LIVED mutant.
	oneLivedOutput = "Killed: 0, Lived: 1"
)

// The file names and contents that the probe modules share.
const (
	probeModFile  = "go.mod"
	probeSrcFile  = "probe.go"
	probeTestFile = "probe_test.go"
	probeMod      = "module probe\n\ngo 1.25\n"
	probeAddSrc   = "package probe\n\nfunc Add(a, b int) int { return a + b }\n"
)

// TestMain runs main in place of the tests when mainHelperEnv is set, so the
// tests can check the exit status of the actual binary entry point.
func TestMain(m *testing.M) {
	if os.Getenv(mainHelperEnv) == "1" {
		main()

		return
	}
	os.Exit(m.Run()) // skipcq: RVV-A0003
}

// probeFiles is a module with one covered mutant that its test does not
// kill and one mutant that no test covers. Its efficacy and its mutant
// coverage are both below 100 percent.
var probeFiles = map[string]string{
	probeModFile: probeMod,
	probeSrcFile: probeAddSrc + "\nfunc Sub(a, b int) int { return a - b }\n",
	probeTestFile: `package probe

import "testing"

func TestAdd(t *testing.T) { _ = Add(1, 2) }
`,
}

func TestFeatureMainExitStatus(t *testing.T) {
	testCases := []struct {
		name       string
		wantOutput string
		args       []string
		wantCode   int
	}{
		{
			name:       "version exits 0",
			args:       []string{"--version"},
			wantCode:   0,
			wantOutput: devVersion,
		},
		{
			name:       "unknown flag exits 1",
			args:       []string{unleashCmd, "--no-such-flag"},
			wantCode:   1,
			wantOutput: "unknown flag",
		},
		{
			name:       "run without thresholds exits 0",
			args:       []string{unleashCmd},
			wantCode:   0,
			wantOutput: "Mutator coverage",
		},
		{
			name:       "efficacy below the threshold exits 10",
			args:       []string{unleashCmd, "--threshold-efficacy", "100"},
			wantCode:   10,
			wantOutput: "below efficacy-threshold",
		},
		{
			name:       "mutant coverage below the threshold exits 11",
			args:       []string{unleashCmd, "--threshold-mcover", "100"},
			wantCode:   11,
			wantOutput: "below mutant coverage-threshold",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			module := writeModule(t, probeFiles)
			code, output := runMain(t, module, tc.args...)
			if code != tc.wantCode {
				t.Errorf("exit code = %d, want %d; output:\n%s", code, tc.wantCode, output)
			}
			if !strings.Contains(output, tc.wantOutput) {
				t.Errorf("output does not contain %q; output:\n%s", tc.wantOutput, output)
			}
		})
	}
}

// writeModule writes files to a new directory and returns its path.
func writeModule(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	return root
}

// mainCommand returns a command that runs main with args in the module dir.
// The command reads only the empty configuration file of the module, and
// stops after mainHelperLimit.
func mainCommand(t *testing.T, dir string, args ...string) (*exec.Cmd, *bytes.Buffer) {
	t.Helper()
	config := filepath.Join(dir, ".gremlins.yaml")
	if err := os.WriteFile(config, nil, 0o600); err != nil {
		t.Fatalf("write the configuration file: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), mainHelperLimit)
	t.Cleanup(cancel)
	// #nosec G204 G702 - The test runs its own binary.
	cmd := exec.CommandContext(ctx, os.Args[0], append([]string{"--config", config}, args...)...)
	cmd.Dir = dir
	cmd.Env = helperEnv()
	cmd.WaitDelay = time.Second
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	return cmd, &output
}

// helperEnv returns the environment of the test process without Gremlins
// overrides, with a Go configuration that cannot use a workspace, another
// toolchain, or the network.
func helperEnv() []string {
	var env []string
	for _, v := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(v), "GREMLINS_") {
			env = append(env, v)
		}
	}

	// exec.Cmd uses the last value of a duplicate variable.
	return append(env, mainHelperEnv+"=1", "GOWORK=off", "GOTOOLCHAIN=local", "GOPROXY=off", "GOFLAGS=")
}

// runMain runs main with args in dir and returns its exit code and output.
func runMain(t *testing.T, dir string, args ...string) (int, string) {
	t.Helper()
	cmd, output := mainCommand(t, dir, args...)

	return exitCode(t, cmd.Run()), output.String()
}

// exitCode returns the exit code of a completed command from its error.
func exitCode(t *testing.T, err error) int {
	t.Helper()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	if err != nil {
		t.Fatalf("run main: %v", err)
	}

	return 0
}

// gcflagsProbeFiles is a module whose test fails unless its test binary was
// built with -gcflags=all=-l. The test does not check the result of Add, so
// its one covered mutant lives when the mutant run gets the same flags and
// is killed when it does not.
var gcflagsProbeFiles = map[string]string{
	probeModFile: probeMod,
	probeSrcFile: probeAddSrc,
	probeTestFile: `package probe

import (
	"runtime/debug"
	"testing"
)

func TestAdd(t *testing.T) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		t.Fatal("no build information")
	}
	for _, s := range info.Settings {
		if s.Key == "-gcflags" && s.Value == "all=-l" {
			_ = Add(1, 2)

			return
		}
	}
	t.Fatal("the test binary was not built with -gcflags=all=-l")
}
`,
}

func TestFeatureGOFLAGSReachesEveryTestRun(t *testing.T) {
	testCases := []struct {
		name       string
		goflags    string
		wantOutput string
		args       []string
		wantCode   int
	}{
		{
			name:       "package mode",
			goflags:    "-gcflags=all=-l",
			args:       []string{unleashCmd},
			wantCode:   0,
			wantOutput: oneLivedOutput,
		},
		{
			name:       "integration mode",
			goflags:    "-gcflags=all=-l",
			args:       []string{unleashCmd, "--integration"},
			wantCode:   0,
			wantOutput: oneLivedOutput,
		},
		{
			name:       "coverage run fails without the flags",
			goflags:    "",
			args:       []string{unleashCmd},
			wantCode:   1,
			wantOutput: "not built with -gcflags=all=-l",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			module := writeModule(t, gcflagsProbeFiles)
			cmd, output := mainCommand(t, module, tc.args...)
			// exec.Cmd uses the last value of a duplicate variable.
			cmd.Env = append(cmd.Env, "GOFLAGS="+tc.goflags)
			code := exitCode(t, cmd.Run())
			if code != tc.wantCode {
				t.Errorf("exit code = %d, want %d; output:\n%s", code, tc.wantCode, output)
			}
			if !strings.Contains(output.String(), tc.wantOutput) {
				t.Errorf("output does not contain %q; output:\n%s", tc.wantOutput, output)
			}
		})
	}
}

// cacheProbeTest is a test that appends the GOCACHE of its test binary to
// the file in PROBE_RECORD. It does not check the result of Add, so its one
// covered mutant lives. With fail set, it fails after it records GOCACHE.
func cacheProbeTest(fail bool) string {
	end := "_ = Add(1, 2)"
	if fail {
		end = `t.Fatal("probe failure")`
	}

	return `package probe

import (
	"os"
	"testing"
)

func TestAdd(t *testing.T) {
	f, err := os.OpenFile(os.Getenv("PROBE_RECORD"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(os.Getenv("GOCACHE") + "\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	` + end + `
}
`
}

func TestFeatureRunUsesTemporaryBuildCache(t *testing.T) {
	testCases := []struct {
		name       string
		wantOutput string
		wantRuns   int
		wantCode   int
		fail       bool
	}{
		{
			name:       "coverage and mutant runs share the cache",
			wantRuns:   2,
			wantCode:   0,
			wantOutput: oneLivedOutput,
		},
		{
			name:       "failed coverage run",
			fail:       true,
			wantRuns:   1,
			wantCode:   1,
			wantOutput: "probe failure",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			module := writeModule(t, map[string]string{
				probeModFile:  probeMod,
				probeSrcFile:  probeAddSrc,
				probeTestFile: cacheProbeTest(tc.fail),
			})
			userCache := t.TempDir()
			sentinel := filepath.Join(userCache, "sentinel")
			if err := os.WriteFile(sentinel, nil, 0o600); err != nil {
				t.Fatalf("write the sentinel: %v", err)
			}
			recordDir := t.TempDir()
			cmd, output := mainCommand(t, module, unleashCmd)
			// exec.Cmd uses the last value of a duplicate variable.
			cmd.Env = append(cmd.Env, "GOCACHE="+userCache, "PROBE_RECORD="+filepath.Join(recordDir, probeRecordFile))

			code := exitCode(t, cmd.Run())

			if code != tc.wantCode {
				t.Errorf("exit code = %d, want %d; output:\n%s", code, tc.wantCode, output)
			}
			if !strings.Contains(output.String(), tc.wantOutput) {
				t.Errorf("output does not contain %q; output:\n%s", tc.wantOutput, output)
			}
			assertRunCache(t, recordDir, tc.wantRuns)
			assertDirHolds(t, userCache, "sentinel")
		})
	}
}

// probeRecordFile is the file in which the cache probe test records GOCACHE.
const probeRecordFile = "record"

// assertRunCache checks that the probe record in recordDir holds the same
// build cache wantRuns times, that the cache was in the work directory of
// the run, and that the run removed it.
func assertRunCache(t *testing.T, recordDir string, wantRuns int) {
	t.Helper()
	data, err := fs.ReadFile(os.DirFS(recordDir), probeRecordFile)
	if err != nil {
		t.Fatalf("read the probe record: %v", err)
	}
	caches := strings.Fields(string(data))
	if len(caches) != wantRuns {
		t.Fatalf("test runs recorded %d caches %q, want %d", len(caches), caches, wantRuns)
	}
	for _, c := range caches[1:] {
		if c != caches[0] {
			t.Errorf("test runs used caches %q, want one cache", caches)
		}
	}
	cache := caches[0]
	if !filepath.IsAbs(cache) || filepath.Base(cache) != "gocache" ||
		!strings.HasPrefix(filepath.Base(filepath.Dir(cache)), "gremlins-") {
		t.Errorf("GOCACHE = %q, want an absolute gocache directory in a gremlins- work directory", cache)
	}
	if _, err := os.Stat(cache); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("stat the cache %q after the run: %v, want %v", cache, err, os.ErrNotExist)
	}
}

// assertDirHolds checks that dir holds only the entry name.
func assertDirHolds(t *testing.T, dir, name string) {
	t.Helper()
	entries, err := fs.ReadDir(os.DirFS(dir), ".")
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if len(names) != 1 || names[0] != name {
		t.Errorf("%s holds %q, want only %q", dir, names, name)
	}
}

func TestRegressionRelativeTempDirKeepsMutantStatus(t *testing.T) {
	module := writeModule(t, gcflagsProbeFiles)
	if err := os.Mkdir(filepath.Join(filepath.Dir(module), "reltmp"), 0o700); err != nil {
		t.Fatalf("make the temporary directory: %v", err)
	}
	cmd, output := mainCommand(t, module, unleashCmd)
	// The command runs in the module, so the relative temporary directory is
	// outside the module. exec.Cmd uses the last value of a duplicate variable.
	tmpDir := filepath.Join("..", "reltmp")
	cmd.Env = append(cmd.Env, "GOFLAGS=-gcflags=all=-l", "TMPDIR="+tmpDir, "TMP="+tmpDir, "TEMP="+tmpDir)

	code := exitCode(t, cmd.Run())

	if code != 0 {
		t.Errorf("exit code = %d, want 0; output:\n%s", code, output)
	}
	if !strings.Contains(output.String(), oneLivedOutput) {
		t.Errorf("output does not contain %q; output:\n%s", oneLivedOutput, output)
	}
}
