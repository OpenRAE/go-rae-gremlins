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

package engine

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

// Arguments that the tests expect in the go test command line.
const (
	goTestArg       = "test"
	failFastArg     = "-failfast"
	allPackagesArg  = "./..."
	timeoutArg      = "-timeout"
	countArg        = "-count=1"
	expectedTimeout = "12s"
	examplePackage  = "example.com/my/package"
)

func TestGetTestArgs(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		buildTags       string
		timeout         time.Duration
		testCPU         int
		integrationMode bool
		pkg             string
		want            []string
	}{
		"should_not_include_tags_flag_when_build_tags_are_empty": {
			timeout: 10 * time.Second,
			pkg:     examplePackage,
			want:    []string{goTestArg, timeoutArg, expectedTimeout, failFastArg, countArg, examplePackage},
		},
		"should_include_tags_flag_when_build_tags_are_set": {
			buildTags: "tag1,tag2",
			timeout:   10 * time.Second,
			pkg:       examplePackage,
			want: []string{
				goTestArg, "-tags", "tag1,tag2", timeoutArg, expectedTimeout, failFastArg, countArg, examplePackage,
			},
		},
		"should_compute_timeout_as_two_seconds_plus_execution_time": {
			timeout: 30 * time.Second,
			pkg:     examplePackage,
			want:    []string{goTestArg, timeoutArg, "32s", failFastArg, countArg, examplePackage},
		},
		"should_not_include_cpu_flag_when_test_cpu_is_zero": {
			timeout: 10 * time.Second,
			testCPU: 0,
			pkg:     examplePackage,
			want:    []string{goTestArg, timeoutArg, expectedTimeout, failFastArg, countArg, examplePackage},
		},
		"should_include_cpu_flag_when_test_cpu_is_nonzero": {
			timeout: 10 * time.Second,
			testCPU: 4,
			pkg:     examplePackage,
			want:    []string{goTestArg, timeoutArg, expectedTimeout, failFastArg, "-cpu", "4", countArg, examplePackage},
		},
		"should_use_package_path_when_integration_mode_is_disabled": {
			timeout:         10 * time.Second,
			integrationMode: false,
			pkg:             examplePackage,
			want:            []string{goTestArg, timeoutArg, expectedTimeout, failFastArg, countArg, examplePackage},
		},
		"should_use_dot_dot_dot_path_when_integration_mode_is_enabled": {
			timeout:         10 * time.Second,
			integrationMode: true,
			pkg:             examplePackage,
			want:            []string{goTestArg, timeoutArg, expectedTimeout, failFastArg, "-p", "1", allPackagesArg},
		},
		"should_include_all_flags_when_all_options_are_configured": {
			buildTags:       "integration",
			timeout:         10 * time.Second,
			testCPU:         2,
			integrationMode: true,
			pkg:             examplePackage,
			want: []string{
				goTestArg, "-tags", "integration", timeoutArg, expectedTimeout, failFastArg,
				"-cpu", "2", "-p", "1", allPackagesArg,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			sut := &mutantExecutor{
				buildTags:       tc.buildTags,
				testCPU:         tc.testCPU,
				integrationMode: tc.integrationMode,
			}

			if diff := cmp.Diff(tc.want, sut.getTestArgs(tc.pkg, tc.timeout)); diff != "" {
				t.Errorf("getTestArgs() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
