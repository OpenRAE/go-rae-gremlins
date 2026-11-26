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

package configuration

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/mitchellh/go-homedir"

	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
)

const (
	testTags    = "tag1,tag2,tag3"
	testConfig1 = "./testdata/config1"
)

type envEntry struct {
	name  string
	value string
}

func TestConfiguration(t *testing.T) {
	testCases := []struct {
		wantedConfig map[string]any
		name         string
		configPaths  []string
		envEntries   []envEntry
		expectErr    bool
	}{
		{
			name:        "from single file",
			configPaths: []string{"testdata/config1/.gremlins.yaml"},
			wantedConfig: map[string]any{
				UnleashDryRunKey: true,
				UnleashTagsKey:   testTags,
			},
		},
		{
			name:        "from returns error if unreadable",
			configPaths: []string{"testdata/config1/.gremlin"},
			wantedConfig: map[string]any{
				UnleashDryRunKey: true,
				UnleashTagsKey:   testTags,
			},
			expectErr: true,
		},
		{
			name:        "from cfg",
			configPaths: []string{testConfig1},
			wantedConfig: map[string]any{
				UnleashDryRunKey: true,
				UnleashTagsKey:   testTags,
			},
		},
		{
			name:        "from cfg multi",
			configPaths: []string{"./testdata/config2", testConfig1},
			wantedConfig: map[string]any{
				UnleashDryRunKey: true,
				UnleashTagsKey:   "tag1.2,tag2.2,tag3.2",
			},
		},
		{
			name: "from env",
			envEntries: []envEntry{
				{name: envDryRun, value: trueValue},
				{name: "GREMLINS_UNLEASH_TAGS", value: testTags},
			},
			wantedConfig: map[string]any{
				UnleashDryRunKey: trueValue,
				UnleashTagsKey:   testTags,
			},
		},
		{
			name: "from cfg override with env",
			envEntries: []envEntry{
				{name: "GREMLINS_UNLEASH_TAGS", value: "tag1env,tag2env,tag3env"},
			},
			configPaths: []string{testConfig1},
			wantedConfig: map[string]any{
				UnleashDryRunKey: true,
				UnleashTagsKey:   "tag1env,tag2env,tag3env",
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			Reset()
			t.Cleanup(Reset)
			for _, e := range tc.envEntries {
				t.Setenv(e.name, e.value)
			}
			err := Init(tc.configPaths)
			if tc.expectErr && err == nil {
				t.Fatalf("Init(%q) error = nil, want an error", tc.configPaths)
			}
			if !tc.expectErr && err != nil {
				t.Fatalf("Init(%q) error = %v, want nil", tc.configPaths, err)
			}
			if tc.expectErr {
				return
			}

			for key, wanted := range tc.wantedConfig {
				got := Get[any](key)
				if got != wanted {
					t.Errorf("Get[any](%q) = %v, want %v", key, got, wanted)
				}
			}
		})
	}
}

const malformedConfig = "unleash: [malformed\n"

type initCase struct {
	setup     func(t *testing.T) []string
	name      string
	expectErr bool
}

func TestFeatureInitConfigurationFile(t *testing.T) {
	runInitCases(t, []initCase{
		{
			name: "missing discovered file is allowed",
			setup: func(t *testing.T) []string {
				t.Helper()

				return []string{t.TempDir()}
			},
		},
		{
			name: "malformed explicit file returns error",
			setup: func(t *testing.T) []string {
				t.Helper()

				return []string{writeConfigFile(t, t.TempDir(), malformedConfig, 0o600)}
			},
			expectErr: true,
		},
	})
}

