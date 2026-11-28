# Descartes extreme mutation: architecture preflight for issue 2

This note records the design boundary for issue 2. It does not implement the issue or prescribe an implementation sequence.
The issue is the delivery contract, but it is not yet precise enough for implementation.

## Readiness decision

The preflight found that issue 2 did not define the Go behavior of the mutations.
The maintainer then made these decisions:

- Deliver all of issue 2 in one pull request.
- Add the types `EXTREME_VOID`, `EXTREME_ZERO`, and `EXTREME_CONSTANT`.

[ADR 0012](../adr/0012-descartes-extreme-mutation.md) records the other decisions of this list:

- The eligible Go declarations. This includes functions, methods, function literals, `init`, `main`, empty bodies, and declarations without bodies.
- The result shapes: zero, one, or multiple results, named results, type parameters, and named result types.
- Every replacement value, with one mutant for each type and function.
- The nilable result types that syntax and scope rules can identify.
- The equivalent bodies that do not get a mutant.
- The stable identity and report location of one function-level mutant.
- The `--descartes` precedence over each mutation-type flag, configuration key, and environment value.
- The default state of each new Mutation type when `--descartes` is false.

## Recommended product boundary

Treat each extreme operator as a normal public Mutation type.
It must use the same status, execution, report, directive, and rollback paths as the current types.
Do not add a second Descartes engine or a second result model.

Treat `--descartes` as a selection group, not as a Mutation type.
When true, it should enable only the types in the Descartes group.
When false, the normal per-type settings should apply.
The issue must state whether an explicit per-type setting can override the group.

Keep group membership in one canonical predicate or table beside `mutator.Types`.
Configuration must have one helper that resolves effective type enablement.
The engine must not reproduce group membership or precedence rules.

Use one registered function-level spec for each accepted replacement behavior.
A spec must identify eligible original syntax and provide one exact reversible AST edit.
It must have no configuration, process, filesystem, report, or logging access.

Do not call a function body a token target.
Generalize only the private candidate identity and reversible edit that token and function specs must share.
Keep token indexing for current operators.
Do not replace the registry with a generic AST transformation framework.

One `(physical position, Mutation type)` pair is currently one candidate identity.
If one constant operator creates multiple values at one function, that identity is insufficient.
Use separate public types, or add a reviewed stable variant identity through directives and reports.
Do not create duplicate candidates that reports and equivalent annotations cannot distinguish.

## Function-level identity and selection

A function-level mutant needs two separate source concepts:

- A stable anchor identifies the declaration in directives, logs, and reports.
- A body span decides coverage and changed-code selection.

Do not use one token position for both concepts.
A declaration-name position does not show whether any body statement ran.
It also does not show whether a changed line is inside the body.

Use physical positions for matching, worker paths, snippets, and source selection.
Keep adjusted `//line` positions only as display metadata.
The current moved-position rejection for equivalent annotations must remain.

Function-level candidates must pass the existing directive rules.
Equivalent annotations must still select exactly one candidate, including disabled types.
Ignore directives must use one documented function-level scope rule.
Do not add a Descartes-only annotation parser.

Coverage must establish that the function body ran.
Do not infer body coverage from the signature, the file, or another function in the package.
The coverage profile cannot observe signatures and some compiled declarations.

Diff mode must select the function when an accepted changed range intersects its relevant body span.
Do not test only the report anchor line.
Keep the existing file exclusions, nested-module exclusions, host build constraints, and package routing.

## Go semantic guardrails

Do not classify nilability or scalar types from identifier spelling.
A named type, alias, interface, pointer, slice, map, channel, function, or type parameter can defeat a syntax-only guess.
Do not generate known invalid candidates and rely on `NOT VIABLE` as type validation.

If accepted semantics require type identity, use compiler-derived type information.
Use the current module root, calling directory, build tags, build cache, cancellation, and Go environment rules.
Do not add an independent package loader with different file selection or network behavior.

