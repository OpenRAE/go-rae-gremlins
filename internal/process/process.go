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

// Run starts cmd in a new process group and waits for it to complete.
// When the context of cmd is done, Run kills the process group. After cmd
// exits, Run kills any process that remains in the group. On Windows only
// the direct process is killed.
//
// Run waits at most five seconds for output pipes after cmd exits or its
// context is done, so a descendant that holds a pipe open cannot block it.
//
// cmd must come from exec.CommandContext. Run sets cmd.SysProcAttr,
// cmd.Cancel, and cmd.WaitDelay. It returns the error of cmd.Run, joined with
// any error of the kill after the exit.
func Run(cmd *exec.Cmd) error {
	setProcessGroup(cmd)
	cmd.Cancel = func() error { return killProcessGroup(cmd) }
	cmd.WaitDelay = 5 * time.Second
	err := cmd.Run()
	if cmd.Process == nil {
		return err
	}
	if killErr := killProcessGroup(cmd); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
		return errors.Join(err, fmt.Errorf("stop the remaining processes of %s: %w", cmd.Path, killErr))
	}

	return err
}