func TestRegressionInitDiscoveredConfigurationErrors(t *testing.T) {
	runInitCases(t, []initCase{
		{
			name: "malformed discovered file returns error",
			setup: func(t *testing.T) []string {
				t.Helper()
				dir := t.TempDir()
				writeConfigFile(t, dir, malformedConfig, 0o600)

				return []string{dir}
			},
			expectErr: true,
		},
		{
			name: "unreadable discovered file returns error",
			setup: func(t *testing.T) []string {
				t.Helper()
				if runtime.GOOS == windowsOs || os.Geteuid() == 0 {
					t.Skip("file permissions do not block reads on this platform or user")
				}
				dir := t.TempDir()
				writeConfigFile(t, dir, "unleash:\n  dry-run: true\n", 0o000)

				return []string{dir}
			},
			expectErr: true,
		},
	})
}

func runInitCases(t *testing.T, testCases []initCase) {
	t.Helper()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			Reset()
			t.Cleanup(Reset)
			paths := tc.setup(t)

			err := Init(paths)

			if tc.expectErr && err == nil {
				t.Fatal("Init() error = nil, want an error")
			}
			if !tc.expectErr && err != nil {
				t.Fatalf("Init() error = %v, want nil", err)
			}
		})
	}
}

func writeConfigFile(t *testing.T, dir, content string, perm os.FileMode) string {
	t.Helper()
	path := filepath.Join(dir, gremlinsCfgName+".yaml")
	if err := os.WriteFile(path, []byte(content), perm); err != nil {
		t.Fatal(err)
	}

	return path
}

func TestConfigPaths(t *testing.T) {
	home := testHome(t)

	t.Run("it lookups in default locations", func(t *testing.T) {
		t.Chdir("testdata/config1")
		moduleRoot, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}

		want := wantConfigPaths(moduleRoot, filepath.Join(home, ".config", "gremlins"), home)
		if diff := cmp.Diff(want, defaultConfigPaths()); diff != "" {
			t.Errorf("defaultConfigPaths() mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("no module root if not in go module", func(t *testing.T) {
		t.Chdir(t.TempDir())

		want := wantConfigPaths("", filepath.Join(home, ".config", "gremlins"), home)
		if diff := cmp.Diff(want, defaultConfigPaths()); diff != "" {
			t.Errorf("defaultConfigPaths() mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("when XDG_CONFIG_HOME is set, it lookups in that locations", func(t *testing.T) {
		t.Chdir(t.TempDir())
		customPath := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", customPath)

		want := wantConfigPaths("", filepath.Join(customPath, "gremlins"), home)
		if diff := cmp.Diff(want, defaultConfigPaths()); diff != "" {
			t.Errorf("defaultConfigPaths() mismatch (-want +got):\n%s", diff)
		}
	})
}

// TestFeatureConfigPathsIgnoresARelativeXDGConfigHome checks that a relative
// XDG_CONFIG_HOME gives the default ~/.config, as the XDG Base Directory
// Specification requires.
func TestFeatureConfigPathsIgnoresARelativeXDGConfigHome(t *testing.T) {
	home := testHome(t)
	t.Chdir(t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", filepath.Join("my", "custom", "path"))

	want := wantConfigPaths("", filepath.Join(home, ".config", "gremlins"), home)
	if diff := cmp.Diff(want, defaultConfigPaths()); diff != "" {
		t.Errorf("defaultConfigPaths() mismatch (-want +got):\n%s", diff)
	}
}

// wantConfigPaths returns the expected search order: the current directory,
// the module root if there is one, the global directory except on Windows,
// the XDG directory, and the home directory.
func wantConfigPaths(moduleRoot, xdg, home string) []string {
	want := []string{"."}
	if moduleRoot != "" {
		want = append(want, moduleRoot)
	}
	if runtime.GOOS != windowsOs {
		want = append(want, "/etc/gremlins")
	}

	return append(want, xdg, home)
}

// testHome sets a new temporary home directory without XDG_CONFIG_HOME for
// the test and returns it.
func testHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	homedir.DisableCache = true
	t.Cleanup(func() { homedir.DisableCache = false })

	return home
}

// Names of the configuration file locations in the selection tests.
const (
	locCWD    = "cwd"
	locModule = "module"
	locXDG    = "xdg"
	locHome   = "home"
)

// selectionCase is a configuration selection test case: the locations that
// have a configuration file, and the location of the file that Init reads.
type selectionCase struct {
	name  string
	files []string
	want  string
}

// TestRegressionInitSelectsTheConfigurationFileOfHighestPrecedence checks
// that Init reads the file of the first location in the documented order,
// and reads the documented user locations.
func TestRegressionInitSelectsTheConfigurationFileOfHighestPrecedence(t *testing.T) {
	testConfigSelection(t, []selectionCase{
		{name: "current directory over module root", files: []string{locCWD, locModule}, want: locCWD},
		{name: "XDG over home", files: []string{locXDG, locHome}, want: locXDG},
		{name: "home file", files: []string{locHome}, want: locHome},
	})
}

// TestFeatureInitPrefersTheProjectFileOverTheUserFile checks that a file of
// the project has precedence over a user file.
func TestFeatureInitPrefersTheProjectFileOverTheUserFile(t *testing.T) {
	testConfigSelection(t, []selectionCase{
		{name: "current directory over XDG", files: []string{locCWD, locXDG}, want: locCWD},
		{name: "module root over XDG", files: []string{locModule, locXDG}, want: locModule},
	})
}

// testConfigSelection writes the files of each case and checks the file that
// Init reads.
func testConfigSelection(t *testing.T, testCases []selectionCase) {
	t.Helper()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			homeDir := testHome(t)
			moduleDir := t.TempDir()
			cwdDir := filepath.Join(moduleDir, "pkg")
			dirs := map[string]string{
				locCWD: cwdDir, locModule: moduleDir, locXDG: filepath.Join(homeDir, ".config", "gremlins"), locHome: homeDir,
			}
			writeTestFile(t, filepath.Join(moduleDir, "go.mod"), "module example.com/m\n")
			for _, f := range tc.files {
				writeTestFile(t, filepath.Join(dirs[f], ".gremlins.yaml"), "unleash:\n  tags: "+f+"\n")
			}
			if err := os.MkdirAll(cwdDir, 0o750); err != nil {
				t.Fatal(err)
			}
			t.Chdir(cwdDir)
			Reset()
			t.Cleanup(Reset)

			if err := Init(nil); err != nil {
				t.Fatalf("Init(nil) error = %v, want nil", err)
			}
			if got := Get[string](UnleashTagsKey); got != tc.want {
				t.Errorf("%s = %q, want %q", UnleashTagsKey, got, tc.want)
			}
		})
	}
}

