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

package flags

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	testCfgKey = "test.cfg"
	testUsage  = "test usage"
)

type unsupportedType int

type testCase struct {
	flag        Flag
	expectError bool
}

func TestSet(t *testing.T) {
	testCases := []testCase{
		{
			flag: Flag{
				Name:      "bool-flag-no-sh",
				CfgKey:    testCfgKey,
				Shorthand: "",
				DefaultV:  true,
				Usage:     testUsage,
			},
		},
		{
			flag: Flag{
				Name:      "bool-flag-sh",
				CfgKey:    testCfgKey,
				Shorthand: "t",
				DefaultV:  true,
				Usage:     testUsage,
			},
		},

		{
			flag: Flag{
				Name:      "string-flag-no-sh",
				CfgKey:    testCfgKey,
				Shorthand: "",
				DefaultV:  "test",
				Usage:     testUsage,
			},
		},
		{
			flag: Flag{
				Name:      "string-flag-sh",
				CfgKey:    testCfgKey,
				Shorthand: "t",
				DefaultV:  "test",
				Usage:     testUsage,
			},
		},
		{
			flag: Flag{
				Name:      "int-flag-no-sh",
				CfgKey:    testCfgKey,
				Shorthand: "",
				DefaultV:  0,
				Usage:     testUsage,
			},
		},
		{
			flag: Flag{
				Name:      "int-flag-sh",
				CfgKey:    testCfgKey,
				Shorthand: "t",
				DefaultV:  0,
				Usage:     testUsage,
			},
		},
		{
			flag: Flag{
				Name:      "float64-flag-no-sh",
				CfgKey:    testCfgKey,
				Shorthand: "",
				DefaultV:  float64(0),
				Usage:     testUsage,
			},
		},
		{
			flag: Flag{
				Name:      "float64-flag-sh",
				CfgKey:    testCfgKey,
				Shorthand: "t",
				DefaultV:  float64(0),
				Usage:     testUsage,
			},
		},
		{
			flag: Flag{
				Name:      "[]string-flag-no-sh",
				CfgKey:    testCfgKey,
				Shorthand: "",
				DefaultV:  []string{},
				Usage:     testUsage,
			},
		},
		{
			flag: Flag{
				Name:      "[]string-flag-sh",
				CfgKey:    testCfgKey,
				Shorthand: "t",
				DefaultV:  []string{},
				Usage:     testUsage,
			},
		},
		{
			flag: Flag{
				Name:      "not-supported-type",
				CfgKey:    testCfgKey,
				Shorthand: "t",
				DefaultV:  unsupportedType(0),
				Usage:     testUsage,
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.flag.Name, func(t *testing.T) {
			defer viper.Reset()

			cmd := &cobra.Command{}

			// #nosec G601 - We are in tests, we don't care
			err := Set(cmd, &tc.flag)
			checkFlag(t, "Set", err, cmd, tc)

			tc.flag.Name += "_persistent"
			// #nosec G601 - We are in tests, we don't care
			err = SetPersistent(cmd, &tc.flag)
			checkFlag(t, "SetPersistent", err, cmd, tc)
		})
	}
}

// checkFlag checks the result of op, the Set or SetPersistent call that
// registered tc.flag on cmd.
func checkFlag(t *testing.T, op string, err error, cmd *cobra.Command, tc testCase) {
	t.Helper()

	if tc.expectError && err == nil {
		t.Fatalf("%s(%q) error = nil, want an error", op, tc.flag.Name)
	}
	if !tc.expectError && err != nil {
		t.Fatalf("%s(%q) error = %v, want nil", op, tc.flag.Name, err)
	}
	if !tc.expectError {
		flag := cmd.Flag(tc.flag.Name)

		if flag == nil {
			t.Errorf("%s(%q): cmd.Flag(%q) = nil, want a registered flag", op, tc.flag.Name, tc.flag.Name)

			return
		}

		if tc.flag.Shorthand != flag.Shorthand {
			t.Errorf("%s(%q): shorthand = %q, want %q", op, tc.flag.Name, flag.Shorthand, tc.flag.Shorthand)
		}
	}
}
