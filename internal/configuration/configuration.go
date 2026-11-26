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

// Package configuration manages application settings from flags, config files, and environment variables.
package configuration

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"

	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
)

// This is the list of the keys available in config files and as flags.
const (
	GremlinsSilentKey             = "silent"
	UnleashDryRunKey              = "unleash.dry-run"
	UnleashOutputStatusesKey      = "unleash.output-statuses"
	UnleashOutputKey              = "unleash.output"
	UnleashTagsKey                = "unleash.tags"
	UnleashCoverPkgKey            = "unleash.coverpkg"
	UnleashWorkersKey             = "unleash.workers"
	UnleashTestCPUKey             = "unleash.test-cpu"
	UnleashTimeoutCoefficientKey  = "unleash.timeout-coefficient"
	UnleashTimeoutSafetyMarginKey = "unleash.timeout-safety-margin"
	UnleashMemoryBudgetKey        = "unleash.memory-budget"
	UnleashIntegrationMode        = "unleash.integration"
	UnleashExcludeFiles           = "unleash.exclude-files"
	UnleashDiffRef                = "unleash.diff"
	UnleashThresholdEfficacyKey   = "unleash.threshold.efficacy"
	UnleashThresholdMCoverageKey  = "unleash.threshold.mutant-coverage"
	UnleashOutputDiffStatusesKey  = "unleash.output-diff-statuses"
)

const (
	gremlinsCfgName      = ".gremlins"
	gremlinsEnvVarPrefix = "GREMLINS"

	xdgConfigHomeKey = "XDG_CONFIG_HOME"

	windowsOs = "windows"
)

// Init initializes the viper configuration for Gremlins.
//
// It sets the configuration file name as .gremlins.yaml, adds the passed paths as ConfigPaths
// AutomaticEnv configuration having GREMLINS as prefix.
// The environment variables take precedence over the configuration file and must be set in the
// format:
//
//	GREMLINS_<COMMAND NAME>_<FLAG NAME>
func Init(cPaths []string) error {
	replacer := strings.NewReplacer(".", "_", "-", "_")
	viper.SetEnvKeyReplacer(replacer)
	viper.SetEnvPrefix(gremlinsEnvVarPrefix)
	viper.AutomaticEnv()
	viper.SetConfigName(gremlinsCfgName)
	viper.SetConfigType("yaml")

	if isSpecificFile(cPaths) {
		viper.SetConfigFile(cPaths[0])

		return viper.ReadInConfig()
	}

	if arePathsNotSet(cPaths) {
		cPaths = defaultConfigPaths()
	}

	for _, p := range cPaths {
		viper.AddConfigPath(p)
	}

	// The discovered configuration file is optional. Ignore only its absence.
	var notFound viper.ConfigFileNotFoundError
	if err := viper.ReadInConfig(); err != nil && !errors.As(err, &notFound) {
		return fmt.Errorf("reading configuration: %w", err)
	}

	return nil
}

// MutantTypeEnabledKey returns the configuration key for a mutant.
// The generated key will have the format 'mutants.mutant-name.enabled",
// which corresponds to the Yaml:
//
//		mutants:
//	 		mutant-name:
//	 			enabled: [bool]
func MutantTypeEnabledKey(mt mutator.Type) string {
	m := mt.String()
	m = strings.ReplaceAll(m, "_", "-")
	m = strings.ToLower(m)

	return fmt.Sprintf("mutants.%s.enabled", m)
}

func isSpecificFile(cPaths []string) bool {
	return len(cPaths) == 1 && filepath.Ext(cPaths[0]) != ""
}

func arePathsNotSet(cPaths []string) bool {
	return len(cPaths) == 0 || len(cPaths) == 1 && cPaths[0] == ""
}

// defaultConfigPaths returns the directories that Init searches for the
// configuration file, in order of precedence. Viper reads only the first file
// that it finds, so a project file has precedence over a global or user file.
func defaultConfigPaths() []string {
	result := []string{"."}
	if root := findModuleRoot(); root != "" {
		result = append(result, root)
	}
	if runtime.GOOS != windowsOs {
		result = append(result, "/etc/gremlins")
	}
	if xch := xdgConfigHome(); xch != "" {
		result = append(result, filepath.Join(xch, "gremlins"))
	}
	if home, err := homedir.Dir(); err == nil {
		result = append(result, home)
	}

	return result
}

