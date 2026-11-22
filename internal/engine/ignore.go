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
	"slices"
	"strings"

	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
)

const (
	ignoreDirective = "ignore"
	ignoreSyntax    = "//gremlins:ignore [TYPE[,TYPE...]]"
)

// ErrIgnoreDirective is the error of an invalid //gremlins:ignore directive
// comment.
var ErrIgnoreDirective = errors.New("invalid ignore directive")

// ignoreScope is the source range [start, end) and the mutation types that
// one //gremlins:ignore directive ignores. A nil types ignores all the types.
type ignoreScope struct {
	types []mutator.Type
	start token.Pos
	end   token.Pos
}

// ignores are the scopes of the //gremlins:ignore directives of a parsed
// file.
type ignores []ignoreScope

// ignored reports whether a scope ignores the mutant of type mt at pos.
func (igs ignores) ignored(pos token.Pos, mt mutator.Type) bool {
	for _, s := range igs {
		if s.start <= pos && pos < s.end && (s.types == nil || slices.Contains(s.types, mt)) {
			return true
		}
	}

	return false
}

// nextLineScope is the scope of a //gremlins:ignore directive alone on its
// line, before it binds to the code of the next line.
type nextLineScope struct {
	scope ignoreScope
	at    token.Position
}

// fileIgnores returns the scopes of the //gremlins:ignore directives dirs of
// a parsed file. A directive after code ignores its physical line. A
// directive alone on its line ignores the syntax nodes that start on the
// physical line after its comment group. Before the package clause, that is
// the file node. fileIgnores returns the errors of all the invalid
// directives, which wrap ErrIgnoreDirective.
func fileIgnores(set *token.FileSet, file *ast.File, dirs []directive) (ignores, error) {
	tf := set.File(file.Pos())
	var igs ignores
	var errs []error
	var pending []*nextLineScope
	byLine := make(map[int][]*ignoreScope)
	for _, d := range dirs {
		types, err := parseIgnoreTypes(d.args, d.at)
		if err != nil {
			errs = append(errs, err)

			continue
		}
		if d.trailing {
			start, end := lineRange(tf, d.at.Line)
			igs = append(igs, ignoreScope{types: types, start: start, end: end})

			continue
		}
		p := &nextLineScope{scope: ignoreScope{types: types}, at: d.at}
		line := tf.PositionFor(d.group.End(), false).Line + 1
		byLine[line] = append(byLine[line], &p.scope)
		pending = append(pending, p)
	}
	bindNextLines(tf, file, byLine)
	for _, p := range pending {
		if !p.scope.start.IsValid() {
			errs = append(errs, ignoreError(p.at, "no code starts on the line after the comment"))

			continue
		}
		igs = append(igs, p.scope)
	}

	return igs, errors.Join(errs...)
}

// parseIgnoreTypes parses the comma-separated mutation types of a
// //gremlins:ignore directive at the physical position at. It returns nil
// for no arguments, which ignores all the types.
func parseIgnoreTypes(args string, at token.Position) ([]mutator.Type, error) {
	if args == "" {
		return nil, nil
	}
	var types []mutator.Type
	for name := range strings.SplitSeq(args, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, ignoreError(at, "empty mutation type")
		}
		mt, ok := mutationType(name)
		if !ok {
			return nil, ignoreError(at, "unknown mutation type %q", name)
		}
		types = append(types, mt)
	}

	return types, nil
}

// bindNextLines sets the range of each scope of next to the syntax nodes of
// file that start on its physical line, from the first start to the last
// end. A scope with no such node keeps an invalid start.
func bindNextLines(tf *token.File, file *ast.File, next map[int][]*ignoreScope) {
	if len(next) == 0 {
		return
	}
	ast.Inspect(file, func(n ast.Node) bool {
		if _, isComment := n.(*ast.CommentGroup); n == nil || isComment {
			return false
		}
		for _, s := range next[tf.PositionFor(n.Pos(), false).Line] {
			if !s.start.IsValid() || n.Pos() < s.start {
				s.start = n.Pos()
			}
			s.end = max(s.end, n.End())
		}

		return true
	})
}

// lineRange returns the range [start, end) of the physical line of tf.
func lineRange(tf *token.File, line int) (token.Pos, token.Pos) {
	end := token.Pos(tf.Base() + tf.Size())
	if line < tf.LineCount() {
		end = tf.LineStart(line + 1)
	}

	return tf.LineStart(line), end
}

// ignoreError returns an ErrIgnoreDirective error of the directive at the
// physical position at.
func ignoreError(at token.Position, format string, args ...any) error {
	return directiveError(at, ErrIgnoreDirective, ignoreSyntax, format, args...)
}
