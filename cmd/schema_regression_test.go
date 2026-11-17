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

// The tests in this file use only the APIs that the base revision also has,
// so that the regression proof can run them against base production code.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/spf13/viper"

	"github.com/OpenRAE/go-rae-gremlins/internal/configuration"
)

const (
	schemaPath            = "../docs/docs/schema/configuration.json"
	configurationPagePath = "../docs/docs/usage/configuration.md"
)

// JSON Schema type names.
const (
	schemaBoolean = "boolean"
	schemaInteger = "integer"
	schemaNumber  = "number"
	schemaString  = stringType
	schemaArray   = "array"
)

// schemaNode is the part of a JSON Schema definition that the contract
// tests read.
type schemaNode struct {
	Default    any                   `json:"default"`
	Properties map[string]schemaNode `json:"properties"`
	Minimum    *float64              `json:"minimum"`
	Maximum    *float64              `json:"maximum"`
	Type       string                `json:"type"`
	Required   []string              `json:"required"`
}

// TestRegressionConfigurationSchemaDefinesBoundKeys checks that the schema
// defines each configuration key that the commands bind, at its runtime
// location, and no other key.
func TestRegressionConfigurationSchemaDefinesBoundKeys(t *testing.T) {
	bound := boundKeys(t)
	defined := sortedKeys(schemaLeaves(readSchema(t)))

	if diff := cmp.Diff(bound, defined); diff != "" {
		t.Errorf("%s keys mismatch (-bound +schema):\n%s", schemaPath, diff)
	}
}

// TestRegressionConfigurationSchemaRequiresNoSetting checks that the schema
// requires no setting, because the runtime uses the default of an omitted key.
func TestRegressionConfigurationSchemaRequiresNoSetting(t *testing.T) {
	for k, req := range requiredLists("", readSchema(t)) {
		t.Errorf("%s object %q requires %v, want no required setting", schemaPath, k, req)
	}
}

// TestRegressionConfigurationReferenceExampleMatchesSchema checks that the
// complete reference example on the configuration page sets each schema key
// to its schema default, and that configuration.Validate accepts it.
func TestRegressionConfigurationReferenceExampleMatchesSchema(t *testing.T) {
	leaves := schemaLeaves(readSchema(t))
	initIsolatedConfiguration(t, referenceExample(t))

	if err := configuration.Validate(); err != nil {
		t.Errorf("configuration.Validate() with the reference example error = %v, want nil", err)
	}
	keys := viper.AllKeys()
	slices.Sort(keys)
	if diff := cmp.Diff(sortedKeys(leaves), keys); diff != "" {
		t.Errorf("reference example keys mismatch (-schema +example):\n%s", diff)
	}

	want := make(map[string]any)
	got := make(map[string]any)
	for k, n := range leaves {
		want[k] = schemaDefault(t, k, n)
		got[k] = typedValue(t, k, n.Type)
	}
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("reference example values mismatch (-schema defaults +example):\n%s", diff)
	}
}

// boundKeys returns the sorted configuration keys that the root and unleash
// commands bind to Viper.
func boundKeys(t *testing.T) []string {
	t.Helper()
	configuration.Reset()
	t.Cleanup(configuration.Reset)
	if _, err := newRootCmd(context.Background(), "1.2.3"); err != nil {
		t.Fatalf("newRootCmd() error = %v, want nil", err)
	}
	keys := viper.AllKeys()
	slices.Sort(keys)

	return keys
}

func sortedKeys(m map[string]schemaNode) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	return keys
}

func readSchema(t *testing.T) schemaNode {
	t.Helper()
	b, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read %s: %v", schemaPath, err)
	}
	var n schemaNode
	if err := json.Unmarshal(b, &n); err != nil {
		t.Fatalf("decode %s: %v", schemaPath, err)
	}

	return n
}