// xdgConfigHome returns the base directory of user configuration files. The
// XDG Base Directory Specification ignores a relative XDG_CONFIG_HOME and
// gives ~/.config as the default.
func xdgConfigHome() string {
	if x := os.Getenv(xdgConfigHomeKey); filepath.IsAbs(x) {
		return x
	}
	x, err := homedir.Expand("~/.config")
	if err != nil {
		return ""
	}

	return x
}

func findModuleRoot() string {
	// This function is duplicated from internal/gomodule. We should find a way
	// to use here gomodule. The problem is the point of initialization, because
	// configuration is initialised before gomodule.
	path, _ := os.Getwd()
	for {
		if fi, err := os.Stat(filepath.Join(path, "go.mod")); err == nil && !fi.IsDir() {
			return path
		}
		d := filepath.Dir(path)
		if d == path {
			break
		}
		path = d
	}

	return ""
}

var mutex sync.RWMutex

// Set offers synchronised access to Viper.
func Set[T any](k string, v T) {
	mutex.Lock()
	defer mutex.Unlock()
	viper.Set(k, v)
}

// Get offers synchronised access to Viper.
//
// Get reads bool, int, float64, string, and []string values with the Viper
// conversion method for T, so a value from a flag, an environment variable, or
// the configuration file has the same type. Get returns the zero value of T
// when the value does not convert. Validate reports these values as errors.
func Get[T any](k string) T {
	mutex.RLock()
	defer mutex.RUnlock()

	var r T
	var v any
	switch any(r).(type) {
	case bool:
		v = viper.GetBool(k)
	case int:
		v = viper.GetInt(k)
	case float64:
		v = viper.GetFloat64(k)
	case string:
		v = viper.GetString(k)
	case []string:
		v = viper.GetStringSlice(k)
	default:
		v = viper.Get(k)
	}
	if t, ok := v.(T); ok {
		return t
	}

	return r
}

// Validate checks the effective value of each typed configuration key.
// It returns an error that names each key with a value that does not convert
// to the type of the key, or with a numeric value that is out of range.
func Validate() error {
	mutex.RLock()
	defer mutex.RUnlock()

	checks := typedKeys()
	bounds := boundedKeys()
	keys := make([]string, 0, len(checks))
	for k := range checks {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var errs []error
	for _, k := range keys {
		v := viper.Get(k)
		if v == nil {
			continue
		}
		err := checks[k](v)
		if bound, ok := bounds[k]; ok && err == nil {
			err = bound(k)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid value for configuration key %q: %w", k, err))
		}
	}

	return errors.Join(errs...)
}

// typedKeys returns the value check of each configuration key that has a
// fixed value type.
func typedKeys() map[string]func(any) error {
	checks := map[string]func(any) error{
		GremlinsSilentKey:             checkBool,
		UnleashDryRunKey:              checkBool,
		UnleashOutputStatusesKey:      checkString,
		UnleashOutputKey:              checkString,
		UnleashTagsKey:                checkString,
		UnleashCoverPkgKey:            checkString,
		UnleashWorkersKey:             checkInt,
		UnleashTestCPUKey:             checkInt,
		UnleashTimeoutCoefficientKey:  checkInt,
		UnleashTimeoutSafetyMarginKey: checkInt,
		UnleashMemoryBudgetKey:        checkSize,
		UnleashIntegrationMode:        checkBool,
		UnleashExcludeFiles:           checkStringSlice,
		UnleashDiffRef:                checkString,
		UnleashThresholdEfficacyKey:   checkFloat,
		UnleashThresholdMCoverageKey:  checkFloat,
		UnleashOutputDiffStatusesKey:  checkString,
	}
	for _, mt := range mutator.Types {
		checks[MutantTypeEnabledKey(mt)] = checkBool
	}

	return checks
}

// boundedKeys returns the range check of each numeric configuration key.
// Validate runs a range check only on a value that passes its type check.
func boundedKeys() map[string]func(string) error {
	return map[string]func(string) error{
		UnleashWorkersKey:             checkNotNegative,
		UnleashTestCPUKey:             checkNotNegative,
		UnleashTimeoutCoefficientKey:  checkNotNegative,
		UnleashTimeoutSafetyMarginKey: checkNotNegative,
		UnleashThresholdEfficacyKey:   checkPercentage,
		UnleashThresholdMCoverageKey:  checkPercentage,
	}
}

// checkNotNegative accepts 0, which selects the default, and positive values.
func checkNotNegative(k string) error {
	if n := viper.GetInt(k); n < 0 {
		return fmt.Errorf("%d is negative; use 0 for the default or a positive number", n)
	}

	return nil
}

// checkPercentage accepts values from 0 to 100. The value 0 disables the
// threshold.
func checkPercentage(k string) error {
	f := viper.GetFloat64(k)
	if f >= 0 && f <= 100 {
		return nil
	}

	return fmt.Errorf("%v is not a percentage from 0 to 100", f)
}

func checkBool(v any) error {
	switch t := v.(type) {
	case bool:
		return nil
	case string:
		if _, err := strconv.ParseBool(t); err != nil {
			return fmt.Errorf("%q is not a boolean", t)
		}

		return nil
	default:
		return fmt.Errorf("%v (%T) is not a boolean", v, v)
	}
}

func checkInt(v any) error {
	switch t := v.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return nil
	case float64:
		if t != math.Trunc(t) {
			return fmt.Errorf("%v is not an integer", t)
		}

		return nil
	case string:
		if _, err := strconv.ParseInt(t, 0, 0); err != nil {
			return fmt.Errorf("%q is not an integer", t)
		}

		return nil
	default:
		return fmt.Errorf("%v (%T) is not an integer", v, v)
	}
}

