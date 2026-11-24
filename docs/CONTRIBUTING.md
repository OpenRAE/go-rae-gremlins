# Contributing to go-rae-gremlins

go-rae-gremlins is the OpenRAE fork of Gremlins.
This guide gives the contribution tasks from issue to release.
Use the [OpenRAE issue tracker](https://github.com/OpenRAE/go-rae-gremlins/issues) for work on this fork.
Follow the [Code of Conduct](../CODE_OF_CONDUCT.md).

Related guidance:

- [Hosted checks](development/checks.md): the CI workflows, the merge queue, and the pull request test gates.
- [Go coding standard](development/coding-standards.md) and [Go test standard](development/test-standard.md): the exact rules.
- [Architecture checks](development/architecture.md) and the [ADRs](adr/README.md): the design rules and decisions.
- [Maintainer setup](MAINTAINER_SETUP.md): the local checks and the tooling of the maintainers.
- [Development process review](development/process-review.md): the 2026-11-02 record of the decisions behind this guide.

## Start with an issue

1. Search the existing issues before you report a bug or suggest a feature.
2. Use the issue templates where they apply.
3. For a bug, give the Gremlins version, the Go version, and the operating system.
   Also give the steps, and the expected and actual results.
4. For a feature, give the problem, the proposed behavior, and the alternatives.
5. Before you start a significant change, agree on it with a maintainer.

Give the issue a clear outcome and acceptance criteria that you can check.
The issue body and discussion hold the requirements and decisions.

A copied upstream issue or PR needs a keep, rework, or close decision before implementation or merge.
The [maintenance plan](development/maintenance-plan.md) records this triage.

## Prepare the clone

1. Install [mise](https://mise.jdx.dev) and [pre-commit](https://pre-commit.com).
2. Install the pinned tools and the hooks:

   ```shell
   make requirements
   make hooks
   ```

3. Run `make hooks` again for each new clone, because Git does not copy local hooks.

`make requirements` installs golangci-lint and the Go toolchain that `mise.toml` pins.
It then uses that Go to install fieldalignment v0.48.0, the `golang.org/x/tools` version that `go.mod` requires.
`FIELDALIGNMENT_VERSION` in the [Makefile](../Makefile) sets this version.
Review the tool before you change its version.

`make hooks` installs the commit-time hook dispatch of the clone and verifies it.
It then runs the commit-stage checks on all files.
If the installation reports an unmanaged hook or a dispatcher problem, examine and correct the problem before you commit.
Do not bypass the hooks.
Do not overwrite a shared hook configuration.

The [pre-commit configuration](../.pre-commit-config.yaml) checks file hygiene, syntax, merge conflicts, private keys, and secrets at commit time.
The broad Go checks are at the `manual` stage: lint, vulnerability analysis, and the race test suite.
CI enforces them; see [Hosted checks](development/checks.md).
To run a broad check on demand, use `pre-commit run <hook-id> --hook-stage manual`.

## Branch and implement

1. Start an issue branch from the current `dev`. Name it `<issue>-<short-slug>`.
2. Before you change code, read `CLAUDE.md`, the [plan rules](../.gc/plan-rules.md), and the [ADRs](adr/README.md).
3. Write your design to `.architecture/designs/<issue>.json`, as the [architecture design check](development/architecture.md#design-before-coding) describes.
4. Run `make architecture-design`. Record its model digest and result in the issue plan.
5. Resolve all design conflicts before implementation.
6. Run the design check again when the design or the model changes.
7. Implement the change, and update the documentation that it affects.
8. Target the PR at `dev`.

`main` receives reviewed promotions from `dev`.
Maintainers approve those merges and all feature PR merges.

To keep the issue branch current, fetch `origin/dev` and merge it into the branch.
Resolve conflicts with a real merge and verify the result.
Keep the existing work and history.
Do not use a hard reset or a force-push to discard conflicts.

Keep the change in the scope of the issue.
Use the existing helpers and dependencies.
When a change adds or changes a flag or a configuration key, also update the JSON schema and the usage pages.
Keep the CLI, configuration, report, and exit-code contracts unless the issue permits a change, because dependent repositories use them.
Keep `testdata` files byte-exact unless the fixture itself must change.

CI requires an updated design for package, contract, and policy changes.
To scan all packages against a fetched target commit, run `make architecture ARCHITECTURE_BASE=origin/dev`.
Each finding gives a repair instruction.
When you fix a violation, remove its stale baseline entry.
Do not regenerate or enlarge the architecture baseline.
Architecture policy changes need maintainer review.

Follow the [Go coding standard](development/coding-standards.md).
Provision the pinned linter and Go with mise, and run `go mod download` before analysis.
For a local lint check, run `make lint LINT_BASE=origin/dev` or give another fetched target commit.
Repair the findings, then run `make lint-shrink` to remove stale allowances.
Do not regenerate the lint baseline.
Changes to rules, suppressions, contexts, and tool versions need explicit review.

For an upstream commit, follow [ADR 0003](adr/0003-upstream-changes.md).
Cherry-pick it with `-x`, keep the original author, adapt the module path, and open a PR to `dev`.
Do not merge an upstream branch into this fork.

## Documentation

Write documentation in ASD-STE100 Simplified Technical English.
The checks use the [doc-tools v0.1.0](https://github.com/OpenRAE/doc-tools/releases/tag/v0.1.0) release.
[`scripts/doc-tools.sh`](../scripts/doc-tools.sh) records the release version and the archive SHA-256.
The script verifies the archive before extraction and installs it in `.tools/`, which Git ignores.
The package installer verifies the ste-cli archive in the same way.
A checksum mismatch stops the check.

To check the Markdown files that you change, give explicit paths:

```shell
bash scripts/doc-tools.sh lint docs/CONTRIBUTING.md README.md
```

To check all Markdown files that changed since the merge base with `origin/dev`, run this command:

```shell
make docs-changed
```

The check examines each changed file completely.
When you edit a page, also correct its existing findings.
The check does not examine `CHANGELOG.md`, `CODE_OF_CONDUCT.md`, or files in `testdata` directories.
A passing check does not prove full ASD-STE100 conformance.
Also review the accuracy, the links, and the diff.

For agent-assisted writing, use the shared `documentation` skill of the same release.
First remove an older personal skill named `documentation`, because two skills with the same name can both show.
In Claude Code, run these commands:

```text
/plugin marketplace add OpenRAE/doc-tools#v0.1.0
/plugin install doc-tools@openrae-doc-tools
```

In Codex, run the preinstalled `skill-installer` script:

```shell
python3 "${CODEX_HOME:-$HOME/.codex}/skills/.system/skill-installer/scripts/install-skill-from-github.py" \
  --repo OpenRAE/doc-tools --path skills/documentation --ref v0.1.0
```

To upgrade or roll back, follow the [installation and upgrade guide](https://github.com/OpenRAE/doc-tools/blob/v0.1.0/docs/how-to-install.md) of the package.
Set `VERSION` and `SHA256` in `scripts/doc-tools.sh` to the values in the `SHA256SUMS` file of the release.

## Verify the change

Follow the [Go test standard](development/test-standard.md) for test behavior, isolation, and evidence.
The [test assessment](development/test-assessment.md) records the gaps found against that standard and their repairs.

1. For a bug, add a regression test. For new behavior, add tests.
2. Name each new Go test `TestRegression...` for a fix or `TestFeature...` for new behavior. The [regression proof](development/checks.md#regression-proof) gives the full rule.
3. Run the narrowest tests that exercise your change, with the race detector. For example:

   ```shell
   go test -race ./internal/coverage -run '^TestCoverageParsesOutput$'
   ```

4. Where it applies, examine the mutation results to measure test effectiveness.
5. Format the changed Go files with the pinned formatter:

   ```shell
   mise exec -- golangci-lint fmt path/to/changed.go
   ```

For a documentation-only change, run the [documentation checks](#documentation).
A documentation-only change needs no new Go test.

CI owns the full verification runs.
The repository also gives `make all` (lint, tests, build) and `make cover` (race tests and coverage).
Do not run these broad targets as a daily local check.
`make policy` runs the commit-stage hooks on all files, and it is fast.
Run targeted checks locally.
The publication hooks and the hosted checks then enforce their gates.

The [hosted checks](development/checks.md) reference lists each workflow, the `PR Gate` inputs, and the merge queue checks.
Examine every check on the current PR head, including the installed security checks.
A green `PR Gate` alone does not show that every hosted check passed.

A fork does not need Ground Control, SonarCloud, GitGuardian, or maintainer access.
In a fork, the jobs that need an OpenRAE secret, service, or branch setting skip.
[Forks](development/checks.md#forks) lists these jobs.

If the merge queue removes your PR, the `merge-queue-removed` label shows on the PR.
Repair the cause, then add the PR to the queue again.

Repair the failures that the change causes or that directly block it, and verify the new head.
Report other concerns with evidence, and do not expand the issue.

## Commit messages and pull requests

Use Conventional Commits for commit messages and PR titles, for example:

```text
fix(coverage): preserve slash-separated profile paths
```

The allowed types are `feat`, `fix`, `docs`, `chore`, `refactor`, `test`, `build`, `ci`, `perf`, `style`, and `revert`.
A scope is optional.
The subject starts with a lowercase letter.
This convention replaces the upstream `area: description` convention, and the release automation of the fork uses it.

Keep branch names and repository metadata free of assistant or vendor attribution.
The review records and the review attestation that the maintainer tooling requires are the only exceptions.

Use the [PR template](../.github/PULL_REQUEST_TEMPLATE.md).
Describe the resulting behavior, link the issue, give the ADR impact, and report the checks that you ran and their results.
To close the delivered issue at merge, write `Closes #<issue>`.
GitHub closes issues automatically only for merges into the default branch.
A merge into `dev` therefore needs an explicit post-merge finalization.

Maintainers review the change.
Pull requests to `dev` merge through the merge queue, which squashes them.
Merges into `main` stay manual, and the maintainer selects the merge method.
Merge only after the current head has the required checks and review.
A contributor or agent who prepares the PR does not merge it.

## Maintainer tooling

The maintainer uses private tooling to deliver issues.
Contributors do not need it, and this guide does not depend on it.
[`.ground-control.yaml`](../.ground-control.yaml) and [`.gc/plan-rules.md`](../.gc/plan-rules.md) configure that tooling for this repository.

After a PR merges, the [Phase E workflow](../.github/workflows/ground-control-phase-e.yml) can post a final report and close the delivered issue.
It runs only in `OpenRAE/go-rae-gremlins`.
[Automatic Phase E finalization](development/phase-e.md) is for the maintainer.

## Release

Follow the [fork release procedure](development/release.md).
[ADR 0002](adr/0002-binary-name-and-versions.md) records the release decisions.
Maintainers approve the `dev` to `main` promotion and the release PR.

release-please manages the versions, tags, release notes, and `CHANGELOG.md`.
GoReleaser adds the binary archives, the source archive, and the checksums.
Do not edit release versions or `CHANGELOG.md` manually.
Do not publish to the upstream Docker, Homebrew, or documentation channels.

The fork does not use the upstream release freeze or maintenance release branches.
`GITHUB_TOKEN` opens the release PR and the synchronization PR, so the normal PR checks do not start on them.
Maintainer review stays necessary.
After a release, review and merge the automated `main` to `dev` synchronization PR as the release procedure describes.

### Pull request test gates

This content moved to [Pull request test gates](development/checks.md#pull-request-test-gates) in the hosted checks reference.
