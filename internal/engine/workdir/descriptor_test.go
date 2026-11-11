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

package workdir

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

const procFDDir = "/proc/self/fd"

func TestCopyReleasesFileDescriptors(t *testing.T) {
	root := t.TempDir()
	skipWithoutProcFD(t, root)
	disableGC(t)

	srcDir := filepath.Join(root, "src")
	workDir := filepath.Join(root, "work")
	for _, dir := range []string{srcDir, workDir} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	const fileCount = 20
	for i := range fileCount {
		name := filepath.Join(srcDir, fmt.Sprintf("file-%d", i))
		if err := os.WriteFile(name, fmt.Appendf(nil, "content %d", i), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	dealer := NewCachedDealer(workDir, srcDir)
	defer dealer.Clean()

	const workerCount = 5
	for w := range workerCount {
		dstDir, err := dealer.Get(fmt.Sprintf("worker-%d", w))
		if err != nil {
			t.Fatal(err)
		}
		if got := openDescriptors(t, root); got != 0 {
			t.Fatalf("after copy %d: got %d open descriptors under the fixture, want 0", w+1, got)
		}
		copied := os.DirFS(dstDir)
		for i := range fileCount {
			name := fmt.Sprintf("file-%d", i)
			got, err := fs.ReadFile(copied, name)
			if err != nil {
				t.Fatal(err)
			}
			want := fmt.Appendf(nil, "content %d", i)
			if !bytes.Equal(got, want) {
				t.Errorf("worker %d copied %s as %q, want %q", w, name, got, want)
			}
		}
	}
}

func TestCopyFailureReleasesFileDescriptors(t *testing.T) {
	testCases := []struct {
		setup func(t *testing.T, root string) (srcPath, dstPath string)
		name  string
	}{
		{
			name: "destination open fails",
			setup: func(t *testing.T, root string) (string, string) {
				t.Helper()
				srcPath := filepath.Join(root, "src")
				if err := os.WriteFile(srcPath, []byte("content"), 0o600); err != nil {
					t.Fatal(err)
				}

				return srcPath, filepath.Join(root, "missing", "dst")
			},
		},
		{
			name: "copy fails after both files open",
			setup: func(t *testing.T, root string) (string, string) {
				t.Helper()
				srcPath := filepath.Join(root, "src-dir")
				if err := os.Mkdir(srcPath, 0o700); err != nil {
					t.Fatal(err)
				}

				return srcPath, filepath.Join(root, "dst")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			skipWithoutProcFD(t, root)
			disableGC(t)
			srcPath, dstPath := tc.setup(t, root)

			if err := doCopy(srcPath, dstPath, 0o600); err == nil {
				t.Fatal("doCopy returned nil error, want an error")
			}
			if got := openDescriptors(t, root); got != 0 {
				t.Errorf("got %d open descriptors under the fixture, want 0", got)
			}
		})
	}
}

// disableGC stops finalizers from closing leaked files during the test.
func disableGC(t *testing.T) {
	t.Helper()
	previous := debug.SetGCPercent(-1)
	t.Cleanup(func() { debug.SetGCPercent(previous) })
}

func skipWithoutProcFD(t *testing.T, root string) {
	t.Helper()
	if _, err := os.ReadDir(procFDDir); err != nil {
		t.Skipf("descriptor count needs %s: %v", procFDDir, err)
	}
	if got := openDescriptors(t, root); got != 0 {
		t.Fatalf("got %d open descriptors under a new fixture, want 0", got)
	}
}

// openDescriptors counts the process descriptors that refer to paths under root.
func openDescriptors(t *testing.T, root string) int {
	t.Helper()
	entries, err := os.ReadDir(procFDDir)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	prefix := resolved + string(os.PathSeparator)
	count := 0
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join(procFDDir, entry.Name()))
		if err == nil && strings.HasPrefix(target, prefix) {
			count++
		}
	}

	return count
}
