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

// Package gomodule detects and provides information about Go modules.
package gomodule

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

// GoModule represents the current execution context in Gremlins.
//
//	Name is the module name of the Go module being tested by Gremlins.
//	Root is the root folder of the Go module.
//	CallingDir is the folder in which Gremlins is running.
type GoModule struct {
	Name       string
	Root       string
	CallingDir string
}

// Init initializes the current module. It finds the module name and the root
// of the module, then returns a GoModule struct.
func Init(path string) (GoModule, error) {
	if path == "" {
		return GoModule{}, errors.New("path is not set")
	}
	root := findModuleRoot(path)
	if root == "" {
		return GoModule{}, fmt.Errorf("no go.mod file in %q or a parent directory", path)
	}
	name, err := moduleName(root)
	if err != nil {
		return GoModule{}, err
	}
	callingDir, err := filepath.Rel(root, path)
	if err != nil {
		return GoModule{}, fmt.Errorf("resolve %q relative to module root %q: %w", path, root, err)
	}

	return GoModule{
		Name:       name,
		Root:       root,
		CallingDir: callingDir,
	}, nil
}

// moduleName returns the module path that the go.mod file in root declares.
// ParseLax ignores directives that this version of x/mod does not know, so a
// go.mod file from a newer Go release still gives its module path.
func moduleName(root string) (string, error) {
	goMod := filepath.Join(root, "go.mod")
	// G304 reports a file path from input. The operator selects this module, and
	// the process reads its go.mod with the operator's own permissions. The read
	// follows symbolic links, as the go command does.
	data, err := os.ReadFile(goMod) //nolint:gosec // Operator-selected module file; no privilege boundary.
	if err != nil {
		return "", err
	}
	file, err := modfile.ParseLax(goMod, data, nil)
	if err != nil {
		return "", fmt.Errorf("parse module file: %w", err)
	}
	if file.Module == nil || file.Module.Mod.Path == "" {
		return "", fmt.Errorf("%s: no module declaration", goMod)
	}
	name := file.Module.Mod.Path
	if err := module.CheckImportPath(name); err != nil {
		return "", fmt.Errorf("%s: invalid module path: %w", goMod, err)
	}

	return name, nil
}

func findModuleRoot(path string) string {
	// Inspired by how Go itself finds the module root.
	path = filepath.Clean(path)
	for {
		if fi, err := os.Stat(filepath.Join(path, "go.mod")); err == nil && !fi.IsDir() {
			return path
		}
		d := filepath.Dir(path)
		if d == path {
			break
		}
		path = d
	}

	return ""
}
