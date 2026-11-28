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
	"go/ast"
	"maps"
	"regexp"
	"strconv"
	"strings"
)

// pkgNames is the set of the package-level names that the files of one
// package declare. A nil set means that the names are not known, for example
// because a file of the package could not be read. Then no predeclared
// identifier counts.
type pkgNames map[string]struct{}

// fileNames returns the names that hide predeclared identifiers in file: the
// package-level names pkg and the import names of file. An import without a
// name gets the last element of its path, and the element before a major
// version element such as v2. It returns nil when pkg is nil.
func fileNames(pkg pkgNames, file *ast.File) pkgNames {
	if pkg == nil {
		return nil
	}
	names := maps.Clone(pkg)
	for _, imp := range file.Imports {
		if imp.Name != nil {
			names[imp.Name.Name] = struct{}{}

			continue
		}
		importPath, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		elems := strings.Split(importPath, "/")
		last := elems[len(elems)-1]
		names[last] = struct{}{}
		if len(elems) > 1 && majorVersion.MatchString(last) {
			names[elems[len(elems)-2]] = struct{}{}
		}
	}

	return names
}

// majorVersion matches the major version element of a module path.
var majorVersion = regexp.MustCompile(`^v[0-9]+$`)

// addPackageNames adds to names the package-level names that file declares:
// its types, variables, constants, and functions. Methods do not declare a
// package-level name.
func addPackageNames(names pkgNames, file *ast.File) {
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil {
				names[d.Name.Name] = struct{}{}
			}
		case *ast.GenDecl:
			addSpecNames(names, d.Specs)
		default:
			// A bad declaration from a syntax error declares no name.
		}
	}
}

func addSpecNames(names pkgNames, specs []ast.Spec) {
	for _, spec := range specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			names[s.Name.Name] = struct{}{}
		case *ast.ValueSpec:
			addIdents(names, s.Names)
		default:
			// An import declares a name of the file, not of the package.
		}
	}
}

func addIdents(names pkgNames, idents []*ast.Ident) {
	for _, id := range idents {
		names[id.Name] = struct{}{}
	}
}

// resultKind is the kind of one result value of a function, as its type
// shows it without type information.
type resultKind int

const (
	// otherResult is a type that the syntax does not classify, for example a
	// named type, a struct, an array, or a type parameter.
	otherResult resultKind = iota
	// nilableResult is a pointer, slice, map, channel, function, or interface
	// type, or the predeclared error or any.
	nilableResult
	boolResult
	numberResult
	stringResult
)

// predeclaredKinds are the kinds of the predeclared type names that a result
// type can use.
var predeclaredKinds = map[string]resultKind{
	"any": nilableResult, "error": nilableResult,
	"bool":   boolResult,
	"string": stringResult,
	"int":    numberResult, "int8": numberResult, "int16": numberResult, "int32": numberResult, "int64": numberResult,
	"uint": numberResult, "uint8": numberResult, "uint16": numberResult, "uint32": numberResult, "uint64": numberResult,
	"uintptr": numberResult, "byte": numberResult, "rune": numberResult,
	"float32": numberResult, "float64": numberResult, "complex64": numberResult, "complex128": numberResult,
}

// funcCandidate is a function declaration with a body. It is the candidate of
// the extreme mutation types of ADR 0012.
type funcCandidate struct {
	decl *ast.FuncDecl
	// pkg holds the package-level and import names of the file of decl.
	pkg pkgNames
	// local holds the names that the declaration declares for its body: the
	// type parameters, the receiver, the parameters, and the results.
	local pkgNames
	// results holds the kind of each result value, in order. It is empty for
	// a function without results.
	results []resultKind
}

// newFuncCandidate returns the candidate of decl in a file where the names
// pkg hide predeclared identifiers. A nil pkg means unknown names.
func newFuncCandidate(decl *ast.FuncDecl, pkg pkgNames) *funcCandidate {
	fn := &funcCandidate{decl: decl, pkg: pkg, local: localNames(decl)}
	if decl.Type.Results == nil {
		return fn
	}
	for _, field := range decl.Type.Results.List {
		kind := fn.kindOf(field.Type)
		for range max(len(field.Names), 1) {
			fn.results = append(fn.results, kind)
		}
	}

	return fn
}

// predeclared reports whether name denotes the predeclared identifier in the
// function body: no package-level, import, or local name shadows it, and the
// package names are known.
func (fn *funcCandidate) predeclared(name string) bool {
	if fn.pkg == nil {
		return false
	}
	_, inPkg := fn.pkg[name]
	_, inLocal := fn.local[name]

	return !inPkg && !inLocal
}

// kindOf returns the kind of the result type expr.
func (fn *funcCandidate) kindOf(expr ast.Expr) resultKind {
	switch e := expr.(type) {
	case *ast.ParenExpr:
		return fn.kindOf(e.X)
	case *ast.StarExpr, *ast.MapType, *ast.ChanType, *ast.FuncType, *ast.InterfaceType:
		return nilableResult
	case *ast.ArrayType:
		if e.Len == nil {
			return nilableResult
		}
	case *ast.Ident:
		if kind, ok := predeclaredKinds[e.Name]; ok && fn.predeclared(e.Name) {
			return kind
		}
	default:
		// A qualified, indexed, or struct type is not classified.
	}

	return otherResult
}

// localNames returns the names that decl declares for its body.
func localNames(decl *ast.FuncDecl) pkgNames {
	names := pkgNames{}
	for _, list := range []*ast.FieldList{decl.Recv, decl.Type.TypeParams, decl.Type.Params, decl.Type.Results} {
		if list == nil {
			continue
		}
		for _, field := range list.List {
			addIdents(names, field.Names)
		}
	}
	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		addIdents(names, receiverTypeParams(decl.Recv.List[0].Type))
	}

	return names
}

// receiverTypeParams returns the type parameters that a receiver type
// declares, for example K and V of *Map[K, V].
func receiverTypeParams(expr ast.Expr) []*ast.Ident {
	switch e := expr.(type) {
	case *ast.ParenExpr:
		return receiverTypeParams(e.X)
	case *ast.StarExpr:
		return receiverTypeParams(e.X)
	case *ast.IndexExpr:
		return identsOf([]ast.Expr{e.Index})
	case *ast.IndexListExpr:
		return identsOf(e.Indices)
	default:
		return nil
	}
}

func identsOf(exprs []ast.Expr) []*ast.Ident {
	var idents []*ast.Ident
	for _, e := range exprs {
		if id, ok := e.(*ast.Ident); ok {
			idents = append(idents, id)
		}
	}

	return idents
}
