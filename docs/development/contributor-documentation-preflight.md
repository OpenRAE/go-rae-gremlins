# Contributor documentation boundaries for issue #223

This explanation reviews revision `59baad1c460ac0170a11270faa68e232b230067e` against the supplied issue #223 contract.
It adds design guidance only. It does not deliver the documentation rewrite or prescribe implementation steps.
The [adoption design](diataxis-adoption.md) remains the structural authority.
Existing ADRs cover this work; no new ADR or architecture rule is needed.

## Ownership and reader purpose

Keep `AGENTS.md`, `CLAUDE.md`, `.gc/plan-rules.md`, `docs/CONTRIBUTING.md`, and `docs/MAINTAINER_SETUP.md` at their recognized paths.
Agent entry points retain binding instructions and link to canonical guidance.
Do not depend on an agent following an optional link to discover a mandatory safety restriction.
Preserve the force of each obligation, recommendation, prerequisite, exception, and condition.

| Reader need | Existing owner and boundary |
| --- | --- |
| Contribution tasks and daily checks | `docs/CONTRIBUTING.md`. Link setup, standards, architecture, and release details instead of copying them. |
| Optional maintainer registration | `docs/MAINTAINER_SETUP.md`. Ordinary contributors need no Ground Control or review-provider credentials. |
| Exact Go and test rules | `coding-standards.md` and `test-standard.md`. Preserve rule IDs, exceptions, and the distinction between checks and human review. |
| Architecture explanation | `CLAUDE.md`, `architecture.md`, `lint-design.md`, `mutation-registry.md`, and `phase-e-design.md`. Preserve package ownership and trust boundaries. |
| Operating procedures | `release.md`, `phase-e.md`, and `memory-budget.md`. Separate task instructions from reference facts and historical comparisons. |
| Decisions and evidence | Both existing ADR trees, preflights, assessments, reviews, `maintenance-plan.md`, `upstream-release.md`, and `evidence/`. Preserve the original scope and source revisions. |

Diataxis forms describe reader purposes, not new directories, runtime layers, or mandatory page templates.
Give each maintained section one purpose. Split a page only when its reader needs justify the split.
Keep a useful old heading and a short destination link when content moves.
Issue #222 owns user-page rewriting; #224 owns final entry-point and navigation reconciliation.
Issue #223 still checks repository Markdown targets and fragments; it cannot defer broken links it introduces to #224.

Historical records can receive clearly separated current context.
Do not replace an old observation with today's result or silently change an ADR status.
Preserve decision IDs, requirement IDs, finding IDs, source hashes, quotations, evidence files, and incoming links.
Both `docs/adr/0009-ignore-mutant-directives.md` and `0009-temporary-build-cache.md` already exist.
Use their full titles and paths to distinguish them; this editorial issue does not renumber either decision.

## Source differences that the rewrite must resolve

These are observations at the reviewed revision, not evidence that the rewrite is complete.

| Current text | Source agreement required |
| --- | --- |
| `MAINTAINER_SETUP.md` says PRs target `main` and the module remains upstream. | Current `go.mod`, `.ground-control.yaml`, AGENTS guidance, and ADRs 0001 and 0007 select the fork and `dev`. The older `architecture/adrs/001-optional-maintainer-tooling.md` needs explicit current context without erasing its original decision. |
| Setup says SonarCloud is unconfigured and names Ground Control 1.8.1. | `.ground-control.yaml`, `sonar-project.properties`, and `ci.yml` configure Sonar. Phase E separately pins v1.12.0. That transport pin does not establish the installed local CLI version or its supported configuration schema. |
| Setup recommends `pre-commit install`. | `make hooks` invokes `scripts/install-hooks.sh`, which installs clone-local hooks and proves effective dispatch. Preserve the shared dispatcher and unmanaged-hook refusal. Configuration validity and green CI do not prove local activation. |
| `.ground-control.yaml` says `make policy` runs broad Go checks. | `Makefile` selects the pre-commit stage. `.pre-commit-config.yaml` puts lint, vulnerability analysis, and race tests at the manual stage. Correct explanatory comments without changing executable commands or stages. |
| `CLAUDE.md` presents full commands as a daily cycle, mentions Codecov, and describes dry-run too briefly. | CI owns full Go and mutation runs. Coverage goes through the configured Sonar pipeline. `cmd/unleash.go` runs coverage tests before dry-run stops mutant execution. Dry-run is not a safe way to avoid target-code execution. |
| Vocabulary names `internal/engine/tokenmutator.go`. | That file is absent. Current mutation behavior belongs to `specs.go`, `spec.go`, `registry.go`, and `mutant.go`. Correct maintained references; preserve old paths inside revision-bound evidence. |
| Release prose says the manifest starts at 0.6.0. | `.release-please-manifest.json` currently contains 0.10.0. Separate release bootstrap history from current generated state. Do not edit the manifest or infer published release availability from it. |