// schemaLeaves returns the definitions of the schema that have no
// properties, keyed by their dotted configuration key.
func schemaLeaves(root schemaNode) map[string]schemaNode {
	leaves := make(map[string]schemaNode)
	var walk func(prefix string, n schemaNode)
	walk = func(prefix string, n schemaNode) {
		for name, child := range n.Properties {
			key := prefix + name
			if len(child.Properties) == 0 {
				leaves[key] = child

				continue
			}
			walk(key+".", child)
		}
	}
	walk("", root)

	return leaves
}

// requiredLists returns the non-empty required lists of the schema objects,
// keyed by the dotted object key.
func requiredLists(key string, n schemaNode) map[string][]string {
	lists := make(map[string][]string)
	if len(n.Required) > 0 {
		lists[key] = n.Required
	}
	for name, child := range n.Properties {
		childKey := name
		if key != "" {
			childKey = key + "." + name
		}
		for k, v := range requiredLists(childKey, child) {
			lists[k] = v
		}
	}

	return lists
}

// schemaDefault returns the default of a schema leaf as the Go value that
// configuration.Get returns for its type.
func schemaDefault(t *testing.T, key string, n schemaNode) any {
	t.Helper()
	var ok bool
	var v any
	switch n.Type {
	case schemaBoolean:
		v, ok = n.Default.(bool)
	case schemaInteger:
		var f float64
		f, ok = n.Default.(float64)
		v = int(f)
	case schemaNumber:
		v, ok = n.Default.(float64)
	case schemaString:
		v, ok = n.Default.(string)
	case schemaArray:
		var items []any
		items, ok = n.Default.([]any)
		v, ok = stringItems(items, ok)
	}
	if !ok {
		t.Fatalf("%s %q default %v (%T) does not match the type %q", schemaPath, key, n.Default, n.Default, n.Type)
	}

	return v
}

// stringItems returns items as strings, and false when ok is false or an
// item is not a string.
func stringItems(items []any, ok bool) ([]string, bool) {
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(items))
	for _, e := range items {
		s, isString := e.(string)
		if !isString {
			return nil, false
		}
		out = append(out, s)
	}

	return out, true
}

// typedValue reads a configuration key with the Go type of a JSON Schema type.
func typedValue(t *testing.T, key, schemaType string) any {
	t.Helper()
	switch schemaType {
	case schemaBoolean:
		return configuration.Get[bool](key)
	case schemaInteger:
		return configuration.Get[int](key)
	case schemaNumber:
		return configuration.Get[float64](key)
	case schemaString:
		return configuration.Get[string](key)
	case schemaArray:
		return configuration.Get[[]string](key)
	default:
		t.Fatalf("%s %q has the unsupported type %q", schemaPath, key, schemaType)

		return nil
	}
}

// referenceExample returns the YAML block of the Reference section of the
// configuration page.
func referenceExample(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(configurationPagePath)
	if err != nil {
		t.Fatalf("read %s: %v", configurationPagePath, err)
	}
	_, section, ok := strings.Cut(string(b), "\n### Reference\n")
	if !ok {
		t.Fatalf("%s has no Reference section", configurationPagePath)
	}
	_, block, ok := strings.Cut(section, "```yaml\n")
	if !ok {
		t.Fatalf("%s Reference section has no YAML block", configurationPagePath)
	}
	block, _, ok = strings.Cut(block, "\n```")
	if !ok {
		t.Fatalf("%s Reference YAML block has no end", configurationPagePath)
	}

	return block
}

// initIsolatedConfiguration resets Viper, clears inherited GREMLINS_* values,
// and loads only the configuration content from a private temporary file.
func initIsolatedConfiguration(t *testing.T, content string) {
	t.Helper()
	configuration.Reset()
	t.Cleanup(configuration.Reset)
	for _, e := range os.Environ() {
		name, _, _ := strings.Cut(e, "=")
		if strings.HasPrefix(name, "GREMLINS_") {
			t.Setenv(name, "")
			if err := os.Unsetenv(name); err != nil {
				t.Fatalf("unset %s: %v", name, err)
			}
		}
	}
	path := filepath.Join(t.TempDir(), ".gremlins.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	if err := configuration.Init([]string{path}); err != nil {
		t.Fatalf("configuration.Init(%q) error = %v, want nil", path, err)
	}
}
