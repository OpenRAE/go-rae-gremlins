# Optional maintainer tooling: architecture preflight for issue #231

This preflight inspected revision `59baad1` against the supplied issue #231 contract.
It adds design guidance only. It does not change a workflow, contributor instruction, or application behavior.

The [existing optional-tooling decision](../../architecture/adrs/001-optional-maintainer-tooling.md) already establishes the required application boundary.
No runtime abstraction is needed.
The implementation records the repository condition in [ADR 0010](../adr/0010-maintainer-integrations-in-the-canonical-repository.md).
Its old module and branch statements yield to [ADR 0001](../adr/0001-module-path.md) and [ADR 0007](../adr/0007-dev-merge-queue.md).
[ADR 0004](../adr/0004-architecture-model-checks.md) governs executable architecture checks.

## Boundary and decisions

Public development uses ordinary Git, Go, Make, pre-commit, and the repository-owned architecture, lint, and documentation tools.
It requires no private package, workflow skill, MCP registration, provider credential, or Ground Control installation.
The maintainer integration remains available in `OpenRAE/go-rae-gremlins` through its existing configuration and workflows.

The repository running the workflow and the repository supplying a PR are separate facts.
Use the exact executing repository identity for maintainer-only applicability.
A same-repository PR inside a fork must not qualify for the OpenRAE integration.
A merged external PR into OpenRAE must retain its existing trusted dispatcher path.
Neither copied configuration, an organization name, an actor name, nor the presence of a secret proves applicability.

Apply the repository restriction to the entire event condition, including manual dispatch.
Private acquisition, installation, finalization, and dispatch must be inapplicable in other repositories before their side effects begin.
Native build, tests, architecture, lint, documentation, and local secret checks remain active there.
Do not restrict the whole CI workflow to OpenRAE.

