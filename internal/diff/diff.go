// Package diff parses git diff output to identify changed lines for incremental mutation testing.
package diff

import (
	"go/token"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

// FileName represents a file path in a diff.
type FileName string

// Change represents a contiguous range of changed lines in a file.
type Change struct {
	StartLine int
	EndLine   int
}

// Diff maps file names to their list of changes.
type Diff map[FileName][]Change

func newDiff(files []*gitdiff.File) Diff {
	result := map[FileName][]Change{}

	for _, file := range files {
		name, changes := newChanges(file)

		result[name] = changes
	}

	return result
}

func newChanges(file *gitdiff.File) (FileName, []Change) {
	var changes []Change

	for _, fragment := range file.TextFragments {
		if fragment.LinesAdded == 0 {
			continue
		}

		line := int(fragment.NewPosition)
		for _, entry := range fragment.Lines {
			switch entry.Op {
			case gitdiff.OpAdd:
				if len(changes) > 0 && changes[len(changes)-1].EndLine == line-1 {
					changes[len(changes)-1].EndLine = line
				} else {
					changes = append(changes, Change{StartLine: line, EndLine: line})
				}
				line++
			case gitdiff.OpContext:
				line++
			case gitdiff.OpDelete:
				// Deleted lines do not advance the new file's position.
			}
		}
	}

	return FileName(file.NewName), changes
}

// IsChanged returns true if the given position is within a changed region.
// A nil diff disables filtering; an empty, non-nil diff selects no positions.
func (d Diff) IsChanged(pos token.Position) bool {
	if d == nil {
		return true
	}

	fileDiff := d[FileName(pos.Filename)]

	for _, change := range fileDiff {
		if pos.Line >= change.StartLine && pos.Line <= change.EndLine {
			return true
		}
	}

	return false
}
