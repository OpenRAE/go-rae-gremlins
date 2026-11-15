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

package cmd

import (
	"context"
	"testing"
)

// Flag types and default values that the command tests compare.
const (
	boolType   = "bool"
	intType    = "int"
	stringType = "string"
	trueValue  = "true"
	falseValue = "false"
)

func TestGremlins(t *testing.T) {
	c, err := newRootCmd(context.Background(), "1.2.3")
	if err != nil {
		t.Fatalf("newRootCmd() error = %v, want nil", err)
	}
	cmd := c.cmd
	// Explicit arguments stop cobra from reading the go test flags in os.Args.
	cmd.SetArgs([]string{"help"})
	if err := c.execute(); err != nil {
		t.Fatalf("execute() with args [help] error = %v, want nil", err)
	}

	if cmd.Version != "1.2.3" {
		t.Errorf("newRootCmd().Version = %q, want %q", cmd.Version, "1.2.3")
	}

	cfgFile := cmd.Flag("config")
	if cfgFile == nil {
		t.Fatal(`Flag("config") = nil, want a registered flag`)
	}
	if cfgFile.Value.Type() != stringType {
		t.Errorf("flag \"config\" type = %q, want %q", cfgFile.Value.Type(), stringType)
	}
	if cfgFile.DefValue != "" {
		t.Errorf("flag \"config\" default = %q, want empty", cfgFile.DefValue)
	}

	silentFlag := cmd.Flag("silent")
	if silentFlag == nil {
		t.Fatal(`Flag("silent") = nil, want a registered flag`)
	}
	if silentFlag.Value.Type() != boolType {
		t.Errorf("flag \"silent\" type = %q, want %q", silentFlag.Value.Type(), boolType)
	}
	if silentFlag.DefValue != falseValue {
		t.Errorf("flag \"silent\" default = %q, want %q", silentFlag.DefValue, falseValue)
	}
}

func TestExecute(t *testing.T) {
	t.Run("should not fail", func(t *testing.T) {
		err := Execute(context.Background(), "1.2.3")
		if err != nil {
			t.Errorf("Execute(version %q) error = %v, want nil", "1.2.3", err)
		}
	})

	t.Run("should fail if version is not set", func(t *testing.T) {
		err := Execute(context.Background(), "")
		if err == nil {
			t.Error("Execute(empty version) error = nil, want an error")
		}

	})
}
