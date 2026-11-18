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
	"go/ast"
	"go/token"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
)

const (
	directivePrefix     = "//gremlins:"
	equivalentDirective = "equivalent"
	equivalentSyntax    = "//gremlins:equivalent TYPE[@COLUMN] REASON"
)

// ErrEquivalentAnnotation is the error of a //gremlins: directive comment
// that is not a valid annotation of exactly one equivalent mutant.
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
// directives of a parsed file annotate. src is the source of the file. Each
// directive must select exactly one mutant among all the mutation types of
// the operators of the file, enabled or not. fileEquivalents returns the
// errors of all the invalid directives, which wrap ErrEquivalentAnnotation.
func fileEquivalents(set *token.FileSet, file *ast.File, src []byte) (equivalents, error) {
	anns, err := parseAnnotations(set, file, src)
	if err != nil || len(anns) == 0 {
		return nil, err
	}
	matchAnnotations(set, file, anns)

	return bindAnnotations(set, anns)
}

// parseAnnotations parses the //gremlins: directive comments of file.
func parseAnnotations(set *token.FileSet, file *ast.File, src []byte) ([]*annotation, error) {
	var anns []*annotation
	var errs []error
	for _, group := range file.Comments {
		for _, c := range group.List {
			if !strings.HasPrefix(c.Text, directivePrefix) {
				continue
			}
			at := set.PositionFor(c.Pos(), false)
			a, err := parseAnnotation(c.Text, at)
			if err != nil {
				errs = append(errs, err)

				continue
			}
			a.line = at.Line
			if !codeBefore(src, at) {
				a.line++
			}
			anns = append(anns, a)
		}
	}

	return anns, errors.Join(errs...)
}

// parseAnnotation parses the text of a //gremlins: directive comment at the
// physical position at.
func parseAnnotation(text string, at token.Position) (*annotation, error) {
	name, args := splitWord(strings.TrimPrefix(text, directivePrefix))
	if name != equivalentDirective {
		return nil, annotationError(at, "unknown directive %q", name)
	}
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
	ast.Inspect(file, func(n ast.Node) bool {
		node, ok := NewTokenNode(n)
		if !ok {
			return true
		}
		pos := set.PositionFor(node.TokPos, false)
		for _, a := range byLine[pos.Line] {
			if (a.column == 0 || a.column == pos.Column) && slices.Contains(TokenMutantType[node.Tok()], a.mt) {
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

// codeBefore reports whether the line of the physical position at has
// source code before at.
func codeBefore(src []byte, at token.Position) bool {
	lineStart := at.Offset - (at.Column - 1)

	return strings.TrimSpace(string(src[lineStart:at.Offset])) != ""
}

// splitWord returns the first word of s and the rest of s, without the
// surrounding white space.
func splitWord(s string) (string, string) {
	s = strings.TrimSpace(s)
	end := strings.IndexFunc(s, unicode.IsSpace)
	if end < 0 {
		return s, ""
	}

	return s[:end], strings.TrimSpace(s[end:])
}

// mutationType returns the mutation type with the name that Type.String
// returns.
func mutationType(name string) (mutator.Type, bool) {
	for _, mt := range mutator.Types {
		if mt.String() == name {
			return mt, true
		}
	}

	return 0, false
}

// annotationError returns an ErrEquivalentAnnotation error of the directive
// at the physical position at. It quotes the position, because the file name
// comes from the module.
func annotationError(at token.Position, format string, args ...any) error {
	detail := fmt.Sprintf(format, args...)

	return fmt.Errorf("%q: %w: %s; want %s", at.String(), ErrEquivalentAnnotation, detail, equivalentSyntax)
}