ADR 0012 does not use type identity. It accepts only the type forms that syntax identifies without type information:
pointer, slice, map, channel, function, and interface types, and predeclared identifiers.
A predeclared identifier counts only when the package and the function do not declare the name.
The [scope rules](https://go.dev/ref/spec#Declarations_and_scope) make this check exact.
A named type, a qualified type, an alias, or a type parameter is not eligible.

The issue must define multiple-result behavior before type loading is designed.
For example, `(T, error)` can combine a constant and `nil` in one return.
That result does not fit an undefined single category.

Replacing the body can make a previously used import unused.
This is a Go-specific difference from bytecode mutation.
An unconditional return before the original statements can remove runtime behavior while preserving compile-time references.
If that representation is selected, the issue must define it as behavior removal rather than source deletion.

The edit must restore the exact original AST references before `Apply` releases the parsed-file lock.
Disk rollback must continue to restore the exact original worker-file bytes after the test.
Do not keep the shared AST changed while a test runs.

Collect all original candidates of one file before dispatching its first mutant.
Never discover a synthetic return expression or a retained unreachable statement as a new candidate.
Keep candidate order explicit because it can affect adaptive timeout samples.

## Cross-cutting layers to reuse

| Layer and canonical incumbent | Required treatment |
| --- | --- |
| Public type inventory: `internal/mutator/mutator.go` | Add only accepted Mutation types. Keep names and group membership canonical here. Preserve existing status and `Mutator` contracts unless a reviewed identity change requires otherwise. |
| Registry and edits: `internal/engine/spec.go`, `specs.go`, `registry.go`, `mutant.go` | Reuse registration validation, per-file locking, AST restoration, printing, snippets, worker writes, byte rollback, and `ErrNoMutation`. Do not add a parallel mutator lifecycle. |
| Discovery and policy: `engine.go`, `clausecoverage.go` | Reuse file walking, build constraints, exclusions, coverage, diff selection, status precedence, cancellation, per-file collection, and package routing. Extend selection for a body span without duplicating policy. |
| Directives: `directive.go`, `equivalent.go`, `ignore.go` | Use the canonical type names and shared candidate catalogue. Preserve strict parsing, physical positions, exact-one equivalent matching, duplicate rejection, reasons, and complete validation before execution. |
| Configuration keys: `internal/configuration/configuration.go` | Add the accepted Boolean group key and reuse `Init`, `Get`, `Validate`, and the `GREMLINS` environment mapping. Keep effective type selection in one helper. |
| Defaults and flags: `mutantenabled.go`, `cmd/unleash.go`, `cmd/internal/flags` | Keep the current flag table, Viper binding, precedence, normalized names, and generated per-type flags. Do not mutate Viper state to implement group selection. |
| Configuration shape: `docs/docs/schema/configuration.json`, `cmd/schema*_test.go` | Keep flag, key, type, default, schema, and reference-example parity. The schema is editor guidance; runtime validation remains `configuration.Validate`. |
| Work directory copy: `internal/engine/workdir`, `parsedFile.inWork` | Write only the physical module-relative file in the worker copy. Retain `os.Root` confinement, full-copy behavior, discard, cleanup, and source-tree read-only behavior. |
| Processes and resources: `executor.go`, `timeout.go`, `memory.go`, `internal/process` | Reuse direct argument arrays, package test routing, adaptive timeouts, memory admission, process-tree cancellation, build-failure classification, and worker shutdown. A spec must never start a process. |
| Errors and exits: ordinary Go errors, `ExecutorResult`, `Engine.Run`, `internal/execution` | Preserve wrapping, joining, cancellation, infrastructure failure, mutant statuses, and exits 1, 10, and 11. Do not add an exception hierarchy or a Descartes error envelope. |
| Logger and reports: `internal/log`, `internal/report` | Use the Logger, silent mode, status filters, snippets, diff output, score rules, and configured JSON file. Add only the accepted type statistics. Do not add telemetry or another output destination. |
| Development policy: `.architecture/model.json`, `tools/architecture`, `Makefile`, hooks, lint, and CI | Declare compatible CLI, configuration, schema, and report changes after semantics are accepted. Preserve the shrink-only baselines, pinned tools, secret checks, and hosted full-test ownership. |

There is no application authentication, authorization, controller, service, repository, or database layer in this CLI flow.
Do not add these layers for naming symmetry.

## Security and runtime boundaries

The Cobra flag and its Viper key must be Boolean.
`configuration.Validate` must reject a value that does not convert to Boolean.
The JSON schema and its parity tests must declare the same type and default.

The environment form will be `GREMLINS_UNLEASH_DESCARTES` if the accepted key is `unleash.descartes`.
This value contains no credential.
No source, replacement value, or inherited environment value belongs in process arguments or logs.

Source parsing must retain `parser.ParseComments`, directive validation, build-tag filtering, and physical path handling.
A `//line` directive must not select a worker path or split the lock identity of one parsed file.

Only `RUNNABLE` mutants can enter the existing apply-and-test path.
The worker copy is not a sandbox.
The mutated module and its tests still run with the operator's permissions and existing Go network behavior.

The existing process arguments, environment inheritance, `GOTMPDIR`, `GOCACHE`, timeouts, memory limits, and process-tree cleanup must remain unchanged.
Type analysis must not introduce a second unbounded command or a different module-download path.

Errors can name quoted local positions, Mutation types, and configuration keys.
They must not dump source files, environment values, credentials, or raw tool output.
Console output must use `internal/log`; machine output must use the current report DTOs.

No secret file, new credential, external service, source upload, database, or persistence format is needed.
The existing configured JSON report file is the only persistent runtime result in scope.

## Compatibility and reporting guardrails

New type flags and configuration keys are additive public contracts.
The group flag is also an additive CLI and configuration contract.
The configuration schema and usage reference must change with them.

The JSON mutation `type` values and `mutator_statistics` fields are downstream contracts.
Add accepted fields without renaming or removing current fields.
Keep `omitempty` behavior and existing score calculations unless the issue explicitly changes them.

Extreme mutants will intentionally change counts, elapsed time, efficacy, mutant coverage, and threshold outcomes.
Do not label these expected result changes as report-shape compatibility.
Exit codes remain unchanged.

The default states need an explicit compatibility decision.
Enabling coarse mutants by default changes every existing run and can change quality-gate results.
Disabling them by default preserves existing runs but requires the group or a type setting to use them.

## Evidence obligations

Use the repository's table-driven tests and independent expected candidates.
Do not compare two code paths that share the new registry or type classifier.

Evidence must cover accepted function shapes, result types, aliases, named types, generics, multiple results, methods, and excluded declarations.
It must also cover stop functions and replacements equal to existing behavior.

Prove the stable anchor, body span, coverage status, diff selection, package routing, build constraints, exclusion, directives, and group precedence.
Cover disabled types and explicit per-type settings under both group states.

Prove valid printed Go, preserved imports, exact AST restoration, byte-exact rollback, snippets, and repeated apply-and-rollback cycles.
Cover print and write failures, discard behavior, cancellation, and same-file concurrent mutants with the race detector.

Prove the additive CLI, configuration, schema, documentation, JSON type, and statistics contracts.
Preserve current status filters, score denominators, threshold exits, and empty-result behavior.

Use focused race tests for changed packages.
Use mutation testing where it can prove the new discovery or restoration behavior.
CI owns full tests, coverage, and mutation runs.

## Gotchas and anti-patterns

- Do not equate Java reference types with Go identifiers or selectors.
- Do not use build failures as the nilability or constant-type classifier.
- Do not identify a function-level mutant only by the first operator in its body.
- Do not use a signature position as proof that the body ran or changed.
- Do not register two indistinguishable replacements at one position and type.
- Do not let `--descartes` rewrite all per-type Viper values.
- Do not add a second enablement table in the command or engine.
- Do not bypass equivalent and ignore directives for coarse mutations.
- Do not remove imports or rewrite raw source text to make a mutant compile.
- Do not hold the AST lock during a test, channel send, or worker wait.
- Do not expose adjusted `//line` filenames to worker file access.
- Do not add a new worker pool, test command, status machine, report DTO, or logging path.
- Do not claim that work directories or resource limits sandbox hostile source.
- Do not reformat byte-exact `testdata` files.
- Do not import an upstream branch. ADR 0003 governs an approved upstream cherry-pick.

## Non-goals

No plugin system, general mutation engine interface, generic AST rewrite framework, new package, or third-party dependency.
No new test runner, process argument, environment forwarding rule, network destination, persistence store, telemetry, or hosted service.
No authentication or authorization layer.
No change to existing Mutation semantics, statuses, score formulas, thresholds, exit codes, or report destinations.
No automatic stop-function heuristic beyond behavior that the issue explicitly accepts.
No support for extra Descartes operators such as empty collections, optionals, arguments, or receiver returns.

The registered function-spec seam must permit such a later public Mutation type without another execution path.

## Binding decisions and sources

ADR 0001 governs the module identity.
ADR 0003 governs actual upstream imports.
ADR 0004 governs the design declaration and architecture checks.
ADR 0006 governs hosted resource limits.
ADR 0008 governs equivalent annotations.

ADR 0009, `ignore-mutant-directives`, governs ignore scopes.
ADR 0009, `temporary-build-cache`, governs the run cache.

Use file names to distinguish the two ADRs numbered 0009.

The current registry design is [Mutation registry](mutation-registry.md).
The configuration contract is described in [Configuration schema preflight](configuration-schema-preflight.md).
The external semantic references are the
[original paper](https://arxiv.org/abs/1611.07163),
[PIT extreme-mutation description](https://pitest.org/quickstart/advanced/#activating-extreme-mutation-pit-descartes),
and [Descartes operator reference](https://stamp-project.github.io/pitest-descartes/mutation-operators.html).

This review inspected repository revision `a4b037df8a8b5f43d7299b7962c4886a425fb016`.
No source, configuration, schema, report, or architecture declaration was changed.
