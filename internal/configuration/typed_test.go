package configuration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
)

const (
	envDryRun   = "GREMLINS_UNLEASH_DRY_RUN"
	envWorkers  = "GREMLINS_UNLEASH_WORKERS"
	envEfficacy = "GREMLINS_UNLEASH_THRESHOLD_EFFICACY"
	envExcludes = "GREMLINS_UNLEASH_EXCLUDE_FILES"
	trueValue   = "true"
	badBool     = "maybe"
	badInt      = "four"
)

const precedenceConfig = `unleash:
  dry-run: false
  workers: 2
  exclude-files:
    - file-a
    - file-b
  threshold:
    efficacy: 50
`

// initIsolated resets Viper, clears inherited GREMLINS_* values, and loads
// only the configuration in dir.
func initIsolated(t *testing.T, dir string) {
	t.Helper()
	Reset()
	t.Cleanup(Reset)
	for _, e := range os.Environ() {
		name, _, _ := strings.Cut(e, "=")
		if strings.HasPrefix(name, gremlinsEnvVarPrefix+"_") {
			t.Setenv(name, "")
			if err := os.Unsetenv(name); err != nil {
				t.Fatalf("unset %s: %v", name, err)
			}
		}
	}
	if err := Init([]string{dir}); err != nil {
		t.Fatalf("Init(%q) error = %v, want nil", dir, err)
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, gremlinsCfgName+".yaml"), []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	return dir
}

func TestFeatureGetConvertsEnvironmentValues(t *testing.T) {
	initIsolated(t, t.TempDir())
	t.Setenv(envDryRun, trueValue)
	t.Setenv(envWorkers, "4")
	t.Setenv(envEfficacy, "90")
	t.Setenv(envExcludes, "a_test.go b_test.go")

	if got := Get[bool](UnleashDryRunKey); got != true {
		t.Errorf("Get[bool](%q) = %v, want true", UnleashDryRunKey, got)
	}
	if got := Get[int](UnleashWorkersKey); got != 4 {
		t.Errorf("Get[int](%q) = %d, want 4", UnleashWorkersKey, got)
	}
	if got := Get[float64](UnleashThresholdEfficacyKey); got != 90 {
		t.Errorf("Get[float64](%q) = %v, want 90", UnleashThresholdEfficacyKey, got)
	}
	if diff := cmp.Diff([]string{"a_test.go", "b_test.go"}, Get[[]string](UnleashExcludeFiles)); diff != "" {
		t.Errorf("Get[[]string](%q) mismatch (-want +got):\n%s", UnleashExcludeFiles, diff)
	}
	if err := Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestFeatureGetPrecedence(t *testing.T) {
	testCases := []struct {
		env          map[string]string
		flags        []string
		wantExcludes []string
		name         string
		wantWorkers  int
		wantEfficacy float64
		wantDryRun   bool
	}{
		{
			name:         "file",
			wantDryRun:   false,
			wantWorkers:  2,
			wantEfficacy: 50,
			wantExcludes: []string{"file-a", "file-b"},
		},
		{
			name: "environment overrides file",
			env: map[string]string{
				envDryRun:   trueValue,
				envWorkers:  "4",
				envEfficacy: "90",
				envExcludes: "env-a env-b",
			},
			wantDryRun:   true,
			wantWorkers:  4,
			wantEfficacy: 90,
			wantExcludes: []string{"env-a", "env-b"},
		},
		{
			name: "flag overrides environment",
			env: map[string]string{
				envDryRun:   trueValue,
				envWorkers:  "4",
				envEfficacy: "90",
				envExcludes: "env-a env-b",
			},
			flags:        []string{"--dry-run=false", "--workers=8", "--threshold-efficacy=75.5", "--exclude-files=flag-a"},
			wantDryRun:   false,
			wantWorkers:  8,
			wantEfficacy: 75.5,
			wantExcludes: []string{"flag-a"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			initIsolated(t, writeConfig(t, precedenceConfig))
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			bindFlags(t, tc.flags)

			if got := Get[bool](UnleashDryRunKey); got != tc.wantDryRun {
				t.Errorf("Get[bool](%q) = %v, want %v", UnleashDryRunKey, got, tc.wantDryRun)
			}
			if got := Get[int](UnleashWorkersKey); got != tc.wantWorkers {
				t.Errorf("Get[int](%q) = %d, want %d", UnleashWorkersKey, got, tc.wantWorkers)
			}
			if got := Get[float64](UnleashThresholdEfficacyKey); got != tc.wantEfficacy {
				t.Errorf("Get[float64](%q) = %v, want %v", UnleashThresholdEfficacyKey, got, tc.wantEfficacy)
			}
			if diff := cmp.Diff(tc.wantExcludes, Get[[]string](UnleashExcludeFiles)); diff != "" {
				t.Errorf("Get[[]string](%q) mismatch (-want +got):\n%s", UnleashExcludeFiles, diff)
			}
			if err := Validate(); err != nil {
				t.Errorf("Validate() error = %v, want nil", err)
			}
		})
	}
}

// bindFlags registers flags of the same types as the unleash command, parses
// args, and binds the flags to Viper.
func bindFlags(t *testing.T, args []string) {
	t.Helper()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.Bool("dry-run", false, "")
	fs.Int("workers", 0, "")
	fs.Float64("threshold-efficacy", 0, "")
	fs.StringArray("exclude-files", nil, "")
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse flags %v: %v", args, err)
	}
	bindings := map[string]string{
		UnleashDryRunKey:            "dry-run",
		UnleashWorkersKey:           "workers",
		UnleashThresholdEfficacyKey: "threshold-efficacy",
		UnleashExcludeFiles:         "exclude-files",
	}
	for key, name := range bindings {
		if err := viper.BindPFlag(key, fs.Lookup(name)); err != nil {
			t.Fatalf("bind flag %s: %v", name, err)
		}
	}
}

