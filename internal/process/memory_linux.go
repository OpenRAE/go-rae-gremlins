//go:build linux

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
	"bufio"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// pageSize returns the memory page size in bytes.
func pageSize() uint64 {
	size := os.Getpagesize()
	if size <= 0 {
		panic("the memory page size is not positive")
	}

	return uint64(size)
}

// maxrssUnit is the unit of Rusage.Maxrss in bytes. Linux reports kilobytes.
const maxrssUnit = 1024

// memorySupported returns an error when the kernel does not list the
// children of a process in /proc, which the watchdog needs.
var memorySupported = sync.OnceValue(func() error {
	self := fmt.Sprintf("/proc/self/task/%d/children", os.Getpid())
	if _, err := os.Stat(self); err != nil {
		return fmt.Errorf("the kernel lists no process children in /proc: %w", err)
	}

	return nil
})

// treeResident returns the total resident memory in bytes of process root
// and its descendants. A process that exits during the walk adds nothing.
func treeResident(root int) (uint64, error) {
	return treeResidentIn("/proc", root)
}

// treeResidentIn is treeResident for the proc file system at proc.
func treeResidentIn(proc string, root int) (uint64, error) {
	var total uint64
	pending := []int{root}
	for len(pending) > 0 {
		pid := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		resident, children, err := processResident(proc, pid)
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ESRCH) {
			continue
		}
		if err != nil {
			return 0, err
		}
		total += resident
		pending = append(pending, children...)
	}

	return total, nil
}

// processResident returns the resident memory in bytes of process pid and
// the process IDs of its children, from the proc file system at proc.
func processResident(proc string, pid int) (uint64, []int, error) {
	dir := filepath.Join(proc, strconv.Itoa(pid))
	statm, err := os.ReadFile(filepath.Join(dir, "statm")) // #nosec G304 - A kernel file of a child process.
	if err != nil {
		return 0, nil, err
	}
	fields := strings.Fields(string(statm))
	if len(fields) < 2 {
		return 0, nil, fmt.Errorf("parse %s/statm: %q has no resident size", dir, statm)
	}
	pages, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0, nil, fmt.Errorf("parse %s/statm: %w", dir, err)
	}
	tasks, err := os.ReadDir(filepath.Join(dir, "task"))
	if err != nil {
		return 0, nil, err
	}
	var children []int
	for _, task := range tasks {
		// #nosec G304 - A kernel file of a child process.
		list, err := os.ReadFile(filepath.Join(dir, "task", task.Name(), "children"))
		if err != nil {
			return 0, nil, err
		}
		for field := range strings.FieldsSeq(string(list)) {
			child, err := strconv.Atoi(field)
			if err != nil {
				return 0, nil, fmt.Errorf("parse the children of %s: %w", dir, err)
			}
			children = append(children, child)
		}
	}

	return pages * pageSize(), children, nil
}

// AvailableMemory returns the memory in bytes that new processes can use:
// MemAvailable of the system, reduced to the headroom of each cgroup v2
// memory limit of the current process and its ancestors.
func AvailableMemory() (uint64, error) {
	available, err := memAvailable("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	headroom, err := cgroupHeadroom("/proc/self/cgroup", "/sys/fs/cgroup")
	if err != nil {
		return 0, err
	}

	return min(available, headroom), nil
}

// memAvailable reads the MemAvailable value of the meminfo file in bytes.
func memAvailable(meminfo string) (_ uint64, err error) {
	f, err := os.Open(meminfo) // #nosec G304 - A fixed kernel file, or a test fixture.
	if err != nil {
		return 0, fmt.Errorf("read the available memory: %w", err)
	}
	defer func() {
		err = errors.Join(err, f.Close())
	}()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 3 || fields[0] != "MemAvailable:" || fields[2] != "kB" {
			continue
		}
		kb, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil || kb > math.MaxUint64/1024 {
			return 0, fmt.Errorf("parse MemAvailable %q of %s", fields[1], meminfo)
		}

		return kb * 1024, nil
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("read %s: %w", meminfo, err)
	}

	return 0, fmt.Errorf("%s has no MemAvailable value", meminfo)
}

// cgroupHeadroom returns the smallest difference between memory.max and
// memory.current of the cgroup v2 of the current process and each ancestor
// under root. It returns math.MaxUint64 when no cgroup v2 limit applies.
func cgroupHeadroom(selfCgroup, root string) (uint64, error) {
	data, err := os.ReadFile(selfCgroup) // #nosec G304 - A fixed kernel file, or a test fixture.
	if err != nil {
		return 0, fmt.Errorf("read the cgroup of the process: %w", err)
	}
	var group string
	for line := range strings.SplitSeq(string(data), "\n") {
		if path, ok := strings.CutPrefix(line, "0::"); ok {
			group = path
		}
	}
	headroom := uint64(math.MaxUint64)
	if group == "" {
		return headroom, nil
	}
	for dir := filepath.Join(root, group); ; dir = filepath.Dir(dir) {
		limit, ok, err := cgroupLimitHeadroom(dir)
		if err != nil {
			return 0, err
		}
		if ok {
			headroom = min(headroom, limit)
		}
		if dir == filepath.Clean(root) || dir == filepath.Dir(dir) {
			return headroom, nil
		}
	}
}

// cgroupLimitHeadroom returns memory.max minus memory.current of dir. It
// returns false when dir has no finite memory.max.
func cgroupLimitHeadroom(dir string) (uint64, bool, error) {
	limit, ok, err := readCgroupValue(filepath.Join(dir, "memory.max"))
	if err != nil || !ok {
		return 0, false, err
	}
	current, ok, err := readCgroupValue(filepath.Join(dir, "memory.current"))
	if err != nil {
		return 0, false, err
	}
	if !ok || current >= limit {
		return 0, true, nil
	}

	return limit - current, true, nil
}

// readCgroupValue reads a byte count from a cgroup file. It returns false
// when the file does not exist or holds "max".
func readCgroupValue(path string) (uint64, bool, error) {
	data, err := os.ReadFile(path) // #nosec G304 - A cgroup file under the cgroup root.
	if errors.Is(err, os.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("read %s: %w", path, err)
	}
	text := strings.TrimSpace(string(data))
	if text == "max" {
		return 0, false, nil
	}
	value, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("parse %q of %s: %w", text, path, err)
	}

	return value, true, nil
}

// onlyZombies returns false. Linux delivers a signal to a group of zombies
// without an error.
func onlyZombies(int) bool {
	return false
}