func writeTestFile(t *testing.T, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestGeneratesMutantTypeEnabledKey(t *testing.T) {
	mt := mutator.ArithmeticBase
	want := "mutants.arithmetic-base.enabled"

	got := MutantTypeEnabledKey(mt)

	if got != want {
		t.Errorf("MutantTypeEnabledKey(%v) = %q, want %q", mt, got, want)
	}
}

func TestViperSynchronisedAccess(t *testing.T) {
	t.Parallel()
	t.Cleanup(Reset)
	testCases := []struct {
		value any
		name  string
		key   string
	}{
		{
			name:  "bool",
			key:   "tvsa.bool.key",
			value: true,
		},
		{
			name:  "int",
			key:   "tvsa.int.key",
			value: 10,
		},
		{
			name:  "float64",
			key:   "tvsa.float64.key",
			value: float64(10),
		},
		{
			name:  "string",
			key:   "tvsa.string.key",
			value: "test string",
		},
		{
			name:  "char",
			key:   "tvsa.char.key",
			value: 'a',
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			Set(tc.key, tc.value)

			got := Get[any](tc.key)

			if !cmp.Equal(got, tc.value) {
				t.Errorf("Get[any](%q) = %v, want %v", tc.key, got, tc.value)
			}
		})
	}
}

func TestReset(t *testing.T) {
	t.Cleanup(Reset)
	Set("test.key", true)

	Reset()

	got := Get[bool]("test.key")

	if got != false {
		t.Errorf(`Get[bool]("test.key") after Reset() = %t, want false`, got)
	}
}
