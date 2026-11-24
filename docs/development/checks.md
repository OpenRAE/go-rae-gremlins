# Hosted checks

This reference lists the GitHub checks of go-rae-gremlins and their conditions.
The workflow files in `.github/workflows/` are the source of these facts.
For the contributor procedure, see [Verify the change](../CONTRIBUTING.md#verify-the-change).

## Workflows

| Workflow | Events | Jobs and verification |
| --- | --- | --- |
| [CI](../../.github/workflows/ci.yml) | Pushes to `main` and `dev`, pull requests to `main` and `dev`, merge groups, and manual dispatch. | `Architecture`: model, design, and shrink-only baseline. `Lint`: golangci-lint with the shrink-only baseline, and Python line and branch coverage. `Vulnerability scan`: govulncheck. `Test`: race tests and Go coverage on Linux, macOS, and Windows. `Policy`: commit-stage hooks on all files, and the merge queue policy. `Docs`: STE checks on changed Markdown. `Release build`: GoReleaser snapshot build of every release target. `Memory budget`: cgroup-v2 budget proof. `Sonar admission` and `SonarCloud`. The Phase E transport jobs. `PR Gate`. |
| [Gremlins](../../.github/workflows/gremlins.yml) | Pushes to `main` and `dev`, pull requests to `main` and `dev`, and merge groups. | `Mutation testing` with a binary built from the candidate commit. `Regression proof` on pull requests and merge groups. |
| [Repository policy](../../.github/workflows/policy.yml) | Pushes to `main` and all pull requests. | `Configuration and secret checks`: commit-stage hooks on all files, and a Gitleaks scan of the full Git history. |
| [PR title](../../.github/workflows/pr-title.yml) | Pull requests to `main` and `dev`, and merge groups. | `Conventional PR title`: a Conventional Commit title with an allowed type and a lowercase subject. |
| [GitGuardian](../../.github/workflows/ggshield.yml) | Pull requests to `main` and `dev`, and merge groups. | `ggshield`: a secret scan of the new commits of the pull request or merge group. |
| [Maintainer Gate](../../.github/workflows/maintainer-gate.yml) | Pull request and review events, and merge groups. | `maintainer-gate`: the maintainer wrote or approved the pull request. |
| [Merge Queue Removed](../../.github/workflows/merge-queue-removed.yml) | A pull request to `dev` enters or leaves the queue, or closes. | Adds the `merge-queue-removed` label when the queue removes an open pull request without a merge. Removes the label when the pull request enters the queue again or closes. |
| [Release](../../.github/workflows/release-please.yml) and [Sync main -> dev](../../.github/workflows/sync-main-to-dev.yml) | Pushes to `main`. | The [release procedure](release.md) describes these workflows. |
| [Ground Control Phase E](../../.github/workflows/ground-control-phase-e.yml) and [fork dispatcher](../../.github/workflows/ground-control-fork-finalization.yml) | Merged pull requests and manual dispatch. | Post-merge finalization. See [Automatic Phase E finalization](phase-e.md). |

## Forks

Some jobs need an OpenRAE secret, service, person, or branch setting.
They run only when `github.repository_id` identifies `OpenRAE/go-rae-gremlins` ([ADR 0010](../adr/0010-maintainer-integrations-in-the-canonical-repository.md)).
A rename or a transfer keeps this ID, and a fork has a different ID.
In a fork, these jobs skip:

- CI: `Sonar admission`, `SonarCloud`, `Phase E release`, and the merge queue policy step of `Policy`.
  `Phase E transport scope` runs, but it selects no transport tests.
- `ggshield`, `maintainer-gate`, and the `merge-queue-removed` label job.
- The Release, Sync main -> dev, Ground Control Phase E, and fork dispatcher workflows.

All other jobs run in a fork.
In `OpenRAE/go-rae-gremlins`, a missing secret still fails its job.
[`test_workflow_applicability.py`](../../.github/scripts/test_workflow_applicability.py) checks the repository condition of each of these jobs.

## PR Gate

`PR Gate` requires success from these CI jobs: `Architecture`, `Lint`, `Vulnerability scan`, `Docs`, `Test`, `Release build`, `Policy`, and `Memory budget`.
It also requires success from `Phase E transport scope`.

`SonarCloud` must succeed in `OpenRAE/go-rae-gremlins`, with one exception.
For a pull request from another repository, `SonarCloud` must be skipped, because that pull request gets no `SONAR_TOKEN`.
In a fork, `SonarCloud` must be skipped.

`Phase E transport tests` must succeed when the scope job selects them, and must be skipped when it does not.
A selection other than `true` or `false` fails `PR Gate`.

The scope job selects them only in `OpenRAE/go-rae-gremlins`.
It selects them on pull requests and merge groups that change the transport or its verification wiring, including `ci.yml` and the `Makefile`.
Fork and Dependabot pull requests get no secrets, so they skip the transport tests.
The merge queue runs the tests for them.
See [pre-merge transport verification](phase-e-design.md#pre-merge-transport-verification).

A green `PR Gate` does not show that all hosted checks passed.
The other workflows above report separate checks.

## Merge queue

Pull requests to `dev` merge through a GitHub merge queue, as [ADR 0007](../adr/0007-dev-merge-queue.md) records.
The queue squashes each pull request.

The queue requires these checks on each merge group commit: `PR Gate`, `Mutation testing`, `Conventional PR title`, `ggshield`, and `maintainer-gate`.
Each of these workflows runs on `merge_group`.

On a merge group, a workflow compares with the merge group base commit and reads the queued pull request.
A failed check removes the pull request from the queue.
`main` has no merge queue.

The `Policy` job of CI runs `make merge-queue-policy`.
This check reads the live branch rules of `dev` and `main` with the GitHub API.
A fork skips this step.
It fails in each of these conditions:

- `dev` has no merge queue.
- `main` has a merge queue.
- `dev` has required checks in classic branch protection.
- `dev` has a merge queue but no required checks.
- `dev` has non-strict required checks and no merge queue.
- A required `dev` check has no workflow job that runs on `merge_group`.

## Pull request test gates

### Mutation testing

The `Mutation testing` job builds this fork from the candidate commit.
On pull requests and merge groups, it mutates the added Go lines relative to the merge base with the base commit.
The thresholds of `.gremlins.yaml` apply.
Lines outside the diff are skipped.
A change without mutation candidates passes, and no threshold applies to the empty selection.

Pushes to `main` and `dev` mutate the full repository.
Each run is inside the [memory budget](memory-budget.md).

### Regression proof

The `Regression proof` job of the Gremlins workflow runs on pull requests and merge groups.
It is not a required queue check and not an input of `PR Gate`.

Each new Go test name must start with `TestRegression` for a fix or `TestFeature` for new behavior.
The job rejects a new test that has neither prefix.
Existing test names need no change.
A new named subtest takes the classification of its parent.
A new subtest under an unclassified existing test fails.

The job runs the head and base suites to find test and subtest names.
It proves each new regression leaf separately, with the race detector on and the test cache off.
A failing sibling case cannot prove a passing case.
Each case must pass on the pull request head.
It must also fail an assertion on the merge base.
For this run, the job puts the test files, `testdata` fixtures, and Go dependency files of the pull request on the base.

These results fail the proof: a pass on the base, a compile error, a panic, a timeout, or a skip.
A case that does not run also fails.
An existing regression name is not a new case.
`TestFeature` cases run in the normal test suite and need no proof.
A pull request without new regression names needs no proof.

When the package does not compile on the base, the job runs the case again.
The second run keeps the pull request files that declare the regression test.
The other test files of the package come from the base, and test files that the base does not have are removed.
A case that does not compile in the second run fails.
The job never replaces base production files with pull request production files.
It rejects test-file and fixture symlinks.

The job runs `python .github/scripts/regression_proof.py "$BASE_COMMIT"` with Python 3.12, the Go version in `go.mod`, and full Git history.
Test files and fixtures must be tracked by Git.
The helper creates and removes a temporary base snapshot.
It does not change the working tree or the Git index.

A pull request from `dev` into `main` in this repository uses the `--per-change` option.
A later `dev` change can change an API that an earlier regression test calls, so that test does not compile against `main`.
With this option, the helper finds the first-parent `dev` change that adds each new regression case.
It proves the case at that change against the parent of that change.
A case that no change adds and proves fails.

## Sonar

CI skips the Sonar jobs for pull requests from other repositories, because those pull requests have no access to the token.
A fork also skips them, because the Sonar project belongs to `OpenRAE/go-rae-gremlins`.
`.ground-control.yaml` and `sonar-project.properties` set the same Sonar project key.
