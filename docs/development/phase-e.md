# Automatic Phase E finalization

The [Phase E workflow](../../.github/workflows/ground-control-phase-e.yml) runs after a PR merges into `dev` or `main`.
It calls Ground Control's existing merged-PR finalizer to post the delivery report and close the delivered issue.
It does not merge PRs or execute delivered Go code, tests, hooks, or workflow commands.

The finalizer finds the issue from the `<issue>-<slug>` head branch of the merged PR.
The issue must have a pickup record for that branch from a user with write access.
`/implement` and `/quickfix` write this record when they start work on the issue.
A PR without such a branch and record is a successful no-op, for example a `dev` to `main` promotion.

## Configure release access

Add the Actions secret `GROUND_CONTROL_READ_TOKEN` to `OpenRAE/go-rae-gremlins`.
Use a fine-grained token restricted to `autarchy-ai/Ground-Control` with **Contents: read-only** and its mandatory metadata read access.
Authorize organization access if required. Do not reuse an operator-wide write token.
The finalizer needs no Sonar access.

In the Phase E workflow, the release token is available only to the download step.
CI's `Phase E release` job also reads this secret to test transport changes. That job checks out no code.

The finalizer uses this repository's automatic token for evidence reads, issue comments, and closure.
No write access to repository contents or PRs is granted.
The local developer `.env` is not copied or changed.
A missing download credential fails with a setup message; it does not silently skip finalization.

## First installation and fork deliveries

Current context: the Phase E workflow and the fork dispatcher are on `dev` and `main`, so the first installation is complete.
Manual dispatch and automatic fork finalization are available.
The steps below record how the first installation worked.

Merge the workflow through the normal reviewed PR into `dev`.
The installation PR can have `finalization: reentry` because its base did not yet contain the workflow.
Re-invoke its `/implement` or `/quickfix` command after merge to finish that delivery.
Later deliveries use automatic finalization once their base includes the workflow and release access is configured.

Manual dispatch becomes available after the workflow reaches the default branch, `main`, through an approved promotion.
GitHub restricts secrets and write tokens on ordinary fork PR events.

The [fork dispatcher](../../.github/workflows/ground-control-fork-finalization.yml) handles merged fork PRs through a trusted merge-only event.
It checks out no source and reads no private secret. It can only dispatch this repository's fixed Phase E workflow on `main`.
The dispatcher has PR-read and Actions-write access; it has no issue-write access.
The finalizer runs under the supported `workflow_dispatch` event with its original permissions.

Automatic fork finalization requires the Phase E definition on `main`, just like manual dispatch.
No Ground Control source change is needed.
Merges made with `GITHUB_TOKEN` can suppress follow-up events; use the same recovery paths.

## Recover a failed run

Inspect the failed **Ground Control Phase E** run and its issue failure record.
Repair the named cause, such as expired release access or unavailable evidence.
Rerun that workflow run, or dispatch it from `main` with the merged PR number:

```shell
gh workflow run ground-control-phase-e.yml --repo OpenRAE/go-rae-gremlins --ref main -f pr=123
```

Replace `123` with the delivered PR number.
The workflow checks the live merge state and base, then checks out its immutable merge revision.
An unmerged PR or a different base is refused.
The same finalizer handles automatic execution, repair, and retries without duplicating final reports.
Local `/implement <issue>` or `/quickfix <issue>` reentry remains available when Actions cannot run.

## Upgrade the utility

The workflow pins the release version, asset name, SHA256, Node version, and external action commits.
The [installation manifest](../../.github/ground-control/package.json) and [lockfile](../../.github/ground-control/package-lock.json)
pin the utility dependency graph and integrity values.
Change the release tuple and regenerate the lockfile together in a reviewed PR.

Copy the `overrides` of the release's `package.json` into the manifest, because npm applies overrides only from the root package.
Seed the lockfile from the release's own lockfile, so the installed versions match the versions the release tested.
Run `npm audit --omit=dev` on the result, and fix each finding before the PR merges.
Verify the selected release's CLI, workflow recognition, and Actions provenance compatibility.
Never commit the downloaded private tarball or installed dependencies.

Installation uses `npm ci` with lifecycle scripts, audit submission, and package-manager caching disabled.
Transport manifests come from the workflow revision, separately from the delivered revision being repaired.
The [design](phase-e-design.md) records data flow, permissions, integrity, and failure boundaries.

## Verify an upgrade locally

Install the pinned utility in a temporary directory using the committed manifest, lockfile, and checksum-verified tarball.
Use the same disabled lifecycle scripts as the workflow.
Point `GROUND_CONTROL_TEST_ROOT` at the installed `node_modules/grndctl` directory.
Put the release tarball two directories above that directory, or set `GROUND_CONTROL_RELEASE_ASSET` to its path.
Then run:

```shell
make phase-e-test
```

The 16 cases run the actual workflow shell steps and the installed CLI against a stateful GitHub stand-in.
They do not replace a finalizer, authorization, issue-discovery, or provenance function.
The cases cover:

- Both events and both base branches, with the correct issue and report, and retries without duplicate writes.
- The `/implement` and `/quickfix` lanes.
- A `dev` to `main` promotion that has no required checks and is not a delivery.
- Skips for a missing pickup record, a pickup record for another branch, and an untrusted pickup author.
- Refusal of an unmerged PR, of a token without issue-write access, and of a mismatched Actions run.
- Rejection of a tampered download.
- Installation of the pinned executable from the locked manifest.
- Fork dispatch for both base branches.

The stand-in enforces the permission map of each job. Each finalization case uses a fresh checksum-verified, locked installation.
The reference package supplies only the YAML parser.
The cases use no live GitHub credential and change no real issue.
For issue #182, all 16 cases passed against the checksum-verified v1.12.0 installation.
With the earlier workflow and v1.9.0, the 11 finalization, promotion, and skip cases failed.

CI runs the same cases when a PR changes the transport. See [pre-merge transport verification](phase-e-design.md#pre-merge-transport-verification).
Fork and Dependabot PRs get no secrets, so the merge queue runs the cases for them.

Verification evidence: [TAP results](evidence/phase-e-v1.12.0.tap) and [release and file hashes](evidence/phase-e-v1.12.0.json).
