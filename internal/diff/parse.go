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
	"bytes"
	"fmt"
	"os/exec"

	"github.com/bluekeyes/go-gitdiff/gitdiff"

	"github.com/OpenRAE/go-rae-gremlins/internal/configuration"
	"github.com/OpenRAE/go-rae-gremlins/internal/log"
)

// New creates a new Diff by parsing git diff output using the default command executor.
func New() (Diff, error) {
	return NewWithCmd(exec.Command)
}

type execCmd interface {
	CombinedOutput() ([]byte, error)
}

// NewWithCmd creates a new Diff by parsing git diff output using a custom command executor.
// This is useful for testing.
func NewWithCmd[T execCmd](cmdContext func(name string, args ...string) T) (Diff, error) {
	diffRef := configuration.Get[string](configuration.UnleashDiffRef)
	if diffRef == "" {
		return nil, nil
	}

	log.Infoln("Gathering files diff...")

	cmd := cmdContext("git", "diff", "--merge-base", diffRef)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("an error occured while calling git diff: %w\n\n%s", err, out)
	}

	files, _, err := gitdiff.Parse(bytes.NewReader(out))
	if err != nil {
		return nil, fmt.Errorf("an error occured while parsing diff: %w", err)
	}

	return newDiff(files), nil
}
