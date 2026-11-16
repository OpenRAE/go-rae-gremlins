//go:build darwin

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
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"syscall"
	"unsafe"
)

// maxrssUnit is the unit of Rusage.Maxrss in bytes. Darwin reports bytes.
const maxrssUnit = 1

// The proc_info system call arguments of libproc, from sys/proc_info.h.
const (
	procInfoCallListPids = 1
	procInfoCallPidInfo  = 2
	procPgrpOnly         = 2
	procPidTaskInfo      = 4
	// maxGroupSize is the largest number of processes that treeResident
	// reads in one process group.
	maxGroupSize = 4096
)

// procTaskInfo is the proc_taskinfo structure of sys/proc_info.h.
type procTaskInfo struct {
	virtualSize  uint64
	residentSize uint64
	times        [4]uint64
	counters     [12]int32
}

// memorySupported returns nil. proc_info reports the memory of a tree.
func memorySupported() error {
	return nil
}

// treeResident returns the total resident memory in bytes of the processes
// in the process group that root leads. A process that exits during the
// read adds nothing.
func treeResident(root int) (uint64, error) {
	pids, err := groupPIDs(root)
	if err != nil {
		return 0, err
	}
	var total uint64
	for _, pid := range pids {
		info, running, err := taskInfo(pid)
		if err != nil {
			return 0, err
		}
		if running {
			total += info.residentSize
		}
	}

	return total, nil
}

// onlyZombies reports whether no process of the group that root leads has
// a task. An exited process that its parent has not waited for, a zombie,
// has no task.
func onlyZombies(root int) bool {
	pids, err := groupPIDs(root)
	if err != nil {
		return false
	}
	for _, pid := range pids {
		if _, running, err := taskInfo(pid); err != nil || running {
			return false
		}
	}

	return true
}

// groupPIDs returns the process IDs of the process group that root leads.
func groupPIDs(root int) ([]int32, error) {
	pids := make([]int32, maxGroupSize)
	// #nosec G103 - Audited: the kernel writes at most the given byte count to
	// pids, which stays alive during the call.
	n, err := procInfo(procInfoCallListPids, procPgrpOnly, root, unsafe.Pointer(&pids[0]), len(pids)*4)
	if err != nil {
		return nil, fmt.Errorf("list the process group %d: %w", root, err)
	}
	var group []int32
	for _, pid := range pids[:n/4] {
		if pid != 0 {
			group = append(group, pid)
		}
	}

	return group, nil
}

// taskInfo returns the task information of process pid. It returns false
// when the process has no task: it exited, or it is a zombie.
func taskInfo(pid int32) (procTaskInfo, bool, error) {
	var info procTaskInfo
	// #nosec G103 - Audited: the kernel writes at most the given byte count to
	// info, which stays alive during the call.
	size, err := procInfo(procInfoCallPidInfo, int(pid), procPidTaskInfo, unsafe.Pointer(&info), int(unsafe.Sizeof(info)))
	if errors.Is(err, syscall.ESRCH) {
		return info, false, nil
	}
	if err != nil {
		return info, false, fmt.Errorf("read the task of process %d: %w", pid, err)
	}

	return info, size == int(unsafe.Sizeof(info)), nil
}

// sysctlCount reads the unsigned integer sysctl value name, which the kernel
// stores in 4 or 8 bytes.
func sysctlCount(name string) (uint64, error) {
	mib, err := sysctlMIB(name)
	if err != nil {
		return 0, err
	}
	var value [8]byte
	size := uintptr(len(value))
	// #nosec G103 - Audited: the kernel writes at most size bytes to value,
	// which stays alive during the call.
	if err := sysctl(mib, unsafe.Pointer(&value[0]), &size, nil, 0); err != nil {
		return 0, err
	}
	switch size {
	case 4:
		return uint64(binary.LittleEndian.Uint32(value[:4])), nil
	case 8:
		return binary.LittleEndian.Uint64(value[:]), nil
	default:
		return 0, fmt.Errorf("%s has %d bytes, want 4 or 8", name, size)
	}
}

// sysctlMIB returns the numeric form of the sysctl name. The kernel returns
// it when name is written to the sysctl {0, 3}, as sysctlnametomib does.
func sysctlMIB(name string) ([]int32, error) {
	var mib [12]int32
	size := uintptr(len(mib) * 4)
	nameBytes := []byte(name)
	// #nosec G103 - Audited: the kernel reads len(name) bytes of nameBytes and
	// writes at most size bytes to mib. Both stay alive during the call.
	err := sysctl([]int32{0, 3}, unsafe.Pointer(&mib[0]), &size, unsafe.Pointer(&nameBytes[0]), uintptr(len(nameBytes)))
	if err != nil {
		return nil, fmt.Errorf("find the sysctl %s: %w", name, err)
	}

	return mib[:size/4], nil
}

// sysctl calls the __sysctl system call.
func sysctl(mib []int32, old unsafe.Pointer, oldSize *uintptr, value unsafe.Pointer, valueSize uintptr) error {
	// #nosec G103 - Audited: mib and oldSize stay alive during the call.
	_, _, errno := syscall.Syscall6(syscall.SYS___SYSCTL, uintptr(unsafe.Pointer(&mib[0])), uintptr(len(mib)),
		uintptr(old), uintptr(unsafe.Pointer(oldSize)), uintptr(value), valueSize)
	if errno != 0 {
		return errno
	}

	return nil
}

// procInfo calls the proc_info system call and returns its result in bytes.
func procInfo(call, pid, flavor int, buffer unsafe.Pointer, size int) (int, error) {
	r, _, errno := syscall.Syscall6(syscall.SYS_PROC_INFO, uintptr(call), uintptr(pid), uintptr(flavor), 0,
		uintptr(buffer), uintptr(size))
	if errno != 0 {
		return 0, errno
	}

	return int(r), nil
}

// AvailableMemory returns the memory in bytes that new processes can use:
// the free pages, the purgeable pages, and the file-backed pages that the
// system can reclaim.
func AvailableMemory() (uint64, error) {
	var pages uint64
	for _, name := range []string{"vm.page_free_count", "vm.page_purgeable_count", "vm.page_pageable_external_count"} {
		count, err := sysctlCount(name)
		if err != nil {
			return 0, fmt.Errorf("read %s: %w", name, err)
		}
		pages += count
	}
	size := os.Getpagesize()
	if size <= 0 {
		return 0, fmt.Errorf("the memory page size %d is not positive", size)
	}
	if pages > math.MaxUint64/uint64(size) {
		return 0, fmt.Errorf("the available page count %d overflows", pages)
	}

	return pages * uint64(size), nil
}
