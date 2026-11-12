# ADR 0007: Merge pull requests to dev through a merge queue

Status: Accepted when #151 merges.

## Context

Issue #151 asks for a GitHub merge queue on `dev`.
With strict status checks, each merge into `dev` makes the other open pull requests out of date.
Each update runs the full CI again.
GitHub reports a required check on a merge group only when its producer runs on the `merge_group` event.
The GitGuardian App posts its check only on pull request commits, and a merge group commit has no pull request.
SonarCloud has no merge queue support.

## Decision

- `dev` uses a merge queue in the `merge-queue-dev` ruleset. The queue squashes, and a merge group holds up to five
  pull requests.
- The same ruleset holds the required `dev` checks: `PR Gate`, `Mutation testing`, `Conventional PR title`, and
  `ggshield`. The `maintainer-gate` ruleset requires `maintainer-gate`. Classic branch protection on `dev` has no
  required checks.
- `dev` does not use strict status checks. The queue tests each pull request against the latest `dev`.
- `main` has no merge queue. Merges into `main` stay manual.
- Each producer of a required `dev` check runs on `pull_request` and on `merge_group`. On a merge group, a producer
  compares with the merge group base commit and reads the queued pull request from the queue ref.
- The `ggshield` job in GitHub Actions produces the GitGuardian check. The GitGuardian App stays installed.
- On a merge group, CI runs the SonarCloud pull request analysis and quality gate of the queued pull request.
- The Ground Control readiness step adds a ready pull request that targets `dev` to the queue.
- The `Policy` job runs `.github/scripts/merge_queue_policy.py` against the live branch rules. It fails when a
  required `dev` check has no `merge_group` producer. It accepts non-strict `dev` checks only when `dev` has a merge
  queue.

## Consequences

- A failed check on a merge group removes the pull request from the queue, and the pull request does not merge.
- The policy check reads the strict setting from the rulesets, because a token without admin access cannot read it
  from classic branch protection. A required `dev` check in classic branch protection fails the check.
- A pull request from another repository gets no secrets, so its `ggshield` check fails. The maintainer can push
  its branch to this repository to run the scan.
