/*
 * Copyright 2023 The Gremlins Authors
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

package diff

import (
	"errors"
	"reflect"
	"testing"

	"github.com/OpenRAE/go-rae-gremlins/internal/configuration"
)

func TestNewWithCmd(t *testing.T) {
	t.Run("must return nil on empty flag", testNewWithCmdEmptyRef)
	t.Run("must return error", testNewWithCmdGitFailure)
	t.Run("must return diff error", testNewWithCmdInvalidDiff)
	t.Run("must return changes", testNewWithCmdChanges)
}

func testNewWithCmdEmptyRef(t *testing.T) {
	setDiffRef(t, "")

	m := &mock{}

	d, err := NewWithCmd(m.call)

	if err != nil {
		t.Errorf("NewWithCmd() with empty diff ref error = %v, want nil", err)
	}
	if d != nil {
		t.Errorf("NewWithCmd() with empty diff ref = %#v, want nil Diff", d)
	}
	if m.calls != 0 {
		t.Errorf("NewWithCmd() with empty diff ref made %d git calls, want 0", m.calls)
	}
}

func testNewWithCmdGitFailure(t *testing.T) {
	setDiffRef(t, "test")

	m := &mock{
		outputErr: errors.New("test"),
	}

	_, err := NewWithCmd(m.call)
	if err == nil {
		t.Error("NewWithCmd() with a failing git command error = nil, want an error")
	}

	if m.calls != 1 {
		t.Fatalf("NewWithCmd() made %d git calls, want 1", m.calls)
	}

	expectedArgs := []string{"diff", "--merge-base", "test"}

	if m.callName != "git" || !reflect.DeepEqual(m.callArgs, expectedArgs) {
		t.Errorf("NewWithCmd() ran %q %q, want %q %q", m.callName, m.callArgs, "git", expectedArgs)
	}
}

func testNewWithCmdInvalidDiff(t *testing.T) {
	setDiffRef(t, "test")

	m := &mock{
		output: []byte(testErrDiff),
	}

	_, err := NewWithCmd(m.call)
	if err == nil {
		t.Error("NewWithCmd() with an invalid diff error = nil, want an error")
	}
}

func testNewWithCmdChanges(t *testing.T) {
	setDiffRef(t, "test")

	m := &mock{
		output: []byte(testDiff),
	}

	expected := Diff{
		"test/test": {{StartLine: 44, EndLine: 44}},
	}

	result, err := NewWithCmd(m.call)

	if err != nil {
		t.Errorf("NewWithCmd() error = %v, want nil", err)
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("NewWithCmd() = %v, want %v", result, expected)
	}
}

// setDiffRef registers the configuration reset before it sets the diff
// reference, so the case restores the shared configuration when it ends.
func setDiffRef(t *testing.T, ref string) {
	t.Helper()
	configuration.Reset()
	t.Cleanup(configuration.Reset)
	configuration.Set(configuration.UnleashDiffRef, ref)
}

type mock struct {
	calls     int
	callName  string
	callArgs  []string
	output    []byte
	outputErr error
}

func (m *mock) call(name string, args ...string) execCmd {
	m.calls++
	m.callName = name
	m.callArgs = args

	return m
}

func (m *mock) CombinedOutput() ([]byte, error) {
	return m.output, m.outputErr
}

const (
	testDiff = `
diff --git a/test/test b/test/test
index 54051bc..b92c425 100644
--- a/test/test
+++ b/test/test
@@ -41,6 +41,7 @@ const (
 	test = "test"
 	test = "test"
 	test = "test"
+	test = "test"
 	test = "test"
 	test = "test"
 )
`
	testErrDiff = `
diff --git a/test/test b/test/test
index 54051bc..b92c425 100644
--- a/test/test
+++ b/test/test
@@ -41,7 +41,7 @@ const (
 	test = "test"
+	test = "test"
 	test = "test"
 )
`
)
