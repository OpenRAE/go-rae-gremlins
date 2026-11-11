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
	"github.com/spf13/viper"

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
			if tc.envEntries != nil {
				for _, e := range tc.envEntries {
					t.Setenv(e.name, e.value)
				}
			}
			err := Init(tc.configPaths)
			if tc.expectErr && err == nil {
				t.Fatal("expected error")
			}
			if tc.expectErr {
				return
			}

			for key, wanted := range tc.wantedConfig {
				got := Get[any](key)
				if got != wanted {
					t.Error(cmp.Diff(got, wanted))
				}
			}
			viper.Reset()
		})
	}
}

func TestInitConfigurationFileErrors(t *testing.T) {
	const malformed = "unleash: [malformed\n"

	testCases := []struct {
		setup     func(t *testing.T) []string
		name      string
		expectErr bool
	}{
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

				return []string{writeConfigFile(t, t.TempDir(), malformed, 0o600)}
			},
			expectErr: true,
		},
		{
			name: "malformed discovered file returns error",
			setup: func(t *testing.T) []string {
				t.Helper()
				dir := t.TempDir()
				writeConfigFile(t, dir, malformed, 0o600)

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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(viper.Reset)
			paths := tc.setup(t)

			err := Init(paths)

			if tc.expectErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.expectErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
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
	home, err := homedir.Dir()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("it lookups in default locations", func(t *testing.T) {
		t.Chdir("testdata/config1")

		var want []string

		// First global
		if runtime.GOOS != windowsOs {
			want = append(want, "/etc/gremlins")
		}

		// Then $XDG_CONFIG_HOME and $HOME
		want = append(want,
			filepath.Join(home, ".config", "gremlins", "gremlins"),
			filepath.Join(home, ".gremlins"),
		)

		// Then module root
		moduleRoot, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		want = append(want, moduleRoot)

		// Last current folder
		want = append(want, ".")

		got := defaultConfigPaths()

		if !cmp.Equal(got, want) {
			t.Error(cmp.Diff(got, want))
		}
	})

	t.Run("no module root if not in go module", func(t *testing.T) {
		t.Chdir(t.TempDir())

		var want []string

		// First global
		if runtime.GOOS != windowsOs {
			want = append(want, "/etc/gremlins")
		}

		// Then $XDG_CONFIG_HOME and $HOME
		want = append(want,
			filepath.Join(home, ".config", "gremlins", "gremlins"),
			filepath.Join(home, ".gremlins"),
		)

		// Last current folder
		want = append(want, ".")

		got := defaultConfigPaths()

		if !cmp.Equal(got, want) {
			t.Error(cmp.Diff(got, want))
		}
	})

	t.Run("when XDG_CONFIG_HOME is set, it lookups in that locations", func(t *testing.T) {
		t.Chdir("testdata/config1")

		customPath := filepath.Join("my", "custom", "path")
		t.Setenv("XDG_CONFIG_HOME", customPath)

		var want []string

		// First global
		if runtime.GOOS != windowsOs {
			want = append(want, "/etc/gremlins")
		}

		// Then $XDG_CONFIG_HOME and $HOME
		want = append(want,
			filepath.Join(customPath, "gremlins", "gremlins"),
			filepath.Join(home, ".gremlins"))

		// Then Go module root
		moduleRoot, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		want = append(want, moduleRoot)

		// Last the current directory
		want = append(want, ".")

		got := defaultConfigPaths()

		if !cmp.Equal(got, want) {
			t.Error(cmp.Diff(got, want))
		}
	})
}

func TestGeneratesMutantTypeEnabledKey(t *testing.T) {
	mt := mutator.ArithmeticBase
	want := "mutants.arithmetic-base.enabled"

	got := MutantTypeEnabledKey(mt)

	if got != want {
		t.Errorf("expected %q, got %q", mt, want)
	}
}

func TestViperSynchronisedAccess(t *testing.T) {
	t.Parallel()
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
				t.Errorf("expected %v, got %v", tc.value, got)
			}
		})
	}
}

func TestReset(t *testing.T) {
	Set("test.key", true)

	Reset()

	got := Get[bool]("test.key")

	if got != false {
		t.Errorf("expected config to be reset")
	}
}
