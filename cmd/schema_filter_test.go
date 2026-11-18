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
	"os"
	"regexp"
	"testing"

	"github.com/OpenRAE/go-rae-gremlins/internal/report"
)

// patternNode is the part of a JSON Schema definition that has the
// properties and the pattern of a definition.
type patternNode struct {
	Properties map[string]patternNode `json:"properties"`
	Pattern    string                 `json:"pattern"`
}

// TestFeatureSchemaOutputStatusesMatchesFilter checks that the schema pattern
// of output-statuses accepts each letter that the console status filter
// accepts, and no other letter.
func TestFeatureSchemaOutputStatusesMatchesFilter(t *testing.T) {
	b, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read %s: %v", schemaPath, err)
	}
	var schema patternNode
	if err := json.Unmarshal(b, &schema); err != nil {
		t.Fatalf("decode %s: %v", schemaPath, err)
	}
	pattern, err := regexp.Compile(schema.Properties["unleash"].Properties["output-statuses"].Pattern)
	if err != nil {
		t.Fatalf("compile the output-statuses pattern of %s: %v", schemaPath, err)
	}

	for c := 'a'; c <= 'z'; c++ {
		letter := string(c)
		_, filterErr := report.ParseFilter(letter)
		if got, want := pattern.MatchString(letter), filterErr == nil; got != want {
			t.Errorf("schema pattern %q matches %q = %t, want %t (ParseFilter error = %v)",
				pattern, letter, got, want, filterErr)
		}
	}
}
