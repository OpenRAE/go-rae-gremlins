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
	"maps"
	"slices"

	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
)

// ErrNoMutation means that the mutation type has no mutation for the
// operator.
var ErrNoMutation = errors.New("no mutation for the token")

// spec is the behavior of one mutation type. A spec has no state, so all the
// mutants of its type share it. It reads only the original syntax and edits
// only the shared AST.
type spec interface {
	// mutationType returns the mutation type of the spec.
	mutationType() mutator.Type

	// tokens returns the operator tokens that the spec can mutate. The
	// registry uses them to select the specs of an operator.
	tokens() []token.Token

	// eligible reports whether the spec can mutate the original operator n.
	eligible(n *NodeToken) bool

	// mutate changes the shared AST to the mutation of n. It returns the
	// function that puts back the original AST exactly. It returns an error
	// that wraps ErrNoMutation, and changes nothing, when n is not eligible.
	mutate(n *NodeToken) (func(), error)
}

// tokenSpec is a spec that replaces the operator token from a table.
type tokenSpec struct {
	replacements map[token.Token]token.Token
	mt           mutator.Type
}

func (s tokenSpec) mutationType() mutator.Type {
	return s.mt
}

func (s tokenSpec) tokens() []token.Token {
	return slices.Collect(maps.Keys(s.replacements))
}

func (s tokenSpec) eligible(n *NodeToken) bool {
	_, ok := s.replacements[n.Tok()]

	return ok
}

func (s tokenSpec) mutate(n *NodeToken) (func(), error) {
	orig := n.Tok()
	mutated, ok := s.replacements[orig]
	if !ok {
		return nil, noMutation(s.mt, orig)
	}
	n.SetTok(mutated)

	return func() { n.SetTok(orig) }, nil
}

// logicalSpec is the INVERT_LOGICAL spec. It replaces && and || from its
// token table, and it changes !x to !!x.
type logicalSpec struct {
	tokenSpec
}

func (s logicalSpec) tokens() []token.Token {
	return append(s.tokenSpec.tokens(), token.NOT)
}

func (s logicalSpec) eligible(n *NodeToken) bool {
	return isNegation(n) || s.tokenSpec.eligible(n)
}

func (s logicalSpec) mutate(n *NodeToken) (func(), error) {
	if isNegation(n) {
		return doubleNegation(n), nil
	}

	return s.tokenSpec.mutate(n)
}

// noMutation returns the ErrNoMutation error of mutation type mt and
// operator tok.
func noMutation(mt mutator.Type, tok token.Token) error {
	return fmt.Errorf("%w: %s %s", ErrNoMutation, mt, tok)
}