GitHub supports repository-specific job conditions and reports skipped jobs separately from executed checks.
The aggregate gate must inspect dependency results explicitly, because dependency skips propagate.
See [job conditions](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-jobs-with-conditions) and [job dependencies](https://docs.github.com/en/actions/how-tos/write-workflows/choose-what-workflows-do/use-jobs).

## Repository surfaces and canonical owners

| Surface | Current coupling and required boundary |
| --- | --- |
| `.github/workflows/ci.yml`: transport jobs and `PR Gate` | Foreign-PR filtering does not exclude same-repository PRs or merge groups inside forks. Retain successful scope selection and explicit selected/unselected results. An inapplicable integration must not read the private release tuple or acquire the package. |
| `.github/workflows/ground-control-phase-e.yml` | Its event guard currently admits same-repository fork deliveries and dispatches. Preserve the existing merged-state, permitted-base, immutable-SHA, default-branch, and provenance checks behind repository applicability. |
| `.github/workflows/ground-control-fork-finalization.yml` | This handles a fork PR merged into the maintainer repository. It must not dispatch private finalization from a fork of this repository. Preserve its fixed workflow target and no-checkout design. |
| `.github/workflows/ci.yml`: Sonar jobs and live queue-policy step | Same-repository fork events currently qualify for Sonar. The queue-policy Make target reads OpenRAE rules. Neither the provider credential nor OpenRAE branch administration is a prerequisite for fork CI. |
| `.github/workflows/ggshield.yml` and `maintainer-gate.yml` | They assume a GitGuardian credential and approval by `Brad-Edwards`. These assumptions also block an unconfigured fork. Restrict existing owner-specific enforcement without weakening native secret checks or OpenRAE enforcement. |
| `Makefile`, `.github/scripts/merge_queue_policy.py`, and `test_merge_queue_policy.py` | Reuse existing commands and the policy parser. `--repository` is already the API-target parameter. Merely changing its value still imposes OpenRAE's queue topology on forks. Applicability belongs at invocation. |
| `.github/workflows/policy.yml`, CI comparisons, `scripts/lint.py`, and `scripts/doc-tools.sh` | Retain public syntax, secret, lint, and documentation checks. Check `origin/dev` assumptions, including lint adoption on `main`, against a fork that copied only `main`. Use existing comparison parameters and valid event commits. Missing history must not become a successful empty comparison. |
| `.github/workflows/gremlins.yml`, `scripts/run-bounded.sh`, and `memory-budget-check.sh` | Candidate-source mutation testing, regression proof, race tests, and Linux memory enforcement already work without Ground Control. Retain their controls and hosted execution boundaries. |
| `.github/workflows/pr-title.yml` and `.github/PULL_REQUEST_TEMPLATE.md` | Public title validation and review evidence already exist independently. Retain them without private workflow attestations becoming contributor prerequisites. |
| Release, synchronization, and queue-label workflows | These are distinct automation surfaces with API writes. A main-only fork also lacks the synchronization target `dev`. Classify owner-specific side effects explicitly; release or back-merge setup must not become a fork CI prerequisite. Do not redesign publishing or branch administration for this issue. |
| `AGENTS.md`, `CLAUDE.md`, `.gc/plan-rules.md`, and `docs/CONTRIBUTING.md` | Public instructions must lead to repository-owned commands and checks. Ground Control commands, context tools, private skill contracts, and delivery records must not become contributor or agent obligations. Preserve tool-independent quality rules. |
| `docs/MAINTAINER_SETUP.md`, `phase-e.md`, `phase-e-design.md`, `process-review.md`, and the optional-tooling ADR | Remove public installation and registration prescriptions and contributor skill invocations. Keep necessary descriptions and recovery facts explicitly limited to the existing maintainer integration. Correct current claims that conflict with `dev`, the fork module path, or the configured Sonar project. Clearly distinguish historical evidence from current instructions. |
| `.ground-control.yaml`, `.architecture/model.json`, and `scripts/check-design.sh` | Retain the existing maintainer configuration and public architecture checker. The model requires canonical policy files, including Ground Control files, to exist as regular checkout files. Optional execution does not mean deleting these files or disabling model checks. |
| `scripts/install-hooks.sh` and `.pre-commit-config.yaml` | The installer invokes pre-commit without Ground Control. Its managed marker and verification variable are compatibility identifiers. Do not rename them merely to remove a product reference or replace the shared dispatcher. |

The `.github/ground-control/` manifest, lockfile, harness, and GitHub stand-in remain the owners of private transport verification.
The harness loads its YAML parser from the private reference package and executes shell steps directly.
It does not evaluate Actions job conditions. Therefore it cannot establish fork scheduling behavior by itself.
The [finalization design](phase-e-design.md) retains the canonical data-flow diagrams and package-handoff boundary.
Repository applicability wraps those existing entry points; it adds no execution path or store.

## Cross-cutting controls

| Layer | Design obligation and incumbent |
| --- | --- |
| Authorization and events | Use GitHub's executing repository context for applicability, then retain existing event restrictions and least-privilege job permissions. Keep `pull_request_target` limited to the trusted dispatcher; it executes no candidate files. Owner applicability is not permission to expose credentials to candidate code. |
| Credentials and OS exposure | Keep the release credential step-scoped and absent from installation, finalization, and candidate tests. Pass event values through environment variables and quoted arguments. No token belongs in argv, URLs, tracing, Git config, outputs, caches, or artifacts. Keep checkout credential persistence disabled. |
| Workflow and environment shapes | Reuse Actions conditions and `actionlint`, pinned PyYAML in public Python checks, and the existing shell input validators. Secrets cannot be referenced directly in Actions `if` expressions. Do not invent unsupported `.ground-control.yaml` fields, a parallel schema, or Gremlins environment bindings. |
| Private installation | Preserve the workflow's release/version/asset/SHA256 tuple, npm manifest and lockfile, and isolated temporary installation. Retain checksum verification, disabled lifecycle scripts, isolated npm config/cache, and disabled audit submission. |
| Architecture and policy validation | Reuse `tools/architecture` and its strict JSON decoder, canonical-file checks, design coverage, and shrink-only baseline. Declare all modeled trust boundaries in the issue design. Keep `development_checks`, `delivery_finalization`, `transport_verification`, and `merge_queue_policy` distinct. |
| Sonar admission | Retain `scripts/check-coverage.sh`, exact expected coverage inputs, property allowlisting, path confinement, duplicate rejection, and validated merge-group scanner arguments. Scope provider execution without weakening its admission checks. |
| Errors and observability | Reuse shell/Make exit status, Actions notices/errors, Node test diagnostics, and the existing `PR Gate`. An explicitly inapplicable job may skip. A selected job's missing credential, failed download, bad digest, invalid metadata, cancellation, or missing result must fail. No `continue-on-error`, blanket skip allowance, or swallowed failure. |
| Persistence and network | Use existing checkout/Git history, temporary runner directories, approved GitHub/npm destinations, and the established finalizer's durable evidence. Preserve PR/merge-group cache isolation; do not introduce base/default-branch private caches or public package artifacts. No database, new provider, external evidence store, or new hosting is needed. |
| Hook and security policy | Preserve fast commit checks, redacted Gitleaks output, history scanning, and the shared hook dispatcher. Making provider jobs inapplicable in forks does not remove native secret or vulnerability checks. Security findings follow `SECURITY.md`; this note adds controls, not public vulnerability records. |

The [Actions contexts reference](https://docs.github.com/en/actions/reference/workflows-and-actions/contexts) defines which contexts each condition can use.
[GitHub's secret guidance](https://docs.github.com/en/actions/how-tos/write-workflows/choose-what-workflows-do/use-secrets) excludes direct secret references in conditionals.
The existing private-package handoff relies on [cache access restrictions](https://docs.github.com/en/actions/reference/workflows-and-actions/dependency-caching).
Repository guards must not broaden that handoff to push or manual runs.

## Expected outcomes and evidence boundaries

| Case | Required result |
| --- | --- |
| Clone without private tooling or credentials | Native build and relevant tests/checks run through existing commands. No registration or private workflow skill is needed. |
| Fork push, same-repository fork PR, or fork merge group | Native CI remains enforceable. OpenRAE-specific integrations are inapplicable. No private package is requested and no OpenRAE administrative policy is queried. |
| Fork merged-PR event or manual Phase E dispatch | Private finalization and its dispatcher are inapplicable before API writes, downloads, or installation. |
| OpenRAE affected same-repository PR or merge group | Existing private transport verification remains required. A missing package credential or failing harness cannot become a permitted skip. |
| OpenRAE foreign or Dependabot PR | Preserve restricted-event credential isolation. Public workflow checks still run; private transport verification retains its merge-queue path. Do not promise all existing provider checks pass on these restricted PR events. |
| OpenRAE same-repository merge, merged external PR, or valid recovery dispatch | Preserve existing finalization, trusted dispatch, immutable delivery, provenance, and idempotent report/closure behavior. |
| Failure or cancellation of any applicable mandatory check | `PR Gate` remains red. A skipped dependent job is not proof of success. |

Public scheduling and gate assertions must run without the private utility, its YAML parser, or its archive.
The scope job must succeed and emit an explicit boolean selection; an absent or invalid output cannot authorize a skip.
Reuse the repository's Python unittest/PyYAML conventions for public policy checks and the existing Node harness for private finalizer behavior.
Use table-driven cases for repository, event, PR origin, selection, and dependency-result combinations.
Exercise actual workflow shell where relevant; do not build a second Actions expression interpreter or copy finalizer logic.
Hosted evidence must confirm Actions scheduling and permissions; static YAML assertions cannot prove either.

Relevant verification includes `actionlint`, the native design gate, explicit-path documentation checks, and focused public workflow-policy checks.
Maintainer-only verification retains the existing transport harness and synthetic GitHub stand-in.
Full Go suites and mutation runs remain CI responsibilities.
For scheduling guards, false-condition and failed-dependency cases provide fault evidence without adding a mutation framework.
This preflight does not certify a fork CI run or a configured finalization run.

## Extension seams and non-goals

Applicability belongs in the existing workflow conditions and transport scope result; required-result enforcement belongs in `PR Gate`.
Keep one owner-specific predicate shape across finalization, dispatch, and verification, without creating a generic workflow engine.
No new activation variable is necessary for this issue.
A future authorized second maintainer repository would change applicability, not installation, finalizer behavior, or public build commands.

The release tuple and lockfile remain the upgrade seam.
`GROUND_CONTROL_TEST_ROOT` and `GROUND_CONTROL_RELEASE_ASSET` remain private-harness inputs.
`--repository`, `ARCHITECTURE_BASE`, `LINT_BASE`, `LINT_ADOPTION_BASE`, and `DOCS_BASE` already parameterize their respective native tools.
Keep documentation links and canonical policy paths valid when removing instructions.

No Go package, Mutation type, CLI flag, Configuration key/schema, JSON report, runtime exit code, or downstream mutation contract changes.
The Go Logger, Exit errors, and Work directory copy have no new responsibility here.
No private finalizer reimplementation, new completion schema, issue-discovery parser, MCP client, durable store, dependency upgrade, or provider migration.
No branch-rule changes, merges, releases, hook bypasses, baseline growth, or formatting of byte-exact `testdata` inputs.
Removing every Ground Control name is not the acceptance criterion; removing contributor obligations and private execution dependencies is.
