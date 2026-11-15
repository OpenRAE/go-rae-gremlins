//go:build unix

/*
 * Copyright 2025 The Gremlins Authors
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
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// processTree is a new process group that cmd leads, so its descendants,
// for example the test binaries of go test, stop together with it.
type processTree struct {
	cmd *exec.Cmd
}

// newProcessTree makes cmd the leader of a new process group when it starts.
func newProcessTree(cmd *exec.Cmd) (*processTree, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	return &processTree{cmd: cmd}, nil
}

// start does nothing. The group exists from the start of cmd.
func (*processTree) start() error {
	return nil
}

// kill sends SIGKILL to every process in the group of cmd. It returns
// os.ErrProcessDone when no process remains in the group.
func (t *processTree) kill() error {
	// A negative PID addresses the process group.
	err := syscall.Kill(-t.cmd.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}

	return err
}

// memory returns the total resident memory in bytes of cmd and its
// descendants.
func (t *processTree) memory() (uint64, error) {
	return treeResident(t.cmd.Process.Pid)
}

// peak returns the largest resident set in bytes of one process in the
// group. The kernel reports the maximum of cmd and its waited descendants.
func (t *processTree) peak() (uint64, error) {
	if t.cmd.ProcessState == nil {
		return 0, errors.New("the process has no exit state")
	}
	usage, ok := t.cmd.ProcessState.SysUsage().(*syscall.Rusage)
	if !ok || usage.Maxrss < 0 {
		return 0, errors.New("the process has no resource usage")
	}

	return uint64(usage.Maxrss) * maxrssUnit, nil
}

// close does nothing. A process group holds no resource of Gremlins.
func (*processTree) close() error {
	return nil
}
