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
// dir is the directory that the engine scans. The Diff file names are
// slash-separated and relative to dir, and the Diff holds no change outside dir.
func New(dir string) (Diff, error) {
	return NewWithCmd(dir, exec.Command)
}

type execCmd interface {
	CombinedOutput() ([]byte, error)
}

// NewWithCmd creates a new Diff by parsing git diff output using a custom command executor.
// This is useful for testing. dir has the same meaning as in New.
func NewWithCmd[T execCmd](dir string, cmdContext func(name string, args ...string) T) (Diff, error) {
	diffRef := configuration.Get[string](configuration.UnleashDiffRef)
	if diffRef == "" {
		return nil, nil
	}

	log.Infoln("Gathering files diff...")

	cmd := cmdContext("git", diffArgs(dir, diffRef)...)

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

// diffArgs returns the git arguments that compare the merge base of ref with
// the working tree of dir.
//
// Git writes the paths of a plain diff relative to the repository root.
// --relative in dir makes the paths relative to dir, as the engine names the
// files, and omits the changes outside dir. The color, external diff, textconv,
// and prefix options keep the user Git configuration from changing the patch
// that the parser reads. --end-of-options and the final -- make Git read ref
// only as a revision, never as an option or a path.
func diffArgs(dir, ref string) []string {
	return []string{
		"-C", dir, "diff",
		"--relative", "--no-color", "--no-ext-diff", "--no-textconv",
		"--src-prefix=a/", "--dst-prefix=b/",
		"--merge-base", "--end-of-options", ref, "--",
	}
}