The Go language requirement is `go 1.25.0`; the toolchain suggestion and mise pin are Go 1.26.11.
Keep these meanings distinct. Verify linter and fieldalignment versions against `mise.toml`, `Makefile`, and `go.mod`.
Claims about installed services, branch settings, or released tools need their own evidence; repository configuration alone does not prove live state.

## Cross-cutting validation and security

| Layer | Canonical incumbent and required treatment |
| --- | --- |
| Writing | Reuse the shared `documentation` skill and `scripts/doc-tools.sh`. The pinned v0.1.0 package uses ste-cli 0.10.1. Its dictionary and analyzer are disabled; only STE-4.2, STE-5.1, and STE-6.6 are enabled. Manually review reader purpose, terminology, sentence rules, and technical meaning. A pass is not full STE conformance. |
| Tool supply chain | Preserve release checksums, fail-on-mismatch behavior, and ignored `.tools/` installs. The wrapper prefixes relative filenames to prevent option interpretation. Use explicit changed Markdown paths; do not add a checker, skill copy, corpus baseline, or service. |
| Architecture admission | Reuse `.architecture/designs/223.json`, `.architecture/model.json`, `scripts/check-design.sh`, and `tools/architecture`. The checker rejects unknown fields, duplicate keys, nulls, and unsafe paths. Guidance rewrites touch modeled `guidance`, `contributing`, and `plan` policies; `.ground-control.yaml` comment corrections also touch `workflow`. |
| Policy coverage | The checker uses changed paths, not editorial intent. A comment-only edit to a modeled contract or policy still needs design coverage. Declare actual affected Go packages and compatible contract edits if maintained source comments require them. Recheck after scope or model changes; never grow either architecture or lint baselines. |
| Configuration shapes | JSON/YAML hooks check syntax. The installed Ground Control parser owns its configuration shape; do not invent fields or copy an unreleased schema. If documentation pointers change, keep `.ground-control.yaml` pointers valid or retain routing at their current targets. |
| Runtime configuration | Cobra, `cmd/internal/flags/flags.go`, Viper, and `configuration.Validate` own flags, bindings, types, and ranges. `docs/docs/schema/configuration.json` is editor reference, not runtime validation. Documentation checks need no `configuration.Init`, host config discovery, or new `GREMLINS_*` key. |
| Environment and comparison | `DOCS_BASE`, `ARCHITECTURE_BASE`, and `LINT_BASE` select distinct check comparisons. `GROUND_CONTROL_TEST_ROOT` selects the existing transport test installation. These are not Gremlins configuration keys. Preserve explicit commit comparisons and report missing history as a limit. |
| Filesystem and OS | Keep searches and check inputs inside this checkout. Preserve literal shell arguments, quoting, paths, and platform conditions. Do not execute setup, release, recovery, allocator, or mutation examples to validate prose. Work directory copies isolate mutations in source files; they do not sandbox target code or credentials. |
| Authentication and secrets | Local writing and link review need no credentials. Keep `.env` and `.mcp.json` ignored and never inspect secret files or values. Examples must use placeholders without credentials in URLs, argv, output, or rendered files. Preserve existing hook dispatch, staged secret scanning, and CI history scanning. |
| CI authorization | Reuse existing jobs and pinned actions. Candidate checks retain read-only access and checkout without persisted credentials. Preserve the separate Sonar admission and scan jobs, transport download and test jobs, and trusted merge-only fork dispatcher. A prose rewrite does not authorize token access or workflow dispatch. |
| Errors and observability | Existing doc-tools, architecture, and optional MkDocs diagnostics plus nonzero status provide check evidence. Keep failures visible. Application Logger, wrapped Go errors, and Exit errors retain ownership; introduce no error hierarchy, HTTP envelope, or mutation-report diagnostics for documentation. |
| Persistence and delivery | Git holds guidance and revision evidence. Existing CI holds check results; ignored local output holds any site build. No database, DTO, repository service, external evidence store, new provider, or publication destination is required. One rewrite PR targets `dev`; the user owns merge approval. |

