//go:build unix

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

package process

import (
	"context"
	"os/exec"
	"testing"
)

func TestFeatureProcessTreePeakNeedsAnExitState(t *testing.T) {
	tree, err := newProcessTree(exec.CommandContext(context.Background(), "go", "version"))
	if err != nil {
		t.Fatalf("newProcessTree() = %v, want nil", err)
	}
	if got, err := tree.peak(); err == nil {
		t.Errorf("peak() of a command that did not run = %d, nil, want an error", got)
	}
}
