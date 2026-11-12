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

package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/OpenRAE/go-rae-gremlins/internal/configuration"
	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
)

func TestUnleash(t *testing.T) {
	c, err := newUnleashCmd(context.Background())
	if err != nil {
		t.Fatal("newUnleashCmd should no fail")
	}
	cmd := c.cmd

	if cmd.Name() != "unleash" {
		t.Errorf("expected 'unleash', got %q", cmd.Name())
	}

	flags := cmd.Flags()

	testCases := []struct {
		name      string
		shorthand string
		flagType  string
		defValue  string
	}{
		{
			name:     "arithmetic-base",
			flagType: boolType,
			defValue: trueValue,
		},
		{
			name:     "conditionals-boundary",
			flagType: boolType,
			defValue: trueValue,
		},
		{
			name:     "conditionals_negation",
			flagType: boolType,
			defValue: trueValue,
		},
		{
			name:     "coverpkg",
			flagType: stringType,
			defValue: "",
		},
		{
			name:      "diff",
			shorthand: "D",
			flagType:  stringType,
			defValue:  "",
		},
		{
			name:      "dry-run",
			shorthand: "d",
			flagType:  boolType,
			defValue:  falseValue,
		},
		{
			name:     "increment-decrement",
			flagType: boolType,
			defValue: trueValue,
		},
		{
			name:      "integration",
			shorthand: "i",
			flagType:  boolType,
			defValue:  falseValue,
		},
		{
			name:     "invert-assignments",
			flagType: boolType,
			defValue: falseValue,
		},
		{
			name:     "invert-bitwise",
			flagType: boolType,
			defValue: falseValue,
		},
		{
			name:     "invert-bwassign",
			flagType: boolType,
			defValue: falseValue,
		},

		{
			name:     "invert-logical",
			flagType: boolType,
			defValue: falseValue,
		},
		{
			name:     "invert-loopctrl",
			flagType: boolType,
			defValue: falseValue,
		},
		{
			name:     "invert-negatives",
			flagType: boolType,
			defValue: trueValue,
		},
		{
			name:      "output",
			shorthand: "o",
			flagType:  stringType,
			defValue:  "",
		},
		{
			name:     "remove-self-assignments",
			flagType: boolType,
			defValue: falseValue,
		},
		{
			name:      "tags",
			shorthand: "t",
			flagType:  stringType,
			defValue:  "",
		},
		{
			name:     "test-cpu",
			flagType: intType,
			defValue: "0",
		},
		{
			name:     "threshold-efficacy",
			flagType: "float64",
			defValue: "0",
		},
		{
			name:     "threshold-mcover",
			flagType: "float64",
			defValue: "0",
		},
		{
			name:     "timeout-coefficient",
			flagType: intType,
			defValue: "0",
		},
		{
			name:     "workers",
			flagType: intType,
			defValue: "0",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			f := flags.Lookup(tc.name)
			if f == nil {
				t.Fatalf("expected flag %q to be registered", tc.name)
			}
			if tc.shorthand != "" && f.Shorthand != tc.shorthand {
				t.Errorf("expected %q to have a shorthand %q, got %q", tc.name, tc.shorthand, f.Shorthand)
			}
			if f.Value.Type() != tc.flagType {
				t.Errorf("expected %q to be type %q, got %q", tc.name, f.Value.Type(), f.Value.Type())
			}
			if f.DefValue != tc.defValue {
				t.Errorf("expected %q to have default value %q, got %q", tc.name, tc.defValue, f.DefValue)
			}
		})
	}

	// test for MutantTypes flags
	for _, mt := range mutator.Types {
		s := strings.ToLower(mt.String())
		mtf := flags.Lookup(s)
		if mtf == nil {
			t.Errorf("expected to have flag for mutant type: %s", mt)

			continue
		}

		if mtf.Value.Type() != boolType {
			t.Errorf("expected %q to be a %q, got %q", s, boolType, mtf.Value.Type())
		}
		wantDef := fmt.Sprintf("%v", configuration.IsDefaultEnabled(mt))
		if mtf.DefValue != wantDef {
			t.Errorf("expected %q have default %q, got %q", s, wantDef, mtf.DefValue)
		}
	}
}