Read source comments as part of their owning subject, including Makefile, script, workflow, and configuration explanations.
Edit maintained prose at its source, without changing commands or generated records to make the text true.
Preserve Go directives, build constraints, `//line`, lint directives, mutation annotations, example output markers, and license notices.
Moving a directive comment can change behavior even when no executable statement changes.
Do not rewrite public help strings, schema descriptions, or byte-exact fixtures as an incidental contributor-documentation edit.
Route user-reference changes to their owning work unless #223 directly needs a bounded correction.

ADRs 0001–0004 govern identity, distribution, upstream imports, and architecture checks.
ADR 0005 preserves the isolated lint parser's standard-library logging exception; do not replace it with application Logger reuse.
ADRs 0006 and 0007 preserve Linux memory containment and merge-queue rules.
ADRs 0008 and both 0009 records govern annotation and build-cache explanations that appear in `CLAUDE.md`.
Retain their exact conditions and cross-references when separating explanation from procedures.

## Links, evidence, and extension boundary

The site root remains `docs/docs`, relative to `docs/mkdocs.yml`.
Contributor navigation currently links to the canonical guide on GitHub; it does not render a contributor-page symlink.
Do not copy contributor pages into the site, widen `docs_dir`, or add include, redirect, staging, or publication machinery.
Check source-relative Markdown targets, heading fragments, and references from outside the rewritten files, including hidden guidance.
Also inspect native MkDocs navigation when a changed link intersects the site.

Site content or navigation changes require a local strict MkDocs build and affected rendered-link and fragment review.
Generic YAML and writing checks do not establish those results.
Use ignored `docs/site` or private temporary output; do not deploy or upload it.
This environment has no installed MkDocs stack; no successful build is claimed by this preflight.

`cmd/schema_regression_test.go` extracts YAML after the exact `### Reference` heading in `docs/docs/usage/configuration.md`.
That user-page boundary is outside this rewrite. Preserve it and avoid incidental changes to its tests or schema.
If scope later includes it, update the extraction boundary and links together, with targeted schema verification.

The extension seam is ordinary links to canonical owners, plus existing explicit path and comparison parameters.
A future guide can add its reader purpose and links without changing schemas, checkers, workflow commands, or directory conventions.
Retain short mandatory instructions in machine-read entry points and route their supporting explanation to the canonical owner.
Do not maintain separate copies of standards for each agent.

The rewrite PR must record its source revision, source paths, corrected differences, actual commands and results, and remaining limits.
For retained records, distinguish the original revision from any new contextual note.
Check source agreement manually; links to other prose alone are not evidence against implementation.
Record any STE dictionary access limit without declaring full terminology verification.
No local full Go, coverage, mutation, transport, or allocator suite is required for this prose-only preflight.

## Preflight evidence

The explicit design check passed ARCH001–ARCH004 against model SHA256 `c28bab810f1497d79aea1fcac11781ad83abf8d2e7b36cb8c78a8e7225243f6b`.
The repository architecture check passed with zero baseline findings against the reviewed `HEAD`.
Both checks used mise-pinned Go with `GOTOOLCHAIN=local`, `GOPROXY=off`, and `GOSUMDB=off`.
The explicit-path writing check passed with ste-cli 0.10.1. JSON syntax and whitespace checks also passed.

`make policy` passed its configured pre-commit-stage checks. Its all-files selection covers tracked files, not new untracked files.
The applicable hooks also passed with explicit paths to both new files.
The new note's sole Markdown link selects the tracked adoption design and has no fragment.
No contributor rewrite, full Markdown link audit, live service check, site build, example execution, or publication was performed.

## Non-goals

No contributor rewrite, implementation plan, new ADR tree, copied skill, custom validator, or documentation framework is delivered here.
The later rewrite changes no package relationship, Mutation type, CLI flag, Configuration key, schema contract, JSON report, or exit code.
It changes no hook activation, trust boundary, service configuration, merge policy, runtime validation, or release state.
Preserve upstream traceability, historical evidence, adopted policy, copyrights, and fixture bytes.
Do not turn discovered runtime or security concerns into an unrequested repair or public finding record.
