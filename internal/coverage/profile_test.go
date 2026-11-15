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

package coverage_test

import (
	"go/token"
	"testing"

	"github.com/OpenRAE/go-rae-gremlins/internal/coverage"
)

const testFilename = "test"

func TestIsCovered(t *testing.T) {
	testCases := []struct {
		name        string
		proFilename string
		posFilename string
		proStartL   int
		proEndL     int
		proStartC   int
		proEndC     int
		posL        int
		posC        int
		expected    bool
	}{
		// Single line coverage
		{
			name:        "true when single line, matches first column",
			proFilename: testFilename,
			proStartL:   10,
			proEndL:     10,
			proStartC:   10,
			proEndC:     10,
			posFilename: testFilename,
			posL:        10,
			posC:        10,
			expected:    true,
		},
		{
			name:        "true when single line, matches last column",
			proFilename: testFilename,
			proStartL:   10,
			proEndL:     10,
			proStartC:   10,
			proEndC:     11,
			posFilename: testFilename,
			posL:        10,
			posC:        11,
			expected:    true,
		},
		{
			name:        "true when single line, matches column lies in between",
			proFilename: testFilename,
			proStartL:   10,
			proEndL:     10,
			proStartC:   10,
			proEndC:     12,
			posFilename: testFilename,
			posL:        10,
			posC:        11,
			expected:    true,
		},
		{
			name:        "false when single line and column lies before",
			proFilename: testFilename,
			proStartL:   10,
			proEndL:     10,
			proStartC:   10,
			proEndC:     12,
			posFilename: testFilename,
			posL:        10,
			posC:        9,
			expected:    false,
		},
		{
			name:        "false when single line and column lies after",
			proFilename: testFilename,
			proStartL:   10,
			proEndL:     10,
			proStartC:   10,
			proEndC:     12,
			posFilename: testFilename,
			posL:        10,
			posC:        13,
			expected:    false,
		},
		// Multi line - in between
		{
			name:        "true when multi line and line lies in between (all cols covered)",
			proFilename: testFilename,
			proStartL:   10,
			proEndL:     12,
			proStartC:   10,
			proEndC:     10,
			posFilename: testFilename,
			posL:        11,
			posC:        1,
			expected:    true,
		},
		// Multi line - first line
		{
			name:        "true when covered on first line and col matches first col",
			proFilename: testFilename,
			proStartL:   10,
			proEndL:     11,
			proStartC:   10,
			proEndC:     11,
			posFilename: testFilename,
			posL:        10,
			posC:        10,
			expected:    true,
		},
		{
			name:        "true when covered on first line and col is covered to end of line",
			proFilename: testFilename,
			proStartL:   10,
			proEndL:     11,
			proStartC:   10,
			proEndC:     10,
			posFilename: testFilename,
			posL:        10,
			posC:        200,
			expected:    true,
		},
		{
			name:        "false when covered on first line and col lies before col",
			proFilename: testFilename,
			proStartL:   10,
			proEndL:     11,
			proStartC:   10,
			proEndC:     10,
			posFilename: testFilename,
			posL:        10,
			posC:        9,
			expected:    false,
		},
		// Multi line - last line
		{
			name:        "true when covered on last line and col matches last col",
			proFilename: testFilename,
			proStartL:   10,
			proEndL:     11,
			proStartC:   10,
			proEndC:     11,
			posFilename: testFilename,
			posL:        11,
			posC:        11,
			expected:    true,
		},
		{
			name:        "true when covered on last line and col lies before last col (ignores first col)",
			proFilename: testFilename,
			proStartL:   10,
			proEndL:     11,
			proStartC:   10,
			proEndC:     11,
			posFilename: testFilename,
			posL:        11,
			posC:        9,
			expected:    true,
		},
		{
			name:        "false when covered on last line and col lies after last col",
			proFilename: testFilename,
			proStartL:   10,
			proEndL:     11,
			proStartC:   10,
			proEndC:     11,
			posFilename: testFilename,
			posL:        11,
			posC:        12,
			expected:    false,
		},
	}

	for _, tc := range testCases {
		tCase := tc
		t.Run(tCase.name, func(t *testing.T) {
			profile := coverage.Profile{
				tCase.proFilename: {
					{
						StartLine: tCase.proStartL,
						StartCol:  tCase.proStartC,
						EndLine:   tCase.proEndL,
						EndCol:    tCase.proEndC,
					},
				},
			}

			position := token.Position{
				Filename: tCase.posFilename,
				Offset:   100,
				Line:     tCase.posL,
				Column:   tCase.posC,
			}

			got := profile.IsCovered(position)

			if got != tCase.expected {
				t.Errorf("IsCovered(%v) = %v, want %v", position, got, tCase.expected)
			}
		})
	}
}

func TestFeatureProfileIsCoveredLookup(t *testing.T) {
	position := token.Position{Filename: testFilename, Line: 20, Column: 5}
	before := coverage.Block{StartLine: 10, StartCol: 1, EndLine: 12, EndCol: 1}
	after := coverage.Block{StartLine: 30, StartCol: 1, EndLine: 32, EndCol: 1}
	matching := coverage.Block{StartLine: 20, StartCol: 1, EndLine: 20, EndCol: 9}
	testCases := []struct {
		profile coverage.Profile
		name    string
		want    bool
	}{
		{name: "file not in the profile", profile: coverage.Profile{"other": {matching}}, want: false},
		{name: "later block matches", profile: coverage.Profile{testFilename: {before, after, matching}}, want: true},
		{name: "no block matches", profile: coverage.Profile{testFilename: {before, after}}, want: false},
		{name: "file has no block", profile: coverage.Profile{testFilename: nil}, want: false},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.profile.IsCovered(position); got != tc.want {
				t.Errorf("IsCovered(%v) = %v, want %v", position, got, tc.want)
			}
		})
	}
}

func TestFeatureProfileIsDirCovered(t *testing.T) {
	block := []coverage.Block{{StartLine: 1, StartCol: 1, EndLine: 2, EndCol: 1}}
	profile := coverage.Profile{
		"root.go":         block,
		"pkg/a.go":        block,
		"pkg/sub/deep.go": block,
		"empty/a.go":      nil,
	}
	testCases := []struct {
		name string
		dir  string
		want bool
	}{
		{name: "module root with a block", dir: ".", want: true},
		{name: "package with a block", dir: "pkg", want: true},
		{name: "nested package with a block", dir: "pkg/sub", want: true},
		{name: "package with a file and no block", dir: "empty", want: false},
		{name: "package not in the profile", dir: "other", want: false},
		{name: "parent directory of a package with a block", dir: "pkg/..", want: false},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := profile.IsDirCovered(tc.dir); got != tc.want {
				t.Errorf("IsDirCovered(%q) = %v, want %v", tc.dir, got, tc.want)
			}
		})
	}
}
