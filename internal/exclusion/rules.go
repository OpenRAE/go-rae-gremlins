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

// Package exclusion provides file exclusion rules based on regex patterns.
package exclusion

import (
	"fmt"
	"regexp"

	"github.com/OpenRAE/go-rae-gremlins/internal/configuration"
)

// Rules represents a collection of regex patterns for file exclusion.
type Rules []*regexp.Regexp

// New creates exclusion rules from the configuration.
func New() (Rules, error) {
	var rules Rules

	flagValues := configuration.Get[[]string](configuration.UnleashExcludeFiles)

	for i, s := range flagValues {
		r, err := regexp.Compile(s)
		if err != nil {
			return nil, fmt.Errorf("error in exclude-files param value #%d: %w", i, err)
		}

		rules = append(rules, r)
	}

	return rules, nil
}

// IsFileExcluded returns true if the given path matches any of the exclusion rules.
func (r Rules) IsFileExcluded(path string) bool {
	if len(r) == 0 {
		return false
	}

	for _, rule := range r {
		if rule.MatchString(path) {
			return true
		}
	}

	return false
}
