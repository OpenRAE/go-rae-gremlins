//go:build windows

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
	"fmt"
	"math"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// killedExitCode is the exit code of the processes that kill stops. It is
// the exit code that os.Process.Kill gives on Windows.
const killedExitCode = 1

// processTree is a job object that holds cmd and every process that it
// starts. Windows has no Unix process groups, so the job addresses the
// descendants.
type processTree struct {
	cmd *exec.Cmd
	// mu orders start against kill, which cmd.Cancel can call from another
	// goroutine.
	mu sync.Mutex
	// inJob is true when cmd is in the job.
	inJob bool
	// killed is true when kill stopped cmd before it was in the job.
	killed bool
	job    windows.Handle
}

// newProcessTree creates the job object and makes cmd start suspended in a
// new process group. The job stops its processes when its last handle
// closes, so the tree stops also when Gremlins exits.
func newProcessTree(cmd *exec.Cmd) (*processTree, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create a job object: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	// #nosec G103 - Audited: the Win32 call takes the address of info, which
	// stays alive on this stack and is read only during the call.
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		return nil, errors.Join(fmt.Errorf("set the job object limits: %w", err), windows.CloseHandle(job))
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_SUSPENDED,
	}

	return &processTree{cmd: cmd, job: job}, nil
}

// start puts the suspended cmd in the job and then resumes it. A process
// that cmd starts is then in the job from its start.
func (t *processTree) start() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.killed {
		return nil
	}
	if t.cmd.Process.Pid < 0 || t.cmd.Process.Pid > math.MaxUint32 {
		return fmt.Errorf("process ID %d is not a Windows process ID", t.cmd.Process.Pid)
	}
	pid := uint32(t.cmd.Process.Pid)
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return fmt.Errorf("open the process: %w", err)
	}
	err = windows.AssignProcessToJobObject(t.job, process)
	if closeErr := windows.CloseHandle(process); closeErr != nil {
		err = errors.Join(err, fmt.Errorf("close the process handle: %w", closeErr))
	}
	if err != nil {
		return fmt.Errorf("assign the process to the job object: %w", err)
	}
	t.inJob = true

	return resumeProcess(pid)
}

// kill stops every process in the job. Before cmd is in the job, kill stops
// cmd only, and start does not resume it.
func (t *processTree) kill() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.inJob {
		t.killed = true

		return t.cmd.Process.Kill()
	}

	return windows.TerminateJobObject(t.job, killedExitCode)
}

// close releases the job object. Any process that remains in the job stops.
func (t *processTree) close() error {
	return windows.CloseHandle(t.job)
}

// resumeProcess resumes the threads of the suspended process pid. A process
// that starts suspended has only its main thread.
func resumeProcess(pid uint32) (err error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return fmt.Errorf("list the threads: %w", err)
	}
	defer func() {
		if closeErr := windows.CloseHandle(snapshot); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close the thread list: %w", closeErr))
		}
	}()
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	resumed := false
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != pid {
			continue
		}
		if err := resumeThread(entry.ThreadID); err != nil {
			return err
		}
		resumed = true
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return fmt.Errorf("list the threads: %w", err)
	}
	if !resumed {
		return errors.New("find the main thread of the process")
	}

	return nil
}

// resumeThread resumes the suspended thread id.
func resumeThread(id uint32) error {
	thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, id)
	if err != nil {
		return fmt.Errorf("open the thread: %w", err)
	}
	_, err = windows.ResumeThread(thread)
	if closeErr := windows.CloseHandle(thread); closeErr != nil {
		err = errors.Join(err, fmt.Errorf("close the thread handle: %w", closeErr))
	}
	if err != nil {
		return fmt.Errorf("resume the thread: %w", err)
	}

	return nil
}
