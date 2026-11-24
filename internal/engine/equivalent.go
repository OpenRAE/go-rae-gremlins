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
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
)

const (
	equivalentDirective = "equivalent"
	equivalentSyntax    = "//gremlins:equivalent TYPE[@COLUMN] REASON"
)

// ErrEquivalentAnnotation is the error of a //gremlins:equivalent directive
// comment that is not a valid annotation of exactly one equivalent mutant.
var ErrEquivalentAnnotation = errors.New("invalid equivalent mutant annotation")

// equivalentKey identifies one mutant of a parsed file: the position of its
// operator and its mutation type.
type equivalentKey struct {
	pos token.Pos
	mt  mutator.Type
}

// equivalents maps each annotated mutant of a parsed file to its review
// reason.
type equivalents map[equivalentKey]string

// annotation is one //gremlins:equivalent directive comment.
type annotation struct {
	at      token.Position
	reason  string
	matches []token.Pos
	line    int
	column  int
	mt      mutator.Type
}

// fileEquivalents returns the mutants that the //gremlins:equivalent
// directives dirs of a parsed file annotate. Each directive must select
// exactly one mutant among all the mutation types of the operators of the
// file, enabled or not. fileEquivalents returns the errors of all the invalid
// directives, which wrap ErrEquivalentAnnotation.
func fileEquivalents(set *token.FileSet, file *ast.File, dirs []directive) (equivalents, error) {
	anns, err := parseAnnotations(dirs)
	if err != nil || len(anns) == 0 {
		return nil, err
	}
	matchAnnotations(set, file, anns)

	return bindAnnotations(set, anns)
}

// parseAnnotations parses the arguments of the //gremlins:equivalent
// directives dirs.
func parseAnnotations(dirs []directive) ([]*annotation, error) {
	var anns []*annotation
	var errs []error
	for _, d := range dirs {
		a, err := parseAnnotation(d.args, d.at)
		if err != nil {
			errs = append(errs, err)

			continue
		}
		a.line = d.at.Line
		if !d.trailing {
			a.line++
		}
		anns = append(anns, a)
	}

	return anns, errors.Join(errs...)
}

// parseAnnotation parses the arguments of a //gremlins:equivalent directive
// at the physical position at.
func parseAnnotation(args string, at token.Position) (*annotation, error) {
	selector, reason := splitWord(args)
	if selector == "" {
		return nil, annotationError(at, "missing mutation type")
	}
	if reason == "" {
		return nil, annotationError(at, "missing reason")
	}
	typeName, column, hasColumn := strings.Cut(selector, "@")
	mt, ok := mutationType(typeName)
	if !ok {
		return nil, annotationError(at, "unknown mutation type %q", typeName)
	}
	a := &annotation{at: at, reason: reason, mt: mt}
	if hasColumn {
		col, err := strconv.ParseUint(column, 10, 31)
		if err != nil || col == 0 {
			return nil, annotationError(at, "column %q is not a positive integer", column)
		}
		a.column = int(col)
	}

	return a, nil
}

// matchAnnotations records in each annotation the operators of file that
// are on its line, at its column if it has one, and that can produce its
// mutation type. It compares physical positions.
func matchAnnotations(set *token.FileSet, file *ast.File, anns []*annotation) {
	byLine := make(map[int][]*annotation, len(anns))
	for _, a := range anns {
		byLine[a.line] = append(byLine[a.line], a)
	}
	inspectTokens(file, func(node *NodeToken) bool {
		pos := set.PositionFor(node.TokPos, false)
		for _, a := range byLine[pos.Line] {
			if (a.column == 0 || a.column == pos.Column) && builtins.eligible(node, a.mt) {
				a.matches = append(a.matches, node.TokPos)
			}
		}

		return true
	})
}

// bindAnnotations returns the mutants that anns select. Each annotation must
// select exactly one mutant, which no other annotation selects, at a position
// that no //line directive moves.
func bindAnnotations(set *token.FileSet, anns []*annotation) (equivalents, error) {
	eqs := make(equivalents, len(anns))
	var errs []error
	for _, a := range anns {
		if err := bindingError(set, a, eqs); err != nil {
			errs = append(errs, err)

			continue
		}
		eqs[equivalentKey{pos: a.matches[0], mt: a.mt}] = a.reason
	}

	return eqs, errors.Join(errs...)
}

// bindingError returns the error of an annotation that does not select
// exactly one mutant, that selects a mutant of eqs, or whose mutant a //line
// directive moves. It returns nil for a valid annotation.
func bindingError(set *token.FileSet, a *annotation, eqs equivalents) error {
	if len(a.matches) == 0 {
		return annotationError(a.at, "selects no %s mutant on line %d", a.mt, a.line)
	}
	if len(a.matches) > 1 {
		return annotationError(a.at, "selects %d %s mutants on line %d; add @COLUMN", len(a.matches), a.mt, a.line)
	}
	key := equivalentKey{pos: a.matches[0], mt: a.mt}
	if _, dup := eqs[key]; dup {
		return annotationError(a.at, "selects the same mutant as another annotation")
	}
	if set.Position(key.pos) != set.PositionFor(key.pos, false) {
		return annotationError(a.at, "a //line directive moves the selected mutant")
	}

	return nil
}

// annotationError returns an ErrEquivalentAnnotation error of the directive
// at the physical position at.
func annotationError(at token.Position, format string, args ...any) error {
	return directiveError(at, ErrEquivalentAnnotation, equivalentSyntax, format, args...)
}
