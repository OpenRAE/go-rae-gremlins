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
	"strings"
	"unicode"

	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
)

const directivePrefix = "//gremlins:"

// ErrDirective is the error of a //gremlins: directive comment with an
// unknown name.
var ErrDirective = errors.New("invalid gremlins directive")

// directive is one //gremlins: directive comment of a parsed file.
type directive struct {
	group    *ast.CommentGroup
	name     string
	args     string
	at       token.Position
	trailing bool
}

// fileDirectives holds the bound directives of a parsed file.
type fileDirectives struct {
	equivalents equivalents
	ignores     ignores
}

// bindDirectives parses and binds the //gremlins: directives of a parsed
// file. src is the source of the file, and scope holds the operand facts of
// its operators and the package-level names of its package. It returns the
// errors of all the invalid directives.
func bindDirectives(set *token.FileSet, file *ast.File, src []byte, scope fileScope) (fileDirectives, error) {
	byName, err := parseDirectives(set, file, src)
	eqs, eqErr := fileEquivalents(set, file, byName[equivalentDirective], scope)
	igs, igErr := fileIgnores(set, file, byName[ignoreDirective])

	return fileDirectives{equivalents: eqs, ignores: igs}, errors.Join(err, eqErr, igErr)
}

// parseDirectives returns the //gremlins: directive comments of file by
// name. It returns an error for each unknown name.
func parseDirectives(set *token.FileSet, file *ast.File, src []byte) (map[string][]directive, error) {
	byName := make(map[string][]directive)
	var errs []error
	for _, group := range file.Comments {
		for _, c := range group.List {
			if !strings.HasPrefix(c.Text, directivePrefix) {
				continue
			}
			at := set.PositionFor(c.Pos(), false)
			name, args := splitWord(strings.TrimPrefix(c.Text, directivePrefix))
			if name != equivalentDirective && name != ignoreDirective {
				errs = append(errs, directiveError(at, ErrDirective, equivalentSyntax+" or "+ignoreSyntax,
					"unknown directive %q", name))

				continue
			}
			byName[name] = append(byName[name], directive{
				group: group, name: name, args: args, at: at, trailing: codeBefore(src, at),
			})
		}
	}

	return byName, errors.Join(errs...)
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

// directiveError returns an error that wraps sentinel for the directive at
// the physical position at, and that names the wanted syntax. It quotes the
// position, because the file name comes from the module.
func directiveError(at token.Position, sentinel error, syntax, format string, args ...any) error {
	detail := fmt.Sprintf(format, args...)

	return fmt.Errorf("%q: %w: %s; want %s", at.String(), sentinel, detail, syntax)
}
