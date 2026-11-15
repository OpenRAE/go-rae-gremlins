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

// Package process runs external commands together with the processes that
// they start.
package process

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// Run starts cmd with the processes that it starts in one process tree and
// waits for cmd to complete. When the context of cmd is done, Run stops the
// complete tree. After cmd exits, Run stops any process that remains in the
// tree. On Unix the tree is a new process group. On Windows it is a new job
// object, and cmd starts suspended until it is in the job.
//
// Run waits at most five seconds for output pipes after cmd exits or its
// context is done, so a descendant that holds a pipe open cannot block it.
//
// cmd must come from exec.CommandContext. Run sets cmd.SysProcAttr,
// cmd.Cancel, and cmd.WaitDelay. It returns the error of cmd.Run, joined with
// any error of the tree control.
func Run(cmd *exec.Cmd) (err error) {
	tree, err := newProcessTree(cmd)
	if err != nil {
		return fmt.Errorf("prepare the process tree of %s: %w", cmd.Path, err)
	}
	defer func() {
		if closeErr := tree.close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("release the process tree of %s: %w", cmd.Path, closeErr))
		}
	}()
	cmd.Cancel = tree.kill
	cmd.WaitDelay = 5 * time.Second
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := tree.start(); err != nil {
		return stopOutsideTree(cmd, fmt.Errorf("start the process tree of %s: %w", cmd.Path, err))
	}
	err = cmd.Wait()
	if killErr := tree.kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
		return errors.Join(err, fmt.Errorf("stop the remaining processes of %s: %w", cmd.Path, killErr))
	}

	return err
}

// stopOutsideTree kills cmd, which must not run outside its process tree,
// and waits for it. It returns cause joined with any kill or wait error. The
// exit of the killed process is expected, so it is not an error; it must not
// make cause look like a test result.
func stopOutsideTree(cmd *exec.Cmd, cause error) error {
	if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		cause = errors.Join(cause, fmt.Errorf("kill %s: %w", cmd.Path, err))
	}
	var exitErr *exec.ExitError
	if err := cmd.Wait(); err != nil && !errors.As(err, &exitErr) {
		cause = errors.Join(cause, fmt.Errorf("wait for %s: %w", cmd.Path, err))
	}

	return cause
}
