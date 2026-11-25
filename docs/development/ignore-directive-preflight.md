# Ignore directive: architecture preflight

> Current context (2026-11-24): This is a historical record of issue #9 at its reviewed revision.
> Current decision and use: [ADR 0009, ignore directives](../adr/0009-ignore-mutant-directives.md) and [ignore mutants](../docs/usage/ignore-mutants.md).

Issue #9 is the contract. [ADR 0009](../adr/0009-ignore-mutant-directives.md) records the syntax and scope decision.
This note defines the design boundaries. It does not prescribe implementation steps.
Sources were inspected at revision `f5a9bff`.

## Meaning and ownership

`//gremlins:ignore` removes mutation candidates during engine discovery.
It does not assert equivalence, mark a mutant killed, or change Go test coverage.
Ignored candidates do not go to the worker stream, the reports, the counts, or the thresholds.
The `SKIPPED` and `EQUIVALENT` statuses and the report structure do not change.

The engine owns directive parsing, scope binding, and candidate filtering.
`internal/exclusion` stays the configuration-based file path filter. Do not give it AST responsibilities.
No new package, dependency, status, configuration key, or report field is necessary.
[ADR 0008](../adr/0008-equivalent-mutant-annotations.md) continues to govern equivalent annotations.

## Directive parsing

One parser reads the `//gremlins:` comments of a file and sends each directive to its binder.
Read each raw `ast.Comment.Text`. Do not read normalized documentation text or use a source-wide regular expression.
Strings, block comments, and prose that mention the directive are not directives.
An unknown directive name stops the run. Its error does not wrap the equivalent annotation error.

Type names come from `mutator.Types` and `Type.String()`. Do not keep a second name list.
A new mutation type then works in the directive without a directive change.
An empty explicit list entry must never become an all-types ignore.

## Scope binding

Each scope is a physical `token.Pos` range and an optional type set:

| Placement | Range |
| --- | --- |
| After code | The physical line of the directive, from `token.File.LineStart`. |
| Alone on its line | From the first to the last end of the syntax nodes that start on the line after the comment group. |

The `ast.File` node starts at the `package` keyword. So a directive before the `package` clause covers the file.
This rule needs no special case for files, functions, or blocks.
`gofmt` moves a directive to the end of a documentation comment, and the comment group stays one group.
A blank line ends a comment group. So a blank line between a directive and its code stops the run.

Use token positions and `PositionFor(pos, false)`. A `//line` directive must not move a scope.
Bind all scopes in one AST pass before the first mutant of the file goes to the workers.
Executors change the shared AST, so discovery must not read the AST after that.
Overlapping scopes combine. There is no re-enable directive.

## Filtering

Filter each operator position and type pair. Ignoring one type must keep the other type of the same operator.
Filter in the mutant creation of the engine, together with the configured type enablement.
Equivalent annotations bind against the original catalogue before the filter.
Thus an ignore scope cannot hide an invalid equivalent annotation or make a valid one stale.

Keep the discovery exclusions: tests, ignored names, `testdata`, vendor trees, nested modules, and `--exclude-files`.
Do not scan files that discovery excludes.

## Cross-cutting layers

| Layer and canonical owner | Treatment |
| --- | --- |
| Source parsing: `engine.go`, `equivalent.go`, `node.go`, `mappings.go` | Reuse `parseFile`, `inspectTokens`, and `TokenMutantType`. An invalid directive fails discovery. |
| Configuration: `internal/configuration` | No change. A directive cannot set configuration or enable a disabled type. |
| Configuration schema: `docs/docs/schema/configuration.json` | No change. A source directive is not a configuration key. |
| Files and processes: `workdir`, `executor.go`, `internal/process` | No change. No directive value goes into a path, an argument, an environment value, or a command. |
| Errors and output: `Engine.Run`, `internal/log`, `internal/execution` | Return wrapped and joined errors. Quote the directive position. Invalid input exits with code 1. |
| Results: `internal/report` | No change. Ignored candidates have no report data. |

There is no authentication, network, storage, or service layer in this path.
Source review is the authority for each directive. The tool cannot prove that a directive is correct.
The directive does not stop the coverage run, compilation, or the copy of the file into work directories.

## Evidence

Use table-driven engine tests with `WithDirFs` and the dry run.
Prove each scope of the issue, the type list, and one type of a two-type operator.
Prove sibling lines and statements outside the scope keep their mutants.
Prove the `//line` case, a disabled type, and an equivalent annotation in an ignored scope.
Prove each error: unknown type, configuration name, empty entry, and a standalone directive without code.

## Non-goals

No new status, flag, configuration key, schema field, report field, or exit code.
No `//nomutant` spelling, configuration-style type names, mandatory reason, re-enable directive, or package-wide ignore.

[ADR 0004](../adr/0004-architecture-model-checks.md) governs [design 9](../../.architecture/designs/9.json).
The design uses only the existing engine package and dependencies. The model and baseline do not change.
