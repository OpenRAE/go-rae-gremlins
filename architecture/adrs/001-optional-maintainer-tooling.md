# ADR-001: Optional maintainer tooling

Status: Accepted

## Context

This fork uses the upstream Go module `github.com/go-gremlins/gremlins`.
Maintainers want Ground Control to work with the fork's GitHub issues and PRs
without changing the application or requiring contributors to install it.

## Decision

Keep the Go packages and their existing dependencies. The CLI in `cmd/`
coordinates configuration and the mutation engine in `internal/`. Coverage,
execution, mutation, diff, and report packages retain their existing roles.
The architecture vocabulary in `.ground-control.yaml` identifies the existing
implementation examples that agents should read; this decision applies
repository-wide. Use the schema supported by the installed release rather than
unreleased settings from another checkout.

```mermaid
flowchart LR
    Contributor --> Make[Go commands in Makefile]
    Maintainer --> GC[Local Ground Control MCP server]
    GC --> Config[Repository configuration and documentation]
    GC --> Make
    GC --> GitHub[Existing fork issues and pull requests]
    Make --> Go[Existing Go packages]
    CI[GitHub Actions] --> Make
    Commit[Git commit through existing hook dispatcher] --> Hooks[pre-commit checks]
    Hooks --> Scanner[Local secret scanner]
```

Commit `.ground-control.yaml`, `.gc/plan-rules.md`, architecture references,
and setup documentation. Use the fork's `origin` identity and `main` base.
Keep `.env` and `.mcp.json` local and ignored. The MCP server runs locally;
review credentials and any source processing follow the user's approved
provider configuration. Onboarding provisions no service or credential.

Reuse pre-commit and Gitleaks rather than building a hook framework. Pin hook
repositories to commits. Commit checks cover staged files; CI also scans Git
history. The shared `core.hooksPath` dispatcher must continue to work.
Repository policy checks use the same hook configuration locally and in CI.
Full Go tests, race detection, and mutation testing remain CI responsibilities;
local implementation uses tests for the changed behavior.

Generate Go coverage in the test matrix. Remove the inherited Codecov uploader
and its configuration because Codecov has no repository configured for this
fork. Do not grant CI an OIDC token permission for an unused integration.
Any future Codecov setup requires an explicit decision to enable the service
for the fork; test failures continue to fail CI independently of reporting.

Ground Control is optional. Native Git and Go commands remain supported.
Merging PRs and changes into `main` requires the user's approval.

## Consequences

The application gains no dependency. Maintainers install Ground Control
separately and register it locally. Contributors need pre-commit for the
repository checks but do not need Ground Control, Node.js, SonarCloud, or
review-provider credentials. Changes requiring features of a newer Ground
Control release need an explicit configuration upgrade after that release is
installed; onboarding does not upgrade host tooling or modify its source.
