# Mutation registry: architecture preflight for issue 20

This explanation sets boundaries for the Stage 4 refactor. It is not an implementation plan.
The supplied issue is the delivery contract. Source inspection used revision `cc0df46`.

## Current state and scope corrections

The issue describes an older implementation. This checkout has no `ExprMutator` or separate `InvertLogicalNot` Mutation type.
`TokenMutator` already applies token replacements and unary operand wrapping through one file lifecycle.
`INVERT_LOGICAL` covers `&&`, `||`, and `!` under one existing configuration key, disabled by default.
There are 11 mutation categories in `internal/mutator/mutator.go`, rather than the proposed 12 token categories.

`TokenMutantType` is already a discovery table, not a mutation-selection switch.
Its consumers include `Engine.findMutations` and equivalent-annotation matching.
`tokenMutations` and `operandMutations` describe edits separately.
The useful change is one registered source of eligibility and behavior, with independently testable specs.
Do not manufacture a second mutator just to extract duplication that is absent here.

Retire `TokenMutantType` as an independent catalogue when registry matching replaces it; retain replacement data within the specs.
The no-switch criterion concerns mutation selection, not AST classification, status handling, configuration conversion, or report formatting.

The claimed Stage 1 subtraction fix is absent: `NodeToken` retains no node kind, and `token.SUB` selects both categories.
Unary and binary subtraction therefore both select `INVERT_NEGATIVES` and `ARITHMETIC_BASE` when enabled.
This refactor must characterize that behavior and preserve candidate identities, including existing duplicate effects.
Changing eligibility changes counts, scores, and equivalent selectors. It needs an explicit behavior decision and regression evidence before coding that change.

`.gremlins.yaml` already sets `unleash.threshold.mutant-coverage: 90`; retain it.
That threshold measures mutant coverage, not Go statement coverage or test efficacy.
The historical 87.98% is not a fresh measurement. Implementation must produce fresh evidence for the issue's 90% statement-coverage target.
Use the existing CI coverage profiles and report repository and changed-package scope separately.

## Architecture decisions

Keep the refactor inside `internal/engine`, retaining `internal/mutator` as the consumer contract.
Use private engine components for the requested registry, specs, factory, and shared mutator/file lifecycle.
The proposed directory tree is illustrative; responsibilities do not require a package per pattern or mutation type.
No new package ownership or dependency allowance is needed for this design.
A later package split requires an updated design and reviewed model change before coding.

| Responsibility | Boundary |
| --- | --- |
| Mutation identity | Existing `mutator.Type`, `Types`, names, and statuses. A spec does not define another enum or configuration schema. |
| Spec | Pure eligibility against original syntax, and the reversible AST edit for that type. No configuration, filesystem, execution, reporting, or status policy. |
| Registry | Explicit built-in registration, then immutable discovery shared by normal discovery and annotation matching. No runtime plugin loading or implicit `init` registration. |
| Factory | Bind a matched spec, original position, package, file, and shared file state into `mutator.Mutator`. No AST edits or status decisions during construction. |
| Shared mutator and file operations | One implementation of metadata, snippets, locking, printing, worker-file writes, byte restoration, and error handling. Composition is sufficient; no inheritance hierarchy. |
| Engine | Parsing, directives, build eligibility, coverage, diff filtering, enablement, status assignment, dispatch, and cancellation. |
| Executor | Work directory assignment, test execution, result classification, rollback, discard, and worker cleanup. Preserve its existing contract. |

Use one registered spec per existing Mutation type; one spec can handle several node/operator forms.
Keep ordinary replacements declarative. A small reusable token-spec implementation can serve multiple registrations without eleven boilerplate structs.
The logical spec includes both replacements and structural wrapping; do not invent a new category for the latter.
Keep a concrete registry and factory function unless an actual consumer needs another interface.
The requested spec seam does not justify a second interface duplicating every method of `mutator.Mutator`.

Registration must reject unknown types, nil specs, and duplicate type identities instead of replacing registrations silently.
Validate completeness against `mutator.Types` once at construction, before dispatch; do not introduce per-node catalogue validation.
Preserve candidate ordering for each original operator with an explicit order, rather than map iteration.
Do not infer that order from enum values or the differently ordered `Types` slice.
Discovered `(position, type)` pairs must be unique, even when two categories produce identical source.
Multiple replacements for one type at one position would need an explicit identity-contract decision, including equivalent selectors.

