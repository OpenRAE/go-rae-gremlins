# Fieldalignment provisioning design — issue #125

> Current context (2026-11-24): This is a historical record of issue #125 at its reviewed revision.
> Current procedure: [Prepare the clone](../CONTRIBUTING.md#prepare-the-clone).
> `make requirements` now installs `FIELDALIGNMENT_VERSION`, not `@latest`.

Issue #125 requires a reviewed fieldalignment version and matching provisioning instructions.
This change belongs to developer provisioning. Existing architecture decisions cover it; no new ADR is needed.

## Decision and ownership

Use `golang.org/x/tools v0.48.0`, the dependency already selected in `go.mod` and recorded in `go.sum`.
The cached module contains `go/analysis/passes/fieldalignment/cmd/fieldalignment/main.go` and declares Go 1.25.0.
This minimum fits the Go 1.26.11 toolchain pinned by `mise.toml` and `go.mod`.

Keep the executable version in one `FIELDALIGNMENT_VERSION` value in [Makefile](../../Makefile), following `GOVULNCHECK_VERSION`.
Use an explicit version suffix and the existing `mise exec --` invocation pattern for installation.
Installing mise tools does not select Go for a later bare `go` command.
At preflight, bare Go was 1.27.4, while mise selected 1.26.11.

The [Go module reference](https://go.dev/ref/mod#go-install) states that version-suffixed installation ignores the current module.
Thus, the checkout's x/tools requirement alone does not pin this installation or its complete dependency graph.
Do not claim that the checkout's `go.sum` authenticates the separate installation.
Preserve Go's normal module authentication and the existing approved provisioning services.

[CONTRIBUTING](../CONTRIBUTING.md#prepare-the-clone) owns contributor setup.
Its provisioning instructions must name v0.48.0 and use `make requirements` as the canonical installation command.
Explain the pin there; do not create a second executable installation recipe.
`CLAUDE.md` already directs contributors to `make requirements`.

The version value is the extension seam for a later reviewed upgrade.
Review compatibility and update the provisioning description when changing it.
Dependabot's Go-module updates do not maintain a Makefile version value.
Keep an intentional difference from the runtime x/tools version explicit; do not derive upgrades from `latest` or shell parsing.

## Boundaries and existing controls

| Layer | Required behavior and canonical owner |
| --- | --- |
| Provisioning and static analysis | `make requirements` installs tools. `scripts/lint.py` owns offline analysis. Do not add installation to lint, hooks, architecture checks, or ordinary builds. |
| Toolchain and environment | `mise.toml` selects Go; use it explicitly. Preserve normal `GOBIN` and `GOPATH` installation behavior. The install destination must be on `PATH` before optional fieldalignment use. |
| Secrets and authorization | The public tool needs no new token or service. Do not read credential files, log the environment, or put credentials in arguments. Existing secret hooks remain enabled. |
| Filesystem and persistence | Installation writes the executable and Go caches, not checkout source. Use a temporary `GOBIN` for verification. Avoid privilege escalation or a new tool store. |
| Errors and observability | Preserve Go and mise diagnostics and Make's nonzero failure propagation. No ignored errors, successful fallback to `latest`, runtime Logger adapter, or new exception type. |
| Configuration and schemas | `internal/configuration`, `GREMLINS_*`, and the configuration JSON schema belong to runtime behavior. Provisioning adds no key, parser, validator, or schema. |
| Runtime contracts | CLI flags, report JSON, and `internal/execution.ExitError` remain unchanged. No auth controller, HTTP error envelope, service, or database participates. |
| Architecture validation | ADR 0004 and `.architecture/model.json` own strict design validation. Declare `build`, `contributing`, and `hooks` policy coverage in `.architecture/designs/125.json`. Keep all modeled trust boundaries unchanged. |
| Existing delivery checks | Preserve the existing hooks, `scripts/install-hooks.sh`, `make policy`, `make lint`, and CI permissions. Add one `pygrep` hook to `.pre-commit-config.yaml` that rejects `@latest` in the Makefile, the scripts, and the workflows. No workflow or baseline change is needed. |

The intended flow is contributor → Make → mise-selected Go → approved module download/cache → installed executable.
Source rewriting remains a separate, explicit contributor action through `make fieldalignment`.
The [coding standard](coding-standards.md) treats this command as an optional rewrite, separate from formatting and lint compliance.
Installing the tool must never run `fieldalignment -fix ./...` or rewrite byte-exact `testdata` inputs.

## Verification and scope

Review the expanded `make -n requirements` command and the provisioning instructions for the same exact version.
For installation evidence, use a temporary binary destination and inspect it with `go version -m`.
Check the recorded module path, v0.48.0 version, and build Go version without running the analyzer.
An existing `fieldalignment` elsewhere on `PATH` does not prove that provisioning installed the selected version.
Confirm that installation leaves tracked source, `go.mod`, and `go.sum` unchanged.
The design check validates policy declarations; it does not prove successful installation or tool compatibility.

Run the existing architecture design check and explicit-path documentation checks for the guidance changes.
Implementation verification uses the provisioning checks above and existing repository gates.
No runtime regression test, full local Go test suite, or mutation run is needed for a version-only provisioning change.
Do not introduce a test framework or a permanent download job to check one fixed command.
The `@latest` hook is the regression gate. It reads tracked files and downloads nothing.

Non-goals include dependency upgrades, a `go tool` migration, a tools module, new scripts, analyzer execution, and formatting cleanup.
Runtime logging, errors, mutation types, work-directory isolation, reports, and downstream contracts require no changes.
Existing guidance disagrees on module identity and PR targets; this issue changes neither and must follow the user's applicable instructions.

## Preflight evidence

Inspected checkout: `bd52c78`, before implementation. `make -n requirements` still selects `@latest`.
`make architecture-design` passed ARCH001–ARCH004 with the mise-pinned Go 1.26.11 and module downloads disabled.
Model SHA-256: `22337dd5b0fb2bc612c0db132f9807dbbd53055daec6c7ee5ddfd2c60ccf31f1`.

The preflight design covered the `build` and `contributing` policies, with no package or runtime contract changes.
The final design adds the `hooks` policy for the `@latest` gate.
The explicit-path documentation check passed with zero findings.
No tool installation, analyzer execution, or implementation change occurred during preflight.
