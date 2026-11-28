# ADR 0012: Descartes extreme mutation types

Status: Accepted when #2 merges.

Design guidance: [Descartes preflight](../development/descartes-extreme-mutation-preflight.md).
Engine design: [mutation registry](../development/mutation-registry.md).
User guide: [mutations](../docs/usage/mutations/index.md).

## Context

Issue #2 asks for Descartes extreme mutations and for a `--descartes` flag that runs only these mutations.
An extreme mutation removes the behavior of a full method, as
[Niedermayr, Juergens, and Wagner](https://arxiv.org/abs/1611.07163) describe.
A method with no result loses its statements. A method with a result returns a constant.
The paper and the [Descartes operators](https://stamp-project.github.io/pitest-descartes/mutation-operators.html)
make two constant mutants for each method: `false` and `true`, `0` and `1`, `""` and `"A"`.

Gremlins identifies a mutant by its position and its mutation type.
The ignore and equivalent directives use the same identity.
Thus one type can have only one mutant at one position.

## Decision

Gremlins adds three public mutation types. They are disabled by default.

| Type | Eligible function | Added statement |
| --- | --- | --- |
| `EXTREME_VOID` | No results. The body is not empty and is not only `return`. | `return` |
| `EXTREME_ZERO` | Each result is nilable or basic. | `return` with `nil`, `false`, `0`, or `""` for each result |
| `EXTREME_CONSTANT` | Each result is nilable or basic, and one or more results are basic. | `return` with `nil`, `true`, `1`, or `"A"` for each result |

- A candidate is a function or method declaration with a body. Function literals are not candidates.
- A nilable result type is a pointer, slice, map, channel, function, or interface type, or `error` or `any`.
- A basic result type is `bool`, a predeclared numeric type, or `string`.
- A named type, a qualified type, an array, a struct, or a type parameter is not nilable or basic.
  A function with such a result has no `EXTREME_ZERO` or `EXTREME_CONSTANT` mutant.
- A predeclared identifier counts only when no file of the package declares the name at package level.
  An import of the file, a type parameter, a receiver, a parameter, or a result of the function must not use the name also.
  The same rule applies to `nil`, `true`, and `false` in the added statement.
- The engine reads the package-level names from the regular Go files of the directory with the same package clause.
  When a Go file of the directory is not a regular file, or cannot be read or parsed, the names are not known.
  Then no predeclared identifier counts, and the run continues without an error.
- Gremlins does not create a mutant when the body is one `return` statement with the same values.
  That mutant is equivalent.

The spec adds the `return` statement before the original statements.
The original statements stay in the source as unreachable code.
Thus each import and each variable stays in use, and the mutated file compiles.
The spec restores the original statement list before `Apply` returns.

The position of the mutant is the `func` keyword.
Reports, logs, and the ignore and equivalent directives use this position.
The coverage position is the first statement of the body. The coverage profile has a block from that statement.
In diff mode, a change on a physical line of the declaration selects the mutant. A `//line` directive does not change the lines.

`--descartes` and the `unleash.descartes` key select a group.
When true, the run tests the Descartes types and no other type. The per-type settings do not apply.
When false, the per-type settings apply.
`internal/mutator` owns the group membership. One configuration helper gives the effective enablement.

## Reasons

- The user chose three types: void, zero, and constant.
  `EXTREME_ZERO` and `EXTREME_CONSTANT` give the two constants of the paper with one mutant for each type and position.
  The Go zero value replaces the Java `null` and the first constant of the paper.
- Disabled defaults keep the results, scores, and thresholds of current runs.
  autarchy-ai/coldlight and OpenRAE/env-packs depend on them.
- An early return keeps the source valid. A deleted body can leave an unused import, which the compiler rejects.
  The [Go specification](https://go.dev/ref/spec#Terminating_statements) permits statements after a `return`.
  [`go test`](https://pkg.go.dev/cmd/go#hdr-Test_packages) runs a vet subset without the `unreachable` check.
- The result kind comes from the syntax and the scope rules of the
  [Go specification](https://go.dev/ref/spec#Declarations_and_scope).
  The scope rule is exact for the accepted forms, and it needs no types.
  Thus it also works in a package with type errors, as the syntax checks of
  [ADR 0011](0011-mutant-eligibility-type-information.md) do.
  A named type needs type information to classify, so it is not eligible.
- A file that discovery does not need must not stop the run or reach the output.
  The engine reads no symbolic link, and it does not report the parser errors of other files.
- The `func` keyword is a stable anchor on the declaration line.
  The profile has no block for a signature, so the first statement gives the coverage.
- A selection group does not change the Viper values of the per-type keys.

## Consequences

- The function-level specs use the existing registry, mutant, executor, status, and report paths.
- A later operator, for example an empty slice, needs one new type and one spec.
- `EXTREME_ZERO` and `EXTREME_CONSTANT` do not apply to a function that returns a named or struct type.
- A test can see the line numbers of the mutated file move by one line after the added statement.
- An import without a name gets the last element of its path as its name, or the element before a major version element.
  A package whose name is a predeclared identifier and is not that element can give a `NOT VIABLE` mutant.
- The snippets of a mutant are at the opening brace of the body, so they show the added statement.

## Security

| Asset | Threat | Control | Verification |
| --- | --- | --- | --- |
| Shared AST | An added statement stays after `Apply`. | The spec restores the original statement list. `Apply` calls the restore function before it releases the file lock. | Spec restoration tests. |
| Result integrity | A wrong result kind makes a mutant that does not compile. | The scope rule rejects shadowed predeclared names. Unknown types are not eligible. | Classification tests and a type check of the mutated files. |
| Source tree | A mutation writes outside the worker copy. | The existing mutant writes only the physical file through an `os.Root`. | Existing confinement tests. |
| Files outside the module | A symbolic link in the package directory makes the name scan read another file and print it in a parser error. | The scan opens only regular files and reports no parser error. | `TestRegressionPackageNamesOfSiblingFiles`. |