The extension seam is the registered type's eligibility predicate and edit operation.
Retain original AST node context so a future variation can distinguish unary and binary operators without changing engine dispatch.
Keep syntax context separate from the run's `context.Context` and from coverage/status data.
A future whole-expression edit needs an explicit original identity anchor; synthetic nodes cannot become candidate identities.
New categories still require the existing enum, configuration, flags, schema, and report integration; registration alone does not replace these contracts.

## Data flow and lifecycle invariants

```mermaid
flowchart LR
  source[Local module source] --> parse[Parser and directive validation]
  parse --> collect[Registry matches original syntax]
  collect --> policy[Existing filters and status assignment]
  policy --> dispatch[Complete file candidates, then dispatch]
  dispatch --> apply[Lock shared AST, edit, print, restore AST]
  apply --> copy[Worker module copy]
  copy --> tests[Existing bounded go test]
  tests --> restore[Restore bytes or discard worker copy]
  restore --> report[Existing results, Logger, JSON report]
```

`runOnFile` already finishes discovery and status assignment before dispatching any candidate of that file.
Retain that ordering, including directive validation and coverage analysis, before workers can edit the shared AST.
Buffer one file, not the whole repository. Keep cancellation checks during collection and dispatch.
Do not hold an AST lock across channel sends, worker waits, subprocess execution, or disk rollback.

The lock protects shared parsed-file identity, not the worker destination pathname.
Different workers can mutate the same AST while writing different module copies.
All operations on one parsed file must use the same lock, including structural and token edits.
One mutex owned by shared parsed-file state can replace the global filename cache without cross-run lock retention.
Do not give each mutant its own mutex or copy a live mutex with a base value.

Use the discovered source-relative filename for worker writes and the physical source line for snippet extraction.
Source-position remapping is display metadata, never a filesystem path or lock identity.
Preserve existing adjusted report positions separately; equivalent selectors retain their physical-position validation.
Keep these path responsibilities in the shared helper, not individual specs.

Distinguish two restoration lifetimes. The operation restores AST references immediately inside `Apply`, before releasing the file lock.
`mutator.Mutator.Rollback` later restores original bytes in the worker copy after tests.
A private AST operation must not duplicate the disk lifecycle merely because the issue calls both actions `Rollback`.

Operations and original-byte buffers belong to individual mutants; stateless specs can be shared.
Capture restoration state under the lock. Restore exact original references and token metadata on every return path.
If an edit can fail after modifying syntax, its failure path must restore that syntax too.

Retain `go/printer`, original snippets, mutated snippets, original positions, and byte-exact disk restoration.
For `!x`, preserve grouping and one evaluation of `x`; repeated cycles cannot accumulate wrapping.
Preserve comments and physical byte columns used by directives, including existing rejection of moved equivalent positions.
An unsupported requested edit must return an error wrapping `ErrNoMutation`, without writing a mutant or selecting `token.ILLEGAL`.
An unmatched node during discovery is an ordinary empty result, not an execution failure.

Preserve write-failure restoration and joined restoration errors.
Apply or rollback failures discard the worker copy; failed mutants cannot become successful results.

## Cross-cutting layers to reuse

