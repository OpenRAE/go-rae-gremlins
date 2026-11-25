# Coverage

This page explains how Gremlins uses the coverage of the tests to select the mutants that it tests.

## Covered mutants

Before it tests the mutants, Gremlins runs the tests of the module once with a Go coverage profile.
Gremlins tests only the mutants in code that the tests ran.
A test cannot detect a change in code that it does not run, so the test of such a mutant gives no information.
Gremlins reports these mutants as `NOT COVERED` and does not test them.

The mutant coverage is the part of the mutants that the tests ran.
See [`--threshold-mcover`](commands/unleash/index.md#threshold-mutant-coverage) for its calculation.

## Switch and select clauses

Go coverage profiles do not record the expressions of `switch` and `select` clauses.
Gremlins finds these expressions from the clause bodies that the tests ran:

- For each case body that ran, Gremlins finds the entry case. It moves back while the previous case body ran and ends
  with `fallthrough`.
- The first expression of the entry case is covered. All expressions of the cases before it are covered.
- All case expressions are covered when the `default` body ran and the previous case body did not fall through to it.
- The channel and value expressions of a `select` statement are covered when one of its clause bodies ran.

## Code that coverage cannot show

Go coverage profiles cannot show some code. Gremlins reports the mutants in this code as `SKIPPED`:

- Package-level constant and type declarations, and the signatures of functions, methods, and function literals. The
  compiler evaluates them, so the profile has no blocks for them.
- Files that the file name or the build constraints exclude from the host build with the configured tags. For example,
  a `_windows.go` file on Linux.

## Package-level variables

Go coverage profiles also have no blocks for the initializers of package-level variables. Go runs these initializers
when it initializes the package, before any other code of the package:

- An initializer is covered when the tests ran other code of the same package.
- Gremlins reports the mutants of an initializer as `SKIPPED` when the tests ran no code of the package.
- The body of a function literal in an initializer has its own blocks in the profile.
