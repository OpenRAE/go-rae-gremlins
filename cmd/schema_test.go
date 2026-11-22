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
	"encoding/json"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/OpenRAE/go-rae-gremlins/cmd/internal/flags"
	"github.com/OpenRAE/go-rae-gremlins/internal/configuration"
)

// setting is the JSON type and the JSON default of one configuration key.
type setting struct {
	Default any
	Type    string
}

// TestFeatureConfigurationSchemaMatchesFlagTypesAndDefaults checks that each
// schema key has the type and the default of the flag that binds it.
func TestFeatureConfigurationSchemaMatchesFlagTypesAndDefaults(t *testing.T) {
	fls := append(unleashFlags(), silentFlag())
	keys := make([]string, 0, len(fls))
	want := make(map[string]setting)
	for _, f := range fls {
		keys = append(keys, f.CfgKey)
		want[f.CfgKey] = setting{Type: jsonType(t, f), Default: jsonValue(t, f.DefaultV)}
	}
	slices.Sort(keys)
	if diff := cmp.Diff(boundKeys(t), keys); diff != "" {
		t.Fatalf("flag table keys mismatch (-bound +flag table):\n%s", diff)
	}

	got := make(map[string]setting)
	for k, n := range schemaLeaves(readSchema(t)) {
		got[k] = setting{Type: n.Type, Default: n.Default}
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("%s settings mismatch (-flags +schema):\n%s", schemaPath, diff)
	}
}

// TestFeatureConfigurationSchemaDeclaresValidateBounds checks that the schema
// declares the numeric bounds that configuration.Validate enforces.
func TestFeatureConfigurationSchemaDeclaresValidateBounds(t *testing.T) {
	zero, hundred := 0.0, 100.0
	testCases := []struct {
		minimum *float64
		maximum *float64
		key     string
	}{
		{key: configuration.UnleashWorkersKey, minimum: &zero},
		{key: configuration.UnleashTestCPUKey, minimum: &zero},
		{key: configuration.UnleashTimeoutCoefficientKey, minimum: &zero},
		{key: configuration.UnleashTimeoutSafetyMarginKey, minimum: &zero},
		{key: configuration.UnleashThresholdEfficacyKey, minimum: &zero, maximum: &hundred},
		{key: configuration.UnleashThresholdMCoverageKey, minimum: &zero, maximum: &hundred},
	}
	leaves := schemaLeaves(readSchema(t))
	for _, tc := range testCases {
		t.Run(tc.key, func(t *testing.T) {
			n, ok := leaves[tc.key]
			if !ok {
				t.Fatalf("%s has no definition for %q", schemaPath, tc.key)
			}
			if diff := cmp.Diff(tc.minimum, n.Minimum); diff != "" {
				t.Errorf("%q minimum mismatch (-want +schema):\n%s", tc.key, diff)
			}
			if diff := cmp.Diff(tc.maximum, n.Maximum); diff != "" {
				t.Errorf("%q maximum mismatch (-want +schema):\n%s", tc.key, diff)
			}
		})
	}
}

// jsonType returns the JSON Schema type of the default of f.
func jsonType(t *testing.T, f *flags.Flag) string {
	t.Helper()
	switch f.DefaultV.(type) {
	case bool:
		return schemaBoolean
	case int:
		return schemaInteger
	case float64:
		return schemaNumber
	case string:
		return schemaString
	case []string:
		return schemaArray
	default:
		t.Fatalf("flag %q default %v has the unsupported type %T", f.Name, f.DefaultV, f.DefaultV)

		return ""
	}
}

// jsonValue returns v as encoding/json decodes it into an interface value.
// A string list, also a nil list, becomes a JSON array.
func jsonValue(t *testing.T, v any) any {
	t.Helper()
	if items, ok := v.([]string); ok {
		out := make([]any, 0, len(items))
		for _, s := range items {
			out = append(out, s)
		}

		return out
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("encode %v: %v", v, err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("decode %s: %v", b, err)
	}

	return out
}