func TestFeatureValidateRejectsInvalidValues(t *testing.T) {
	testCases := []struct {
		env     map[string]string
		config  string
		name    string
		wantKey []string
	}{
		{
			name:    "bool from environment",
			env:     map[string]string{envDryRun: badBool},
			wantKey: []string{UnleashDryRunKey},
		},
		{
			name:    "int from environment",
			env:     map[string]string{envWorkers: badInt},
			wantKey: []string{UnleashWorkersKey},
		},
		{
			name:    "float from environment",
			env:     map[string]string{envEfficacy: "high"},
			wantKey: []string{UnleashThresholdEfficacyKey},
		},
		{
			name:    "string slice from file",
			config:  "unleash:\n  exclude-files:\n    nested: value\n",
			wantKey: []string{UnleashExcludeFiles},
		},
		{
			name:    "mutant enabled from file",
			config:  "mutants:\n  arithmetic-base:\n    enabled: sometimes\n",
			wantKey: []string{MutantTypeEnabledKey(mutator.ArithmeticBase)},
		},
		{
			name: "every invalid key",
			env: map[string]string{
				envDryRun:  badBool,
				envWorkers: badInt,
			},
			wantKey: []string{UnleashDryRunKey, UnleashWorkersKey},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			initIsolated(t, writeConfig(t, tc.config))
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			err := Validate()
			if err == nil {
				t.Fatal("Validate() error = nil, want an invalid-value error")
			}
			for _, k := range tc.wantKey {
				if !strings.Contains(err.Error(), `"`+k+`"`) {
					t.Errorf("Validate() error = %q, want it to name key %q", err, k)
				}
			}
		})
	}
}

func TestFeatureValidateAcceptsUnsetValues(t *testing.T) {
	initIsolated(t, t.TempDir())

	if err := Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestFeatureGetReturnsZeroForInvalidValue(t *testing.T) {
	initIsolated(t, t.TempDir())
	t.Setenv(envWorkers, badInt)

	if got := Get[int](UnleashWorkersKey); got != 0 {
		t.Errorf("Get[int](%q) = %d, want 0", UnleashWorkersKey, got)
	}
}

func TestFeatureTypedValueChecks(t *testing.T) {
	testCases := []struct {
		check   func(any) error
		name    string
		valid   []any
		invalid []any
	}{
		{
			name:    "bool",
			check:   checkBool,
			valid:   []any{true, trueValue, "1", "F"},
			invalid: []any{badBool, 1, []any{true}},
		},
		{
			name:    "int",
			check:   checkInt,
			valid:   []any{4, int64(4), uint8(4), float64(4), "4", "-4", "0x10"},
			invalid: []any{badInt, "08", "4.5", float64(4.5), true, map[string]any{}},
		},
		{
			name:    "float",
			check:   checkFloat,
			valid:   []any{float64(90.5), 90, "90", "90.5"},
			invalid: []any{"high", true, []any{90}},
		},
		{
			name:    "string",
			check:   checkString,
			valid:   []any{"tag1,tag2", 123, true},
			invalid: []any{[]any{"a"}, map[string]any{"a": "b"}},
		},
		{
			name:    "string slice",
			check:   checkStringSlice,
			valid:   []any{"a b", []string{"a", "b"}, []any{"a", 1}},
			invalid: []any{[]any{"a", map[string]any{}}, map[string]any{"a": "b"}, 1},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, v := range tc.valid {
				if err := tc.check(v); err != nil {
					t.Errorf("check(%#v) error = %v, want nil", v, err)
				}
			}
			for _, v := range tc.invalid {
				if err := tc.check(v); err == nil {
					t.Errorf("check(%#v) error = nil, want an error", v)
				}
			}
		})
	}
}

func TestFeatureGetConvertsFileValues(t *testing.T) {
	initIsolated(t, writeConfig(t, "unleash:\n  workers: 4.0\n  threshold:\n    efficacy: 90\n  tags: 123\n"))

	if got := Get[int](UnleashWorkersKey); got != 4 {
		t.Errorf("Get[int](%q) = %d, want 4", UnleashWorkersKey, got)
	}
	if got := Get[float64](UnleashThresholdEfficacyKey); got != 90 {
		t.Errorf("Get[float64](%q) = %v, want 90", UnleashThresholdEfficacyKey, got)
	}
	if got := Get[string](UnleashTagsKey); got != "123" {
		t.Errorf("Get[string](%q) = %q, want %q", UnleashTagsKey, got, "123")
	}
	if err := Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestFeatureGetUntypedReturnsRawValue(t *testing.T) {
	initIsolated(t, t.TempDir())
	Set("test.rune", 'a')

	if got := Get[rune]("test.rune"); got != 'a' {
		t.Errorf("Get[rune](%q) = %v, want 'a'", "test.rune", got)
	}
	if got := Get[[]string](UnleashExcludeFiles); len(got) != 0 {
		t.Errorf("Get[[]string](%q) = %#v, want empty", UnleashExcludeFiles, got)
	}
}
