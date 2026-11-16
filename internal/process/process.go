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
// they start, and limits the memory of these process trees.
package process

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync/atomic"
	"time"
)

// sampleInterval is the time between two memory samples of a process tree.
// A tree can pass its limit by what it allocates in one interval.
const sampleInterval = 50 * time.Millisecond

// Usage is the memory use of a process tree that RunBounded ran.
type Usage struct {
	// Peak is the peak memory in bytes of the largest process in the tree:
	// the resident set on Unix, and the committed memory on Windows.
	Peak uint64
	// OverLimit is true when the tree used more memory than its limit, so
	// RunBounded stopped it.
	OverLimit bool
}

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
func Run(cmd *exec.Cmd) error {
	_, err := RunBounded(cmd, 0)

	return err
}

// RunBounded runs cmd like Run and returns the memory use of the tree. When
// memoryLimit is not zero, a watchdog samples the total memory of the tree
// every sampleInterval: the resident memory of its processes on Unix, and
// the peak committed memory of the job on Windows. When the total passes
// memoryLimit, the watchdog stops the tree and Usage.OverLimit is true.
// When the watchdog cannot read the memory of a running tree, it stops the
// tree and RunBounded returns the error.
func RunBounded(cmd *exec.Cmd, memoryLimit uint64) (usage Usage, err error) {
	if memoryLimit != 0 {
		if err := memorySupported(); err != nil {
			return Usage{}, fmt.Errorf("limit the memory of %s: %w", cmd.Path, err)
		}
	}
	tree, err := newProcessTree(cmd)
	if err != nil {
		return Usage{}, fmt.Errorf("prepare the process tree of %s: %w", cmd.Path, err)
	}
	defer func() {
		if closeErr := tree.close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("release the process tree of %s: %w", cmd.Path, closeErr))
		}
	}()
	cmd.Cancel = tree.kill
	cmd.WaitDelay = 5 * time.Second
	if err := cmd.Start(); err != nil {
		return Usage{}, err
	}
	if err := tree.start(); err != nil {
		return Usage{}, stopOutsideTree(cmd, fmt.Errorf("start the process tree of %s: %w", cmd.Path, err))
	}
	stop := make(chan struct{})
	watched := make(chan error, 1)
	var overLimit atomic.Bool
	if memoryLimit != 0 {
		go func() { watched <- watchMemory(tree, memoryLimit, stop, &overLimit) }()
	} else {
		watched <- nil
	}
	err = cmd.Wait()
	close(stop)
	if watchErr := <-watched; watchErr != nil {
		err = errors.Join(err, fmt.Errorf("watch the memory of %s: %w", cmd.Path, watchErr))
	}
	if killErr := tree.kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
		return Usage{}, errors.Join(err, fmt.Errorf("stop the remaining processes of %s: %w", cmd.Path, killErr))
	}
	peak, peakErr := tree.peak()
	if peakErr != nil {
		return Usage{}, errors.Join(err, fmt.Errorf("read the peak memory of %s: %w", cmd.Path, peakErr))
	}

	return Usage{Peak: peak, OverLimit: overLimit.Load()}, err
}

// watchMemory samples the memory of tree until stop is closed. It stops the
// tree and sets overLimit when the memory passes limit. It stops the tree
// and returns the error when it cannot read the memory.
func watchMemory(tree *processTree, limit uint64, stop <-chan struct{}, overLimit *atomic.Bool) error {
	ticker := time.NewTicker(sampleInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return nil
		case <-ticker.C:
		}
		used, err := tree.memory()
		if err != nil {
			return errors.Join(err, ignoreDone(tree.kill()))
		}
		if used > limit {
			overLimit.Store(true)

			return ignoreDone(tree.kill())
		}
	}
}

// ignoreDone returns err, or nil when err is os.ErrProcessDone.
func ignoreDone(err error) error {
	if errors.Is(err, os.ErrProcessDone) {
		return nil
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
