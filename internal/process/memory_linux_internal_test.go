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
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestFeatureMemAvailableReadsMeminfo(t *testing.T) {
	testCases := map[string]struct {
		meminfo string
		want    uint64
		wantErr bool
	}{
		"reads MemAvailable in bytes": {
			meminfo: "MemTotal:       16000 kB\nMemFree:  100 kB\nMemAvailable:    2048 kB\n",
			want:    2048 * 1024,
		},
		"rejects a missing value":   {meminfo: "MemTotal: 16000 kB\n", wantErr: true},
		"rejects a malformed value": {meminfo: "MemAvailable: x kB\n", wantErr: true},
		"rejects an overflow":       {meminfo: "MemAvailable: 18446744073709551615 kB\n", wantErr: true},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "meminfo")
			writeFile(t, path, tc.meminfo)
			got, err := memAvailable(path)
			if (err != nil) != tc.wantErr {
				t.Fatalf("memAvailable(%q) error = %v, want error %t", tc.meminfo, err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("memAvailable(%q) = %d, want %d", tc.meminfo, got, tc.want)
			}
		})
	}
}

func TestFeatureCgroupHeadroomUsesTheSmallestLimit(t *testing.T) {
	const (
		nested     = "0::/a/b\n"
		outerMax   = "a/memory.max"
		outerUsage = "a/memory.current"
		thousand   = "1000\n"
	)
	testCases := map[string]struct {
		self    string
		files   map[string]string
		want    uint64
		wantErr bool
	}{
		"no cgroup v2 entry has no limit": {
			self: "12:memory:/a\n",
			want: math.MaxUint64,
		},
		"no limit file has no limit": {
			self: nested,
			want: math.MaxUint64,
		},
		"an ancestor limit applies": {
			self: nested,
			files: map[string]string{
				outerMax: thousand, outerUsage: "400\n",
				"a/b/memory.max": "max\n", "a/b/memory.current": "300\n",
			},
			want: 600,
		},
		"the smallest headroom wins": {
			self: nested,
			files: map[string]string{
				outerMax: thousand, outerUsage: "400\n",
				"a/b/memory.max": "500\n", "a/b/memory.current": "300\n",
			},
			want: 200,
		},
		"a full cgroup has no headroom": {
			self:  "0::/a\n",
			files: map[string]string{outerMax: thousand, outerUsage: "1200\n"},
			want:  0,
		},
		"a malformed limit is an error": {
			self:    "0::/a\n",
			files:   map[string]string{outerMax: "lots\n"},
			wantErr: true,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			self := filepath.Join(dir, "self-cgroup")
			writeFile(t, self, tc.self)
			root := filepath.Join(dir, "cgroup")
			if err := os.MkdirAll(filepath.Join(root, "a", "b"), 0o750); err != nil {
				t.Fatalf("create the cgroup tree: %v", err)
			}
			for name, content := range tc.files {
				writeFile(t, filepath.Join(root, name), content)
			}
			got, err := cgroupHeadroom(self, root)
			if (err != nil) != tc.wantErr {
				t.Fatalf("cgroupHeadroom() error = %v, want error %t", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("cgroupHeadroom() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestFeatureTreeResidentCountsTheProcess(t *testing.T) {
	got, err := treeResident(os.Getpid())
	if err != nil {
		t.Fatalf("treeResident(self) = %v, want nil", err)
	}
	if got == 0 {
		t.Error("treeResident(self) = 0, want the resident memory of the test process")
	}
	if got, err := treeResident(math.MaxInt32); err != nil || got != 0 {
		t.Errorf("treeResident(missing process) = %d, %v, want 0, nil", got, err)
	}
}

func TestFeatureTreeResidentInSumsTheTree(t *testing.T) {
	const (
		rootStatm    = "10/statm"
		rootChildren = "10/task/10/children"
	)
	page := pageSize()
	testCases := map[string]struct {
		files   map[string]string
		want    uint64
		wantErr bool
	}{
		"parent and child": {
			files: map[string]string{
				rootStatm: "100 3 0 0 0 0 0\n", rootChildren: "11 ",
				"11/statm": "100 4 0 0 0 0 0\n", "11/task/11/children": "",
			},
			want: 7 * page,
		},
		"a child that exited adds nothing": {
			files: map[string]string{rootStatm: "100 3 0 0 0 0 0\n", rootChildren: "11"},
			want:  3 * page,
		},
		"a statm without a resident size": {
			files:   map[string]string{rootStatm: "100\n", rootChildren: ""},
			wantErr: true,
		},
		"a statm with a malformed resident size": {
			files:   map[string]string{rootStatm: "100 x\n", rootChildren: ""},
			wantErr: true,
		},
		"a malformed child list": {
			files:   map[string]string{rootStatm: "100 3\n", rootChildren: "eleven"},
			wantErr: true,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			proc := t.TempDir()
			for file, content := range tc.files {
				path := filepath.Join(proc, file)
				if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
					t.Fatalf("create the directory of %s: %v", file, err)
				}
				writeFile(t, path, content)
			}
			got, err := treeResidentIn(proc, 10)
			if (err != nil) != tc.wantErr {
				t.Fatalf("treeResidentIn() error = %v, want error %t", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("treeResidentIn() = %d, want %d", got, tc.want)
			}
		})
	}
}

// writeFile writes content to path.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
