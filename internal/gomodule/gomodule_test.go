/*
 * Copyright 2022 The Gremlins Authors
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

package gomodule_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenRAE/go-rae-gremlins/internal/gomodule"
)

const exampleModule = "example.com/m"

// writeGoMod writes content to dir/go.mod and returns the file path.
func writeGoMod(t *testing.T, dir, content string) string {
	t.Helper()
	goMod := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(goMod, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", goMod, err)
	}

	return goMod
}

// makeDir creates the directory path under root and returns its path.
func makeDir(t *testing.T, root string, path ...string) string {
	t.Helper()
	dir := filepath.Join(append([]string{root}, path...)...)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("create %s: %v", dir, err)
	}

	return dir
}

func TestDetectsModule(t *testing.T) {
	t.Run("does not return error if it can retrieve module", func(t *testing.T) {
		rootDir := t.TempDir()
		pkgDir := "pkgDir"
		absPkgDir := makeDir(t, rootDir, pkgDir)
		writeGoMod(t, rootDir, "module "+exampleModule)

		mod, err := gomodule.Init(absPkgDir)
		if err != nil {
			t.Fatalf("Init(%q) error = %v, want nil", absPkgDir, err)
		}

		if mod.Name != exampleModule {
			t.Errorf("Init(%q).Name = %q, want %q", absPkgDir, mod.Name, exampleModule)
		}
		if mod.Root != rootDir {
			t.Errorf("Init(%q).Root = %q, want %q", absPkgDir, mod.Root, rootDir)
		}
		if mod.CallingDir != pkgDir {
			t.Errorf("Init(%q).CallingDir = %q, want %q", absPkgDir, mod.CallingDir, pkgDir)
		}
	})

	t.Run("returns error if go.mod is invalid", func(t *testing.T) {
		path := t.TempDir()
		writeGoMod(t, path, "")

		if _, err := gomodule.Init(path); err == nil {
			t.Errorf("Init(%q) with an empty go.mod error = nil, want an error", path)
		}
	})

	t.Run("returns error if it cannot find module", func(t *testing.T) {
		path := t.TempDir()
		if _, err := gomodule.Init(path); err == nil {
			t.Errorf("Init(%q) without go.mod error = nil, want an error", path)
		}
	})

	t.Run("returns error if path is empty", func(t *testing.T) {
		if _, err := gomodule.Init(""); err == nil {
			t.Error(`Init("") error = nil, want an error`)
		}
	})
}

func TestRegressionModuleDeclaration(t *testing.T) {
	testCases := []struct {
		name    string
		content string
	}{
		{name: "comment before module", content: "// license\nmodule example.com/m\n\ngo 1.25.0\n"},
		{name: "blank lines before module", content: "\n\nmodule example.com/m\n"},
		{name: "comments and blank lines before module", content: "// a\n\n// b\nmodule example.com/m\n"},
		{name: "comment before module with CRLF", content: "// license\r\nmodule example.com/m\r\n"},
		{name: "quoted path", content: "module \"example.com/m\"\n"},
		{name: "block form", content: "module (\n\texample.com/m\n)\n"},
		{name: "tab separator", content: "module\texample.com/m\n"},
		{name: "trailing comment", content: "module example.com/m // note\n"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeGoMod(t, root, tc.content)

			mod, err := gomodule.Init(root)
			if err != nil {
				t.Fatalf("Init(%q) error = %v, want nil", root, err)
			}
			if mod.Name != exampleModule {
				t.Errorf("Init(%q).Name = %q, want %q", root, mod.Name, exampleModule)
			}
			if mod.Root != root {
				t.Errorf("Init(%q).Root = %q, want %q", root, mod.Root, root)
			}
			if mod.CallingDir != "." {
				t.Errorf("Init(%q).CallingDir = %q, want %q", root, mod.CallingDir, ".")
			}
		})
	}
}

func TestRegressionModuleDeclarationErrors(t *testing.T) {
	testCases := []struct {
		name    string
		content string
	}{
		{name: "comment only", content: "// only a comment\n"},
		{name: "no module directive", content: "go 1.25.0\n"},
		{name: "module without path", content: "module\n"},
		{name: "module with extra argument", content: "module example.com/m extra\n"},
		{name: "repeated module", content: "module example.com/a\nmodule example.com/b\n"},
		{name: "unterminated quoted path", content: "module \"example.com/m\n"},
		{name: "empty quoted path", content: "module \"\"\n"},
		{name: "raw string path", content: "module `example.com/m`\n"},
		{name: "path with a space", content: "module \"example.com/a b\"\n"},
		{name: "local path", content: "module ./m\n"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			goMod := writeGoMod(t, root, tc.content)

			mod, err := gomodule.Init(root)
			if err == nil {
				t.Fatalf("Init(%q) = %+v, nil; want an error for %q", root, mod, tc.content)
			}
			if !strings.Contains(err.Error(), goMod) {
				t.Errorf("Init(%q) error = %q, want it to name %q", root, err, goMod)
			}
			if mod != (gomodule.GoModule{}) {
				t.Errorf("Init(%q) module = %+v, want the zero value", root, mod)
			}
		})
	}

	t.Run("invalid nested module does not use the parent module", func(t *testing.T) {
		root := t.TempDir()
		writeGoMod(t, root, "module "+exampleModule+"\n")
		nested := makeDir(t, root, "nested")
		goMod := writeGoMod(t, nested, "go 1.25.0\n")

		mod, err := gomodule.Init(nested)
		if err == nil {
			t.Fatalf("Init(%q) = %+v, nil; want an error for %s", nested, mod, goMod)
		}
		if !strings.Contains(err.Error(), goMod) {
			t.Errorf("Init(%q) error = %q, want it to name %q", nested, err, goMod)
		}
	})
}