func TestFeatureUnleashRejectsInvalidConfiguration(t *testing.T) {
	configuration.Reset()
	t.Cleanup(configuration.Reset)
	if err := configuration.Init([]string{t.TempDir()}); err != nil {
		t.Fatalf("configuration.Init error = %v, want nil", err)
	}
	t.Setenv("GREMLINS_UNLEASH_WORKERS", "four")

	err := runUnleash(context.Background())(nil, []string{t.TempDir()})
	if err == nil {
		t.Fatal("runUnleash error = nil, want an invalid-value error")
	}
	if !strings.Contains(err.Error(), configuration.UnleashWorkersKey) {
		t.Errorf("runUnleash error = %q, want it to name %q", err, configuration.UnleashWorkersKey)
	}
}

func TestFeatureUnleashRequiresGoModule(t *testing.T) {
	configuration.Reset()
	t.Cleanup(configuration.Reset)
	if err := configuration.Init([]string{t.TempDir()}); err != nil {
		t.Fatalf("configuration.Init error = %v, want nil", err)
	}
	dir := t.TempDir()

	err := runUnleash(context.Background())(nil, []string{dir})
	if err == nil || !strings.Contains(err.Error(), "not in a Go module") {
		t.Errorf("runUnleash(%q) error = %v, want a not-in-a-Go-module error", dir, err)
	}
}

func TestFeatureModulePath(t *testing.T) {
	t.Run("argument", func(t *testing.T) {
		got, err := modulePath([]string{"some/path"})
		if err != nil || got != "some/path" {
			t.Errorf("modulePath([some/path]) = %q, %v; want %q, nil", got, err, "some/path")
		}
	})

	t.Run("current directory", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		want, err := os.Getwd()
		if err != nil {
			t.Fatalf("os.Getwd error = %v", err)
		}

		got, err := modulePath(nil)
		if err != nil || got != want {
			t.Errorf("modulePath(nil) = %q, %v; want %q, nil", got, err, want)
		}
	})

	t.Run("current directory removed", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("only Linux os.Getwd fails after removal of the current directory")
		}
		dir := filepath.Join(t.TempDir(), "removed")
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatalf("os.Mkdir(%q) error = %v", dir, err)
		}
		t.Chdir(dir)
		t.Setenv("PWD", "")
		if err := os.Remove(dir); err != nil {
			t.Fatalf("os.Remove(%q) error = %v", dir, err)
		}

		got, err := modulePath(nil)
		if err == nil {
			t.Errorf("modulePath(nil) = %q, nil; want an error", got)
		}
	})
}

func TestRegressionUnleashFailsOnUnparsableSource(t *testing.T) {
	configuration.Reset()
	t.Cleanup(configuration.Reset)
	if err := configuration.Init([]string{t.TempDir()}); err != nil {
		t.Fatalf("configuration.Init error = %v, want nil", err)
	}
	configuration.Set(configuration.UnleashDryRunKey, true)

	// The build constraint keeps broken.go out of the coverage run, so only
	// the mutant discovery reads and parses it.
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":       "module example.com/broken\n\ngo 1.25\n",
		"pass.go":      "package broken\n\nfunc Pass(a int) bool { return a > 0 }\n",
		"pass_test.go": "package broken\n\nimport \"testing\"\n\nfunc TestPass(t *testing.T) {\n\tif !Pass(1) {\n\t\tt.Fail()\n\t}\n}\n",
		"broken.go":    "//go:build ignore\n\npackage broken\n\nfunc broken() {\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// The coverage run changes the process directory to the module root. Restore
	// it before the TempDir cleanup, which cannot remove a current directory on Windows.
	t.Chdir(dir)

	err := runUnleash(context.Background())(nil, []string{dir})
	if err == nil || !strings.Contains(err.Error(), "broken.go") {
		t.Errorf("runUnleash(%q) error = %v, want a parse error that names broken.go", dir, err)
	}
}
