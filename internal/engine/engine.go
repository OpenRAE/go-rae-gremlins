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

// Package engine orchestrates mutation testing by discovering, applying, and testing mutations.
package engine

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/OpenRAE/go-rae-gremlins/internal/coverage"
	"github.com/OpenRAE/go-rae-gremlins/internal/diff"
	"github.com/OpenRAE/go-rae-gremlins/internal/engine/workerpool"
	"github.com/OpenRAE/go-rae-gremlins/internal/exclusion"
	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
	"github.com/OpenRAE/go-rae-gremlins/internal/report"

	"github.com/OpenRAE/go-rae-gremlins/internal/configuration"
	"github.com/OpenRAE/go-rae-gremlins/internal/gomodule"
)

// Engine is the "engine" that performs the mutation testing.
//
// It traverses the AST of the project, finds which TokenMutator can be applied and
// performs the actual mutation testing.
type Engine struct {
	fs           fs.FS
	jDealer      ExecutorDealer
	codeData     CodeData
	mutantStream chan mutator.Mutator
	module       gomodule.GoModule
	logger       report.MutantLogger
}

// CodeData is used to check if the mutant should be executed.
type CodeData struct {
	Cov       coverage.Profile
	Diff      diff.Diff
	Exclusion exclusion.Rules
}

// Option for the Engine initialization.
type Option func(m Engine) Engine

// New instantiates an Engine.
//
// It gets a fs.FS on which to perform the analysis, a CodeData to
// check if the mutants are executable and a sets of Option.
func New(mod gomodule.GoModule, codeData CodeData, jDealer ExecutorDealer, opts ...Option) Engine {
	dirFS := os.DirFS(filepath.Join(mod.Root, mod.CallingDir))
	mut := Engine{
		module:   mod,
		jDealer:  jDealer,
		codeData: codeData,
		fs:       dirFS,
		logger:   report.NewLogger(),
	}
	for _, opt := range opts {
		mut = opt(mut)
	}

	return mut
}

// WithDirFs overrides the fs.FS of the module (mainly used for testing purposes).
func WithDirFs(dirFS fs.FS) Option {
	return func(m Engine) Engine {
		m.fs = dirFS

		return m
	}
}

// Run executes the mutation testing.
//
// It walks the fs.FS provided and checks every .go file which is not a test.
// For each file it will scan for tokenMutations and gather all the mutants found.
// A walk, open, or parse failure stops the discovery. When an executor fails,
// Run stops the discovery and schedules no new mutants. Run then returns the
// joined errors with the partial results, which the caller must not report as
// success. The results do not contain the failed mutants.
// When ctx is done, Run stops the discovery and the pending tests. Run returns
// only after the discovery and the workers it started have exited.
func (mu *Engine) Run(ctx context.Context) (report.Results, error) {
	runCtx, stop := context.WithCancel(ctx)
	defer stop()

	mu.mutantStream = make(chan mutator.Mutator)
	discoverErr := make(chan error, 1)
	go func() {
		defer close(mu.mutantStream)
		discoverErr <- mu.discover(runCtx)
	}()

	start := time.Now()
	res, execErr := mu.executeTests(runCtx, stop)
	err := <-discoverErr
	res.Elapsed = time.Since(start)
	res.Module = mu.module.Name

	var errs []error
	if err != nil {
		errs = append(errs, fmt.Errorf("discovering mutants: %w", err))
	}
	if execErr != nil {
		errs = append(errs, execErr)
	}

	return res, errors.Join(errs...)
}

// discover walks the module file system and sends the mutants of each Go
// source file to the mutant stream. It returns the first walk, open, or parse
// error. It stops without an error when ctx is done.
func (mu *Engine) discover(ctx context.Context) error {
	return fs.WalkDir(mu.fs, ".", func(walkPath string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return fs.SkipAll
		}
		if err != nil {
			return err
		}
		if d.IsDir() {
			if walkPath != "." && mu.skipDir(walkPath, d.Name()) {
				return fs.SkipDir
			}

			return nil
		}

		isGoCode := filepath.Ext(walkPath) == ".go" && !strings.HasSuffix(walkPath, "_test.go")
		if !isGoCode || mu.codeData.Exclusion.IsFileExcluded(walkPath) {
			return nil
		}

		return mu.runOnFile(ctx, walkPath)
	})
}

// skipDir reports whether dirPath is a testdata tree, a vendor tree, or the
// root of a nested Go module. None of these belong to the module being
// mutated, so Run must not descend into them.
func (mu *Engine) skipDir(dirPath, name string) bool {
	if name == "testdata" || name == "vendor" {
		return true
	}

	_, err := fs.Stat(mu.fs, path.Join(dirPath, "go.mod"))

	return err == nil
}

func (mu *Engine) runOnFile(ctx context.Context, fileName string) error {
	set := token.NewFileSet()
	file, err := mu.parseFile(set, fileName)
	if err != nil {
		return err
	}

	built, err := mu.hostBuilds(fileName)
	if err != nil {
		return err
	}
	cov := newFileCoverage(set, file, mu.codeData.Cov, built)
	stopped := false
	ast.Inspect(file, func(node ast.Node) bool {
		if stopped {
			return false
		}
		n, ok := NewTokenNode(node)
		if !ok {
			return true
		}
		stopped = !mu.findMutations(ctx, fileName, set, file, n, cov)

		return !stopped
	})
	if stopped {
		return fs.SkipAll
	}

	return nil
}

