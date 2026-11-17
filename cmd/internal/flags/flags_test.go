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

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	testCfgKey  = "test.cfg"
	testUsage   = "test usage"
	testChanged = "changed"
)

type unsupportedType int

type testCase struct {
	flag        Flag
	want        any
	arg         string
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
			arg:  "false",
			want: false,
		},
		{
			flag: Flag{
				Name:      "bool-flag-sh",
				CfgKey:    testCfgKey,
				Shorthand: "t",
				DefaultV:  true,
				Usage:     testUsage,
			},
			arg:  "false",
			want: false,
		},

		{
			flag: Flag{
				Name:      "string-flag-no-sh",
				CfgKey:    testCfgKey,
				Shorthand: "",
				DefaultV:  "test",
				Usage:     testUsage,
			},
			arg:  testChanged,
			want: testChanged,
		},
		{
			flag: Flag{
				Name:      "string-flag-sh",
				CfgKey:    testCfgKey,
				Shorthand: "t",
				DefaultV:  "test",
				Usage:     testUsage,
			},
			arg:  testChanged,
			want: testChanged,
		},
		{
			flag: Flag{
				Name:      "int-flag-no-sh",
				CfgKey:    testCfgKey,
				Shorthand: "",
				DefaultV:  7,
				Usage:     testUsage,
			},
			arg:  "42",
			want: 42,
		},
		{
			flag: Flag{
				Name:      "int-flag-sh",
				CfgKey:    testCfgKey,
				Shorthand: "t",
				DefaultV:  7,
				Usage:     testUsage,
			},
			arg:  "42",
			want: 42,
		},
		{
			flag: Flag{
				Name:      "float64-flag-no-sh",
				CfgKey:    testCfgKey,
				Shorthand: "",
				DefaultV:  0.5,
				Usage:     testUsage,
			},
			arg:  "1.5",
			want: 1.5,
		},
		{
			flag: Flag{
				Name:      "float64-flag-sh",
				CfgKey:    testCfgKey,
				Shorthand: "t",
				DefaultV:  0.5,
				Usage:     testUsage,
			},
			arg:  "1.5",
			want: 1.5,
		},
		{
			flag: Flag{
				Name:      "[]string-flag-no-sh",
				CfgKey:    testCfgKey,
				Shorthand: "",
				DefaultV:  []string{"default"},
				Usage:     testUsage,
			},
			arg:  testChanged,
			want: []string{testChanged},
		},
		{
			flag: Flag{
				Name:      "[]string-flag-sh",
				CfgKey:    testCfgKey,
				Shorthand: "t",
				DefaultV:  []string{"default"},
				Usage:     testUsage,
			},
			arg:  testChanged,
			want: []string{testChanged},
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
			checkSetter(t, "Set", Set, tc)
			checkSetter(t, "SetPersistent", SetPersistent, tc)
		})
	}
}

// checkSetter registers tc.flag with set, the function named op, on a new
// command and a reset Viper, so a check cannot read the binding of an earlier check.
func checkSetter(t *testing.T, op string, set func(*cobra.Command, *Flag) error, tc testCase) {
	t.Helper()

	viper.Reset()
	t.Cleanup(viper.Reset)
	cmd := &cobra.Command{}
	flag := tc.flag

	err := set(cmd, &flag)
	checkFlag(t, op, err, cmd, tc)
}

// checkFlag checks the result of op, the Set or SetPersistent call that
// registered tc.flag on cmd.
func checkFlag(t *testing.T, op string, err error, cmd *cobra.Command, tc testCase) {
	t.Helper()

	if tc.expectError {
		if err == nil {
			t.Fatalf("%s(%q) error = nil, want an error", op, tc.flag.Name)
		}

		return
	}
	if err != nil {
		t.Fatalf("%s(%q) error = %v, want nil", op, tc.flag.Name, err)
	}

	flag := cmd.Flag(tc.flag.Name)
	if flag == nil {
		t.Fatalf("%s(%q): cmd.Flag(%q) = nil, want a registered flag", op, tc.flag.Name, tc.flag.Name)
	}
	if flag.Shorthand != tc.flag.Shorthand {
		t.Errorf("%s(%q): shorthand = %q, want %q", op, tc.flag.Name, flag.Shorthand, tc.flag.Shorthand)
	}

	if diff := cmp.Diff(tc.flag.DefaultV, configValue(tc.flag.CfgKey, tc.want)); diff != "" {
		t.Errorf("%s(%q): key %q before parse: default mismatch (-want +got):\n%s", op, tc.flag.Name, tc.flag.CfgKey, diff)
	}

	arg := "--" + tc.flag.Name + "=" + tc.arg
	if err := cmd.ParseFlags([]string{arg}); err != nil {
		t.Fatalf("%s(%q): ParseFlags(%s) error = %v, want nil", op, tc.flag.Name, arg, err)
	}
	if diff := cmp.Diff(tc.want, configValue(tc.flag.CfgKey, tc.want)); diff != "" {
		t.Errorf("%s(%q): key %q after %s: value mismatch (-want +got):\n%s", op, tc.flag.Name, tc.flag.CfgKey, arg, diff)
	}
}

// configValue reads key with the Viper getter for the type of like.
func configValue(key string, like any) any {
	switch like.(type) {
	case bool:
		return viper.GetBool(key)
	case int:
		return viper.GetInt(key)
	case float64:
		return viper.GetFloat64(key)
	case []string:
		return viper.GetStringSlice(key)
	default:
		return viper.GetString(key)
	}
}