| Layer and canonical incumbent | Required treatment |
| --- | --- |
| Module and paths: `internal/gomodule`, `cmd.modulePath`, `cmd.checkWorkDir` | Preserve module declaration validation, module-root/calling-directory routing, and rejection of temporary roots inside the module, including symlink aliases. |
| Source parser: `engine.parseFile`, `inspectTokens`, standard `go/parser`, `go/ast`, `go/token` | Preserve parsed comments, read/close/parse errors, original positions, AST traversal order, and type-constraint exclusion. No text scanner or second traversal that includes generic type terms. |
| Directives: `directive.go`, `equivalent.go`, `ignore.go` | Share registry eligibility with equivalent matching before enablement or ignore filtering. Retain exact-one matching, duplicates, physical positions, unknown-type errors, reasons, and complete file validation. Invalid equivalents still fail inside ignored scopes. |
| Eligibility and status: `engine.go`, `clausecoverage.go`, `internal/coverage`, `internal/diff`, `internal/exclusion` | Preserve file/directory exclusions, nested-module exclusion, host build tags and suffixes, clause coverage, variable initializers, and package routing. Ignored/disabled mutants are absent; skipped takes precedence over equivalent. |
| Flags, configuration, environment: `cmd/internal/flags`, `configuration.Init`, `Get`, `Validate`, `MutantTypeEnabledKey`, `mutantenabled.go` | Retain boolean types, defaults, flag/config/environment precedence, and `GREMLINS` binding. Specs cannot parse environment values or duplicate validation. The JSON schema is editor guidance; runtime validation remains `Validate`. |
| Configuration shape: `docs/docs/schema/configuration.json`, `cmd/schema*_test.go` | Keep the existing schema and flag parity checks. This refactor needs no new key, flag, default, alias, or schema. |
| Filesystem: shared mutator lifecycle and `workdir.CachedDealer` | Reuse temporary copies, regular-file copy behavior, existing write permissions, discard, and cleanup. Keep source-relative paths distinct from worker paths and report positions. No hard links, source rewrites, arbitrary paths from specs, or alternate scratch tree. |
| Processes and resources: `executor.go`, `timeout.go`, `memory.go`, `internal/process`, `workerpool` | Retain direct argument arrays, package/integration routing, memory admission, adaptive timeout, process-tree cancellation, and waiting for workers. A spec never launches processes. |
| Child environments and cache: coverage, executor, `cmd.runBuildCache` | Preserve inherited runtime environment and the final `GOTMPDIR`/`GOCACHE` overrides. No source, spec text, or credential enters a new argument or environment value. Do not log inherited environment values. |
| Hosted execution: `scripts/run-bounded.sh`, `.github/workflows/gremlins.yml` | Preserve validated aggregate budgets, effective-controller checks, unprivileged execution, environment allowlists, credential-value filtering, cleanup, and fail-closed behavior. No unbounded comparison lane. |
| Errors: `ErrNoMutation`, directive sentinels, `ExecutorResult`, `Engine.Run`, `internal/execution` | Reuse ordinary errors, `%w`, `errors.Is`/`As`, and `errors.Join`. Preserve infrastructure failures versus mutant statuses, cancellation, partial-result failure, and exits 1, 10, and 11. No parallel exception hierarchy or JSON error envelope. |
| Output and persistence: `internal/report`, `report/internal/structure.go`, `internal/log` | Keep JSON fields, names, ordering policy, scoring, reason propagation, snippets, diff formatting, output filters, and silent-mode behavior. No new report DTO, database, telemetry, log destination, or raw-source error dump. Quote new input-derived diagnostic fields. |
| Development policy: `.architecture/model.json`, `tools/architecture`, `scripts/check-design.sh` | Use the strict design decoder and ARCH001–ARCH004, existing package ownership, contract declarations, and shrink-only baseline. The model checker does not prove semantic compatibility or absence of cycles. |
| Verification and supply chain: `Makefile`, `mise.toml`, `.golangci.yml`, `.pre-commit-config.yaml`, `scripts/lint.py`, `scripts/doc-tools.sh` | Use pinned tools, existing lint contexts and baselines, fast secret checks, and explicit-path document checks. No new dependency, provider, hook dispatcher, or copied workflow. |
| CI reports and credentials: `.github/workflows/ci.yml`, `scripts/check-coverage.sh`, `sonar-project.properties` | Preserve Linux/Windows/macOS coverage admission, exact artifact shapes, secret-free candidate execution, and the separate credential-bearing Sonar scan. Do not change scanner inputs to inflate coverage. |

There is no application authentication, authorization, controller, service repository, or database layer in this local CLI flow.
Do not add these layers for naming symmetry. Source compilation and tests retain the operator's permissions and existing Go network behavior.
Work directory copies and resource limits do not sandbox hostile source or prevent its filesystem/network access.
No secret-file access, new service, source transfer, or credential handling is needed for this refactor.

## Evidence obligations and gotchas

