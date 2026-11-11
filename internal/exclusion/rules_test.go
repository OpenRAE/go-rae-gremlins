/*
 * Copyright 2024 The Gremlins Authors
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

package exclusion

import (
	"testing"

	"github.com/OpenRAE/go-rae-gremlins/internal/configuration"
)

var testPath = []string{
	"something/test.go",
	"something/something.go",
	"internal/test.go",
}

func TestRules_IsFileExcluded(t *testing.T) {
	t.Run("must exclude files by regexp", func(t *testing.T) {
		ss := []any{"test", "internal"}
		configuration.Set(configuration.UnleashExcludeFiles, ss)

		rules, err := New()
		if err != nil || countTrue(testPath, rules.IsFileExcluded) != 2 {
			t.Error("must match 2 paths")
		}
	})

	t.Run("must return parsing error", func(t *testing.T) {
		ss := []any{"test", "internal[[["}
		configuration.Set(configuration.UnleashExcludeFiles, ss)

		rules, err := New()
		if err == nil || rules != nil {
			t.Error("must return error")
		}
	})

	t.Run("no rules", func(t *testing.T) {
		configuration.Set(configuration.UnleashExcludeFiles, []string(nil))

		rules, err := New()
		if err != nil || len(rules) != 0 {
			t.Error("must return empty rules")
		}

		if countTrue(testPath, rules.IsFileExcluded) != 0 {
			t.Error("must not match any")
		}
	})

}

func countTrue(ss []string, f func(s string) bool) int {
	count := 0

	for _, s := range ss {
		if f(s) {
			count++
		}
	}

	return count
}
