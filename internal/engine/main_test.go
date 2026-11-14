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

package engine_test

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"
)

const (
	// treeRoleEnv makes the test binary run a process tree helper role
	// instead of the tests.
	treeRoleEnv    = "GO_TEST_TREE_ROLE"
	treeRoleParent = "parent"
	treeRoleChild  = "descendant"
	// treeHelperRun selects no test, so a child without a role does nothing.
	treeHelperRun   = "-test.run=^$"
	treeHelperLimit = 30 * time.Second
	treeWait        = 10 * time.Second
)

// TestMain runs the process tree helper role in treeRoleEnv instead of the
// tests.
func TestMain(m *testing.M) {
	if role := os.Getenv(treeRoleEnv); role != "" {
		os.Exit(runTreeRole(role)) // skipcq: RVV-A0003
	}
	os.Exit(m.Run()) // skipcq: RVV-A0003
}

// runTreeRole runs one role of the process tree helper and returns its exit
// code. The parent role starts one descendant that shares its output, as a
// go test binary does. Each role writes its first letter to descriptor 3 and
// then blocks for at most treeHelperLimit. Both roles hold descriptor 3 open,
// so the reader gets EOF only when the complete tree has exited.
func runTreeRole(role string) int {
	report := os.NewFile(3, "report")
	if role == treeRoleParent {
		// #nosec G204 G702 - The helper starts the test binary itself.
		cmd := exec.CommandContext(context.Background(), os.Args[0], treeHelperRun)
		cmd.Env = []string{treeRoleEnv + "=" + treeRoleChild}
		cmd.ExtraFiles = []*os.File{report}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			return 2
		}
	}
	if _, err := report.Write([]byte(role[:1])); err != nil {
		return 2
	}
	time.Sleep(treeHelperLimit)

	return 0
}
