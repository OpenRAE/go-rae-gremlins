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
	"go.mod":   "module probe\n\ngo 1.25\n",
	"probe.go": "package probe\n\nfunc Add(a, b int) int { return a + b }\n\nfunc Sub(a, b int) int { return a - b }\n",
	"probe_test.go": `package probe

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