Preserve the existing behavior tests while replacing private structure assertions that depend on the old tables.
Reuse table-driven tests, `testing`, `go-cmp`, temporary writable files, executor doubles, and `WithDirFs`.
Compare independent expected candidate sets, edits, positions, types, statuses, and reports; comparing two paths sharing the new registry is insufficient.
Retain all original candidates, including adjacent/nested negations and multiple categories at one operator.
Evidence must cover malformed directives, disabled types, ignored scopes, constraints, build exclusions, diff routing, dry runs, and cancellation.
Baseline coverage executes before discovery, including in dry-run mode; registry validation cannot promise to precede that existing execution.

Retain the existing write-failure/AST-restoration regression, missing-mutation test, original-negation discovery test, and executor discard tests.
Exercise same-file and different-file concurrent edits, repeated cycles, print/write failures, restoration failure, and cancellation without timing-based sleeps.
Verify that remapped source positions cannot change the worker write destination or split the shared AST lock.
Use existing bounded test helpers rather than a new process harness.
Do not copy shared AST pointers into a supposedly independent comparison implementation.
Do not keep the old discovery path selectable in the shipped binary after migration.

Use selected `go test -race ./internal/engine -run '<tests>'` cases locally during implementation.
Full tests, fresh repository statement coverage, and mutation runs belong to existing CI.
The default self-mutation configuration leaves `INVERT_LOGICAL` disabled; a generic green mutation job does not cover its structural edit.
Require focused mutation evidence for matching, edit selection, restoration, and failure propagation within existing CI limits.
Do not name compatibility tests `TestRegression` unless they demonstrate a defect against base code under the existing regression-proof workflow.

The no-performance-regression criterion needs comparable discovery and apply/print measurements, including allocations and candidate counts.
Compare the same source, enabled types, worker count, toolchain, and cache conditions; whole-run timing alone is dominated by compilation and tests.
Avoid repeated whole-file discovery per spec, filesystem reads in predicates, module-wide buffering, or a global lock around different files.
Preserve per-operator candidate order: scheduling order can affect adaptive timeout samples, even when concurrent completion is already nondeterministic.
This preflight establishes no coverage or performance result.

## Non-goals and governing decisions

No new mutation semantics, category, public contract, configuration schema, output destination, execution lane, dependency, or plugin API.
No generic AST framework, service/repository layer, duplicated validation, parallel status machine, or blanket lint/baseline refresh.
The requested extraction must consolidate the existing lifecycle; literal zero repeated lines is not a reason to merge unrelated responsibilities.
Keep byte-exact `testdata` inputs intact except for deliberate fixture changes.
During implementation, update the architecture flow and adding-mutations guide in `CLAUDE.md` to describe the actual registry.
Do not document registration as sufficient for a new public category.

Binding references are ADR 0001 (module identity), ADR 0003 (actual upstream imports), and ADR 0004 (design/model checks).
ADR 0008 and ADR 0009 `ignore-mutant-directives` govern annotation matching and filtering.
ADR 0006 and ADR 0009 `temporary-build-cache` govern existing resource and temporary-state boundaries.
Use filenames to distinguish the two existing ADRs numbered 0009.
The earlier issue-7 guidance against a strategy hierarchy was scoped to one operator; Stage 4 justifies the private spec seam described here.
Existing ADRs cover these boundaries; no new accepted ADR or architecture-policy relaxation is needed.

The contributor guide requires a copied-issue triage disposition before implementation.
The supplied context does not contain that record; this preflight neither invents a disposition nor authorizes an upstream import.
PRs target `dev`, and the user owns merges. This note authorizes no publication or workflow change.

## Preflight verification

The design declaration is [`.architecture/designs/20.json`](../../.architecture/designs/20.json).
It declares the engine change and the future compatible update to the canonical `CLAUDE.md` guidance.
No runtime contract-file change or modeled trust-boundary expansion is proposed.

The explicit design check passed ARCH001, ARCH002, ARCH003, and ARCH004 with Go 1.26.11 through mise.
The model SHA256 was `c28bab810f1497d79aea1fcac11781ad83abf8d2e7b36cb8c78a8e7225243f6b`.
The repository architecture scan passed against local `HEAD` with zero baseline findings.
`make policy` and explicit-path documentation lint passed.

No implementation tests, coverage runs, benchmarks, or mutation runs were executed during preflight.
These checks validate declared boundaries and repository policy; they do not prove runtime correctness, coverage, or performance.