func checkFloat(v any) error {
	switch t := v.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return nil
	case string:
		if _, err := strconv.ParseFloat(t, 64); err != nil {
			return fmt.Errorf("%q is not a number", t)
		}

		return nil
	default:
		return fmt.Errorf("%v (%T) is not a number", v, v)
	}
}

// checkSize accepts an empty value, which selects the default, and a size
// that ParseSize accepts.
func checkSize(v any) error {
	if !isScalar(v) {
		return fmt.Errorf("%v (%T) is not a size", v, v)
	}
	s := fmt.Sprint(v)
	if s == "" {
		return nil
	}
	_, err := ParseSize(s)

	return err
}

// ParseSize parses a positive byte count with an optional binary unit
// suffix: K (KiB), M (MiB), G (GiB), or T (TiB). For example, "2G" is 2 GiB.
func ParseSize(s string) (uint64, error) {
	units := map[byte]uint{'K': 10, 'M': 20, 'G': 30, 'T': 40}
	digits, shift := s, uint(0)
	if n := len(s); n > 0 {
		if u, ok := units[s[n-1]]; ok {
			digits, shift = s[:n-1], u
		}
	}
	n, err := strconv.ParseUint(digits, 10, 64)
	if err != nil || n == 0 || n > math.MaxUint64>>shift {
		return 0, fmt.Errorf("%q is not a positive size such as 512M or 2G", s)
	}

	return n << shift, nil
}

func checkString(v any) error {
	if !isScalar(v) {
		return fmt.Errorf("%v (%T) is not a string", v, v)
	}

	return nil
}

func checkStringSlice(v any) error {
	switch t := v.(type) {
	case string, []string:
		return nil
	case []any:
		return checkItems(t)
	default:
		return fmt.Errorf("%v (%T) is not a list of strings", v, v)
	}
}

func checkItems(items []any) error {
	for i, e := range items {
		if !isScalar(e) {
			return fmt.Errorf("item #%d %v (%T) is not a string", i, e, e)
		}
	}

	return nil
}

// isScalar reports whether Viper can read v as a string.
func isScalar(v any) bool {
	switch v.(type) {
	case string, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return true
	default:
		return false
	}
}

// Reset is used mainly for testing purposes, in order to clean up the Viper
// instance.
func Reset() {
	mutex.Lock()
	defer mutex.Unlock()
	viper.Reset()
}
