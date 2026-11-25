# ADR 0010: Run maintainer integrations only in the canonical repository

Status: Accepted when #231 merges.

## Context

Issue #231 asks that a fork or clone works without Ground Control.
Ground Control is private maintainer tooling, and its source is not public.

Some workflows need a credential, a person, or a service that only `OpenRAE/go-rae-gremlins` has:

- Phase E finalization and the CI transport tests download the private Ground Control release.
- SonarCloud and GitGuardian need repository secrets and the OpenRAE service projects.
- The maintainer gate requires one named maintainer.
- The merge queue policy check reads the live OpenRAE branch rules.
- The release, the `main` to `dev` synchronization, and the queue label write to OpenRAE.

In a fork, these jobs fail or write to the wrong repository.
A same-repository pull request inside a fork also qualified for the private transport download.

GitHub reports a skipped job as a success, also for a required check.
Thus a guard that is false by mistake in the canonical repository removes that gate.

## Decision

- Each maintainer-only job has the condition `github.repository_id == '1441387140'`.
  This ID identifies `OpenRAE/go-rae-gremlins`. A rename or a transfer does not change it, and a fork has a
  different ID.
- When a job has other conditions, the repository condition applies to the full condition.
- The same condition applies to the merge queue policy step of the `Policy` job.
- The `Phase E transport scope` job selects no transport tests outside the canonical repository.
- `PR Gate` requires a successful SonarCloud result for each canonical event except a pull request from another
  repository. It requires a skipped result in all other cases.
- The build, tests, architecture, lint, documentation, mutation, regression proof, and policy hooks have no
  repository condition. They run in every repository.
- A public unit test checks the condition on each maintainer-only job. Each job that reads a secret other than
  `GITHUB_TOKEN` must have it. The test also runs the `PR Gate` script against a table of job results.
- Contributor and agent instructions name only repository commands. They do not tell a reader to install or use
  Ground Control. The maintainer keeps `.ground-control.yaml`, `.gc/plan-rules.md`, and `.github/ground-control/`.

## Consequences

- A fork passes CI without Ground Control, SonarCloud, GitGuardian, or maintainer access.
- In the canonical repository, every gate keeps its current behavior. A missing secret still fails its job.
- A fork does not run SonarCloud, GitGuardian, or the maintainer gate.
- A fork does not create releases or synchronization pull requests.
- A move of the canonical repository to a new repository ID needs a change to this condition.
