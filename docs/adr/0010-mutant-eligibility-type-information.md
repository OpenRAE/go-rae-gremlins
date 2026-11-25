# ADR 0010: Create only mutants that can build

Status: Accepted when the PR of #222 merges.

Design: [mutation registry](../development/mutation-registry.md#eligibility).
User reference: [mutations](../docs/usage/mutations/index.md).

## Context

Before #222, a spec selected an operator from its token alone.
Some selected mutants could not build, and some mutants had the same edit:

| Mutation type | Operator | Result |
| --- | --- | --- |
| `INVERT_BITWISE` | Unary `&` (address) and unary `^` (complement) | `\|x` is not Go syntax. `&x` changes the type of the value. |
| `ARITHMETIC_BASE` | String `+` | Go has no `-` operator for strings. |
| `INVERT_ASSIGNMENTS` | String `+=` | Go has no `-=` operator for strings. |
| `REMOVE_SELF_ASSIGNMENTS` | `<<=` and `>>=` | The count of a shift can have a type that the left operand cannot hold. |
| `INVERT_LOOPCTRL` | `break` outside a loop, for example in a `switch` | `continue` is valid only in a loop. |
| `INVERT_NEGATIVES` and `ARITHMETIC_BASE` | Unary `-` and binary `-` | Both types made the same `-` to `+` edit. |

A mutant that cannot build costs one compilation and gets the `NOT VIABLE` status.
A duplicate mutant costs one more test run.
The maintainer asked for these mutants to be removed in #222.

## Decision

Each spec checks the syntax and the types of its operator before it selects it:

- `INVERT_NEGATIVES` selects only unary `-`.
- `ARITHMETIC_BASE` selects binary operators and unary `+`. It does not select a `+` whose `-` form does not type-check, for example with strings.
- `INVERT_BITWISE` selects only binary operators.
- `INVERT_ASSIGNMENTS` does not select a `+=` whose `-` form does not type-check.
- `REMOVE_SELF_ASSIGNMENTS` does not select a shift assignment whose count cannot be assigned to the left operand.
- `INVERT_LOOPCTRL` selects a `break` only where a `continue` with the same label is valid.

The `internal/typeinfo` package gets the types.
It loads the scanned packages once with `golang.org/x/tools/go/packages`, after the coverage run.
It uses the build tags and the Go build cache of the run.
It gives a fact for each operator that the specs need: a `-` form that does not type-check, or a shift count that cannot be assigned.
For the `-` form, it asks the type checker to check the changed expression at the operator position.

The approved imports of `.golangci.yml` add `golang.org/x/tools/go/packages`.
The module already requires `golang.org/x/tools`. It adds `golang.org/x/sync` as an indirect requirement.

The engine gives the facts to each operator before the specs check it.
Discovery and equivalent directives use the same checks.

A package with type errors gives no facts. The specs then select its operators as before, and Gremlins logs the package.
A failure to load the packages stops the run with an error.
A file that the host build does not include gives no facts. Its mutants are `SKIPPED`.

## Reasons

- Go type checking is the only reliable way to find operands without `-`. Two names, `a + b`, do not show their types.
- The type checker applies the effective type set of a type parameter, so generic constraints need no custom logic.
- `go/packages` is the standard loader. It applies build tags and module rules as the `go` command does.
- The coverage run has compiled the dependencies into the same build cache, so the load reuses that work.
- A missing fact keeps the earlier behavior. A package that does not type-check cannot cause a new failure.
- The syntax checks need no types, so they also work in a package with type errors.

## Consequences

- Runs make fewer `NOT VIABLE` mutants and no duplicate `-` to `+` mutants. Mutant counts in reports can decrease.
- Unary `+x` to `-x` stays an `ARITHMETIC_BASE` mutant. Unary `-x` to `+x` is only an `INVERT_NEGATIVES` mutant.
- An equivalent directive that selects a removed mutant now selects no mutant. The run stops with an error, so a reviewer must update it.
- The type load adds time to the start of a run. It does not run tests.
- Some mutants still cannot build. For example, a constant expression can divide by zero after the edit.
  An untyped constant shift count can also be too large for the left operand.
