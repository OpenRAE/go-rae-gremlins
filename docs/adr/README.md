# Architecture decision records

This directory holds the architecture decision records (ADRs) of go-rae-gremlins.
This index lists the ADRs of both ADR directories and their status on 2026-11-27.
Each ADR keeps its original status line and rationale.
A current-context note in an ADR records later changes to its facts.

## docs/adr

| ADR | Decision | Status |
| --- | --- | --- |
| [0001](0001-module-path.md) | Use the module path of the fork | Accepted. The change for #64 merged. |
| [0002](0002-binary-name-and-versions.md) | Keep the binary name and use semantic versions from release-please | Accepted. The change for #64 merged. |
| [0003](0003-upstream-changes.md) | Take upstream changes by cherry-pick | Accepted. The change for #64 merged. |
| [0004](0004-architecture-model-checks.md) | One architecture model for repository checks and design review | Accepted. The change for #76 merged. |
| [0005](0005-static-lint-parser.md) | Keep the lint declaration parser independent of application code | Accepted. The change for #77 merged. |
| [0006](0006-bounded-memory-for-mutation-runs.md) | Bound the memory of a mutation run with a cgroup-v2 runner | Accepted. The change for #127 merged. Its current-context note records the later per-mutant budget. |
| [0007](0007-dev-merge-queue.md) | Merge pull requests to dev through a merge queue | Accepted. The change for #151 merged. |
| [0008](0008-equivalent-mutant-annotations.md) | Annotate one equivalent mutant in the source | Accepted. The change for #201 merged. Amended by 0009, ignore mutant directives. |
| [0009](0009-ignore-mutant-directives.md) | Ignore mutants with a source directive | Accepted. The change for #9 merged. |
| [0009](0009-temporary-build-cache.md) | Use a temporary Go build cache for each run | Accepted. The pull request for #8 merged. |
| [0010](0010-maintainer-integrations-in-the-canonical-repository.md) | Run maintainer integrations only in the canonical repository | Accepted. The change for #231 merged. |
| [0011](0011-mutant-eligibility-type-information.md) | Create only mutants that can build | Accepted. [PR #234](https://github.com/OpenRAE/go-rae-gremlins/pull/234) delivered the change for #222. |
| [0012](0012-descartes-extreme-mutation.md) | Add the Descartes extreme mutation types | Accepted when #2 merges. |

Two ADRs use the number 0009.
Identify each of them by its title and file name.
The numbers do not change, because other documents link to both files.

## architecture/adrs

| ADR | Decision | Status |
| --- | --- | --- |
| [ADR-001](../../architecture/adrs/001-optional-maintainer-tooling.md) | Optional maintainer tooling | Accepted. Its current-context note records the later module path, base branch, and checks. |

ADR-001 uses a separate numbering from `docs/adr`.
It is not the same decision as ADR 0001.