// parseFile parses a Go file of the module file system.
func (mu *Engine) parseFile(set *token.FileSet, fileName string) (*ast.File, error) {
	src, err := mu.fs.Open(fileName)
	if err != nil {
		return nil, err
	}
	file, err := parser.ParseFile(set, fileName, src, parser.ParseComments)
	if closeErr := src.Close(); closeErr != nil {
		err = errors.Join(err, fmt.Errorf("close: %w", closeErr))
	}
	if err != nil {
		return nil, err
	}

	return file, nil
}

// hostBuilds tells if the host build with the configured build tags includes
// a Go file of the module file system. It applies the file name suffixes and
// the build constraints of the file, as the go command does.
func (mu *Engine) hostBuilds(fileName string) (bool, error) {
	ctx := build.Default
	ctx.BuildTags = strings.FieldsFunc(configuration.Get[string](configuration.UnleashTagsKey), func(r rune) bool {
		return r == ',' || r == ' '
	})
	ctx.JoinPath = path.Join
	ctx.OpenFile = func(name string) (io.ReadCloser, error) { return mu.fs.Open(name) }

	ok, err := ctx.MatchFile(path.Dir(fileName), path.Base(fileName))
	if err != nil {
		return false, fmt.Errorf("matching build constraints: %w", err)
	}

	return ok, nil
}

// findMutations sends the mutants of node to the mutant stream. It returns
// false when ctx is done before a mutant is sent.
func (mu *Engine) findMutations(
	ctx context.Context, fileName string, set *token.FileSet, file *ast.File, node *NodeToken, cov fileCoverage,
) bool {
	mutantTypes, ok := TokenMutantType[node.Tok()]
	if !ok {
		return true
	}

	pkg := mu.pkgName(fileName)
	for _, mt := range mutantTypes {
		if !configuration.Get[bool](configuration.MutantTypeEnabledKey(mt)) {
			continue
		}
		mutantType := mt
		tm := NewTokenMutant(pkg, set, file, node)
		tm.SetType(mutantType)
		tm.SetStatus(mu.mutationStatus(cov, node.TokPos))

		select {
		case mu.mutantStream <- tm:
		case <-ctx.Done():
			return false
		}
	}

	return true
}

// pkgName returns the import path of the package that fileName belongs to.
// The package is the directory of the file, relative to the module root. The
// package declaration name does not identify the directory, for example for a
// main package. fileName is slash-separated and relative to the calling
// directory, as fs.WalkDir reports it.
func (mu *Engine) pkgName(fileName string) string {
	dir := path.Join(filepath.ToSlash(mu.module.CallingDir), path.Dir(fileName))
	if dir == "." {
		return mu.module.Name
	}

	return mu.module.Name + "/" + dir
}

// mutationStatus returns the status of a new mutant at tokPos. A mutant that
// is outside the diff, or that no test run can cover, is skipped. Coverage
// then tells if the mutant is runnable or not covered.
func (mu *Engine) mutationStatus(cov fileCoverage, tokPos token.Pos) mutator.Status {
	switch {
	case !mu.codeData.Diff.IsChanged(cov.set.Position(tokPos)) || !cov.isObservable(tokPos):
		return mutator.Skipped
	case cov.isCovered(tokPos):
		return mutator.Runnable
	default:
		return mutator.NotCovered
	}
}

// executeTests runs an executor for each mutant of the stream on a worker
// pool. It stops the pool when the stream ends or ctx is done, and returns
// after all the started executors have reported. When an executor fails,
// executeTests calls stop, so the discovery ends and no new mutant is
// scheduled. It returns the joined executor errors.
func (mu *Engine) executeTests(ctx context.Context, stop context.CancelFunc) (report.Results, error) {
	pool := workerpool.Initialize("mutator")
	pool.Start(ctx)

	var mutants []mutator.Mutator
	var errs []error
	outCh := make(chan ExecutorResult)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer pool.Stop()
		mu.submit(ctx, pool, outCh, wg)
	}()

	go func() {
		wg.Wait()
		close(outCh)
	}()

	for r := range outCh {
		if r.Err != nil {
			errs = append(errs, r.Err)
			stop()

			continue
		}
		mu.logger.Mutant(r.Mutant)
		mutants = append(mutants, r.Mutant)
	}

	if len(errs) > 0 {
		return results(mutants), fmt.Errorf("mutation testing failed: %w", errors.Join(errs...))
	}

	return results(mutants), nil
}

// submit adds an executor to the pool for each mutant of the stream, until
// the stream ends or ctx is done.
func (mu *Engine) submit(ctx context.Context, pool *workerpool.Pool, outCh chan<- ExecutorResult, wg *sync.WaitGroup) {
	for {
		var mut mutator.Mutator
		select {
		case <-ctx.Done():
			return
		case m, ok := <-mu.mutantStream:
			if !ok {
				return
			}
			mut = m
		}
		wg.Add(1)
		if !pool.AppendExecutor(ctx, mu.jDealer.NewExecutor(mut, outCh, wg)) {
			wg.Done()

			return
		}
	}
}

func results(m []mutator.Mutator) report.Results {
	return report.Results{Mutants: m}
}
