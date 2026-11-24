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

package engine_test

import (
	"context"
	"fmt"
	"testing"
	"testing/fstest"

	"github.com/OpenRAE/go-rae-gremlins/internal/coverage"
	"github.com/OpenRAE/go-rae-gremlins/internal/engine"
	"github.com/OpenRAE/go-rae-gremlins/internal/gomodule"
)

// benchmarkFile is the name of the benchmark source in the module.
const benchmarkFile = "bench.go"

// benchmarkFunction is a function with operators of every mutation type. Its
// %d verb is the function number.
const benchmarkFunction = `
func f%d(a, b int, p, q bool) int {
	c := a + b - a*b/2%%3
	c += a
	c -= b
	c *= 2
	c /= 3
	c %%= 4
	c &= a
	c |= b
	c ^= a
	c &^= b
	c <<= 1
	c >>= 1
	c = c&a | b ^ a&^b<<1>>1
	c++
	c--
	c = -c
	for j := 0; j < c; j++ {
		if j >= a || j <= b && j > 0 || j == b || j != a {
			break
		}
		if !p && !(q || p) {
			continue
		}
	}

	return c
}
`

// benchmarkSource returns a Go file with functions copies of
// benchmarkFunction.
func benchmarkSource(functions int) []byte {
	src := "package main\n"
	for i := range functions {
		src += fmt.Sprintf(benchmarkFunction, i)
	}

	return []byte(src)
}

// BenchmarkEngineDiscovery measures the discovery of the mutants of one file,
// without their execution.
func BenchmarkEngineDiscovery(b *testing.B) {
	src := benchmarkSource(50)
	mod := gomodule.GoModule{Name: expectedModule, Root: ".", CallingDir: "."}
	mapFS := fstest.MapFS{benchmarkFile: {Data: src}}
	b.ReportAllocs()
	for b.Loop() {
		mut := engine.New(mod, engine.CodeData{Cov: coverage.Profile{}}, &executorDealerStub{}, engine.WithDirFs(mapFS))
		if _, err := mut.Run(context.Background()); err != nil {
			b.Fatalf("Run() error = %v", err)
		}
	}
}

// BenchmarkEngineApplyAndRollback measures the discovery of the mutants of
// one file and the application and rollback of each mutant in a copy of the
// file.
func BenchmarkEngineApplyAndRollback(b *testing.B) {
	src := benchmarkSource(5)
	mod := gomodule.GoModule{Name: expectedModule, Root: ".", CallingDir: "."}
	mapFS := fstest.MapFS{benchmarkFile: {Data: src}}
	b.ReportAllocs()
	for b.Loop() {
		dealer := &applyingDealerStub{base: b.TempDir(), fileName: benchmarkFile, src: src}
		mut := engine.New(mod, engine.CodeData{Cov: coverage.Profile{}}, dealer, engine.WithDirFs(mapFS))
		if _, err := mut.Run(context.Background()); err != nil {
			b.Fatalf("Run() error = %v", err)
		}
	}
}
