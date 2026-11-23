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

package engine

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sync"

	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
)

// snippetContext is the number of lines before and after the mutated line in
// a snippet.
const snippetContext = 3

// parsedFile is a parsed Go source file. All the mutants of the file share
// its AST.
type parsedFile struct {
	set  *token.FileSet
	file *ast.File
	// name is the slash-separated path of the file, relative to the calling
	// directory of the module. It is the physical name: //line directives
	// do not change it.
	name string
	// mu serializes the AST edits of the mutants of the file. Mutants of
	// different files apply in parallel.
	mu sync.Mutex
}

// readWork returns the bytes of the file in the module copy at workDir.
func (f *parsedFile) readWork(workDir string) ([]byte, error) {
	var data []byte
	err := f.inWork(workDir, func(root *os.Root, name string) error {
		var err error
		data, err = root.ReadFile(name)

		return err
	})

	return data, err
}

// writeWork writes data to the file in the module copy at workDir.
func (f *parsedFile) writeWork(workDir string, data []byte) error {
	return f.inWork(workDir, func(root *os.Root, name string) error {
		return root.WriteFile(name, data, 0o600)
	})
}

// inWork calls fn with the module copy at workDir as an os.Root, and with
// the name of the file in it. The root confines the file access to the copy,
// also for a name with .. or a symbolic link. An empty workDir is the
// current directory.
func (f *parsedFile) inWork(workDir string, fn func(root *os.Root, name string) error) error {
	if workDir == "" {
		workDir = "."
	}
	root, err := os.OpenRoot(workDir)
	if err != nil {
		return err
	}
	err = fn(root, filepath.FromSlash(f.name))
	if closeErr := root.Close(); closeErr != nil {
		err = errors.Join(err, fmt.Errorf("close %s: %w", workDir, closeErr))
	}

	return err
}

// line returns the physical line of pos. //line directives do not change it.
func (f *parsedFile) line(pos token.Pos) int {
	return f.set.PositionFor(pos, false).Line
}

// source returns the Go source of the AST.
func (f *parsedFile) source() ([]byte, error) {
	var w bytes.Buffer
	if err := printer.Fprint(&w, f.set, f.file); err != nil {
		return nil, err
	}

	return w.Bytes(), nil
}

// mutant is the mutator.Mutator of one mutation type at one original
// operator of a parsed file. The registry creates it.
type mutant struct {
	reg  *registry
	src  *parsedFile
	node *NodeToken

	pkg              string
	workDir          string
	equivalentReason string
	origFile         []byte
	origSnippet      []byte
	mutatedSnippet   []byte
	status           mutator.Status
	mt               mutator.Type
}

// Type returns the mutator.Type of the mutant.
func (m *mutant) Type() mutator.Type {
	return m.mt
}

// SetType sets the mutator.Type of the mutant.
func (m *mutant) SetType(mt mutator.Type) {
	m.mt = mt
}

// Status returns the mutator.Status of the mutant.
func (m *mutant) Status() mutator.Status {
	return m.status
}

// SetStatus sets the mutator.Status of the mutant.
func (m *mutant) SetStatus(s mutator.Status) {
	m.status = s
}

// Position returns the token.Position of the operator. //line directives
// change it, so use it only to report the mutant.
func (m *mutant) Position() token.Position {
	return m.src.set.Position(m.node.TokPos)
}

// Pos returns the token.Pos of the operator.
func (m *mutant) Pos() token.Pos {
	return m.node.TokPos
}

// Pkg returns the import path of the package of the mutant.
func (m *mutant) Pkg() string {
	return m.pkg
}

// SetWorkdir sets the module copy in which Apply and Rollback change the
// file. By default, it is the current directory.
func (m *mutant) SetWorkdir(path string) {
	m.workDir = path
}

// Workdir returns the module copy in which Apply and Rollback change the
// file.
func (m *mutant) Workdir() string {
	return m.workDir
}

// Apply writes the mutated source to the file in the module copy. It keeps
// the original bytes of the file for Rollback.
//
// The AST is shared among the mutants of the file, so Apply holds the lock
// of the file during the edit and puts back the original AST on every path
// before it returns. When the spec of the mutant has no mutation for the
// operator, Apply returns an error that wraps ErrNoMutation and writes
// nothing. When the mutated file cannot be printed or written, Apply writes
// back the original file and returns the error, joined with any restoration
// error.
func (m *mutant) Apply() error {
	s, ok := m.reg.byType[m.mt]
	if !ok {
		return noMutation(m.mt, m.node.Tok())
	}
	m.src.mu.Lock()
	defer m.src.mu.Unlock()

	origFile, err := m.src.readWork(m.workDir)
	if err != nil {
		return err
	}
	m.origFile = origFile
	line := m.src.line(m.node.TokPos)
	m.origSnippet = extractSnippet(origFile, line, snippetContext)

	restore, err := s.mutate(m.node)
	if err != nil {
		return err
	}
	defer restore()

	payload, err := m.src.source()
	if err == nil {
		err = m.src.writeWork(m.workDir, payload)
	}
	if err != nil {
		defer m.resetOrigFile()
		if restoreErr := m.src.writeWork(m.workDir, origFile); restoreErr != nil {
			return errors.Join(err, fmt.Errorf("restore %s: %w", m.src.name, restoreErr))
		}

		return err
	}
	m.mutatedSnippet = extractSnippet(payload, line, snippetContext)

	return nil
}

// Rollback writes back the original file after the test and frees the
// original bytes.
func (m *mutant) Rollback() error {
	defer m.resetOrigFile()

	return m.src.writeWork(m.workDir, m.origFile)
}

func (m *mutant) resetOrigFile() {
	m.origFile = nil
}

// OrigSnippet returns the original code snippet around the mutation point.
func (m *mutant) OrigSnippet() []byte {
	return m.origSnippet
}

// MutatedSnippet returns the mutated code snippet around the mutation point.
func (m *mutant) MutatedSnippet() []byte {
	return m.mutatedSnippet
}

// EquivalentReason returns the review reason of the annotation that marks
// the mutant as equivalent, or an empty string.
func (m *mutant) EquivalentReason() string {
	return m.equivalentReason
}

// SetEquivalentReason sets the review reason of the annotation that marks
// the mutant as equivalent.
func (m *mutant) SetEquivalentReason(reason string) {
	m.equivalentReason = reason
}

// extractSnippet extracts lines around the target line with the given context size.
func extractSnippet(content []byte, targetLine, contextLines int) []byte {
	lines := bytes.Split(content, []byte("\n"))

	start := max(targetLine-contextLines-1, 0)
	end := min(targetLine+contextLines, len(lines))
	start = min(start, end)

	return bytes.Join(lines[start:end], []byte("\n"))
}
