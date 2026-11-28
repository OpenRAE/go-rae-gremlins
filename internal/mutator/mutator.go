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

// Package mutator provides mutation types, statuses, and interfaces for mutation testing.
package mutator

import "go/token"

// Status represents the status of a given TokenMutant.
//
//   - NotCovered means that a TokenMutant has been identified, but is not covered
//     by tests.
//   - Runnable means that a TokenMutant has been identified and is covered by tests,
//     which means it can be executed.
//   - Lived means that the TokenMutant has been tested, but the tests did pass, which
//     means the test suite is not effective in catching it.
//   - Killed means that the TokenMutant has been tested and the tests failed, which
//     means they are effective in covering this regression.
//   - MemoryError means that the test run of the TokenMutant reached its memory
//     limit, so the Go runtime stopped it.
//   - Equivalent means that a reviewed annotation marks the TokenMutant as
//     equivalent: it does not change the observable behavior, so a test
//     cannot kill it. Gremlins does not run its tests.
type Status int

// Currently supported MutantStatus.
const (
	NotCovered Status = iota
	Runnable
	Skipped
	Lived
	Killed
	NotViable
	TimedOut
	MemoryError
	Equivalent
)

func (ms Status) String() string {
	switch ms {
	case NotCovered:
		return "NOT COVERED"
	case Runnable:
		return "RUNNABLE"
	case Skipped:
		return "SKIPPED"
	case Lived:
		return "LIVED"
	case Killed:
		return "KILLED"
	case NotViable:
		return "NOT VIABLE"
	case TimedOut:
		return "TIMED OUT"
	case MemoryError:
		return "MEMORY ERROR"
	case Equivalent:
		return "EQUIVALENT"
	default:
		panic("this should not happen")
	}
}

// Type represents the category of the TokenMutant.
//
// A single token.Token can be mutated in various ways depending on the
// specific mutation being tested.
// For example `<` can be mutated to `<=` in case of ConditionalsBoundary
// or `>=` in case of ConditionalsNegation.
type Type int

// The currently supported Type in Gremlins.
const (
	ArithmeticBase Type = iota
	ConditionalsBoundary
	ConditionalsNegation
	IncrementDecrement
	InvertAssignments
	InvertBitwise
	InvertBitwiseAssignments
	InvertLogical
	InvertLoopCtrl
	InvertNegatives
	RemoveSelfAssignments
	ExtremeVoid
	ExtremeZero
	ExtremeConstant
)

// Types allows to iterate over Type.
var Types = []Type{
	ArithmeticBase,
	ConditionalsBoundary,
	ConditionalsNegation,
	InvertAssignments,
	InvertBitwise,
	InvertBitwiseAssignments,
	IncrementDecrement,
	InvertLogical,
	InvertLoopCtrl,
	InvertNegatives,
	RemoveSelfAssignments,
	ExtremeVoid,
	ExtremeZero,
	ExtremeConstant,
}

func (mt Type) String() string {
	switch mt {
	case ConditionalsBoundary:
		return "CONDITIONALS_BOUNDARY"
	case ConditionalsNegation:
		return "CONDITIONALS_NEGATION"
	case IncrementDecrement:
		return "INCREMENT_DECREMENT"
	case InvertLogical:
		return "INVERT_LOGICAL"
	case InvertNegatives:
		return "INVERT_NEGATIVES"
	case ArithmeticBase:
		return "ARITHMETIC_BASE"
	case InvertLoopCtrl:
		return "INVERT_LOOPCTRL"
	case InvertAssignments:
		return "INVERT_ASSIGNMENTS"
	case InvertBitwise:
		return "INVERT_BITWISE"
	case InvertBitwiseAssignments:
		return "INVERT_BWASSIGN"
	case RemoveSelfAssignments:
		return "REMOVE_SELF_ASSIGNMENTS"
	case ExtremeVoid:
		return "EXTREME_VOID"
	case ExtremeZero:
		return "EXTREME_ZERO"
	case ExtremeConstant:
		return "EXTREME_CONSTANT"

	default:
		panic("this should not happen")
	}
}

// IsDescartes reports whether mt is a Descartes extreme mutation type. An
// extreme mutation removes the behavior of a full function. The unleash
// descartes setting runs only these types.
func (mt Type) IsDescartes() bool {
	switch mt {
	case ExtremeVoid, ExtremeZero, ExtremeConstant:
		return true
	case ArithmeticBase, ConditionalsBoundary, ConditionalsNegation, IncrementDecrement, InvertAssignments,
		InvertBitwise, InvertBitwiseAssignments, InvertLogical, InvertLoopCtrl, InvertNegatives, RemoveSelfAssignments:
		return false
	default:
		return false
	}
}

// Mutator represents a possible mutation of the source code.
type Mutator interface {
	// Type returns the Type of the Mutator.
	Type() Type

	// SetType sets the Type of the Mutator.
	SetType(mt Type)

	// Status returns the Status of the Mutator.
	Status() Status

	// SetStatus sets the Status of the Mutator.
	SetStatus(s Status)

	// Position returns the token.Position for the Mutator.
	// token.Position consumes more space than token.Pos, and in the future
	// we can consider a refactoring to remove its use and only use Mutator.Pos.
	Position() token.Position

	// Pos returns the token.Pos of the Mutator.
	Pos() token.Pos

	// Pkg returns the package where the Mutator is fount.
	Pkg() string

	// SetWorkdir sets the working directory which contains the source code on
	// which the Mutator will apply its mutations.
	SetWorkdir(p string)

	// Workdir returns the current working dir in which the Mutator will apply its mutations
	Workdir() string

	// Apply applies the mutation on the actual source code.
	Apply() error

	// Rollback removes the mutation from the source code and sets it back to
	// its original status.
	Rollback() error

	// OrigSnippet returns the original code snippet around the mutation point.
	OrigSnippet() []byte

	// MutatedSnippet returns the mutated code snippet around the mutation point.
	MutatedSnippet() []byte

	// EquivalentReason returns the review reason of the annotation that marks
	// the Mutator as equivalent, or an empty string when no annotation marks it.
	EquivalentReason() string
}
