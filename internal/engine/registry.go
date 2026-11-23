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
	"errors"
	"fmt"
	"go/token"
	"iter"
	"slices"

	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
)

// errInvalidRegistry means that a list of specs cannot form a registry.
var errInvalidRegistry = errors.New("invalid mutation registry")

// registry is the validated set of specs, one for each mutation type. It
// does not change after its construction, so concurrent use is safe.
type registry struct {
	byType map[mutator.Type]spec
	// byToken holds the specs of each operator token, in registration
	// order.
	byToken map[token.Token][]spec
}

// newRegistry returns the registry of specs. The order of specs is the order
// of the mutants of one operator. newRegistry returns an error for a nil
// spec, for a spec of a type that mutator.Types does not contain, for a
// second spec of a type, and for a type of mutator.Types without a spec.
func newRegistry(specs ...spec) (*registry, error) {
	r := &registry{byType: make(map[mutator.Type]spec, len(specs)), byToken: make(map[token.Token][]spec)}
	for i, s := range specs {
		if s == nil {
			return nil, fmt.Errorf("%w: spec %d is nil", errInvalidRegistry, i)
		}
		mt := s.mutationType()
		if !slices.Contains(mutator.Types, mt) {
			return nil, fmt.Errorf("%w: spec %d has unknown type %d", errInvalidRegistry, i, int(mt))
		}
		if _, dup := r.byType[mt]; dup {
			return nil, fmt.Errorf("%w: second spec of %s", errInvalidRegistry, mt)
		}
		r.byType[mt] = s
		for _, tok := range s.tokens() {
			r.byToken[tok] = append(r.byToken[tok], s)
		}
	}
	for _, mt := range mutator.Types {
		if _, ok := r.byType[mt]; !ok {
			return nil, fmt.Errorf("%w: no spec of %s", errInvalidRegistry, mt)
		}
	}

	return r, nil
}

// mustRegistry returns the registry of specs. It panics when the specs
// cannot form a registry, because the built-in specs are a program
// invariant.
func mustRegistry(specs ...spec) *registry {
	r, err := newRegistry(specs...)
	if err != nil {
		panic(err)
	}

	return r
}

// eligibleTypes returns the mutation types that can mutate the original
// operator n, in registration order.
func (r *registry) eligibleTypes(n *NodeToken) iter.Seq[mutator.Type] {
	return func(yield func(mutator.Type) bool) {
		for _, s := range r.byToken[n.Tok()] {
			if s.eligible(n) && !yield(s.mutationType()) {
				return
			}
		}
	}
}

// eligible reports whether mutation type mt can mutate the original
// operator n.
func (r *registry) eligible(n *NodeToken, mt mutator.Type) bool {
	s, ok := r.byType[mt]

	return ok && s.eligible(n)
}

// newMutant returns the mutant of type mt at the operator n of the parsed
// file src, in the package pkg. It does not set the status and does not
// change the AST.
func (r *registry) newMutant(src *parsedFile, pkg string, n *NodeToken, mt mutator.Type) *mutant {
	return &mutant{reg: r, src: src, node: n, pkg: pkg, mt: mt}
}
