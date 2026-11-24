# Maintainer setup

This page is for the maintainers of `OpenRAE/go-rae-gremlins`. Contributors
use [contributing](CONTRIBUTING.md) and ordinary Git, GitHub, and Go
commands. Pull requests target `dev`; the user approves merges. The Go module
path is `github.com/OpenRAE/go-rae-gremlins`
([ADR 0001](adr/0001-module-path.md)).

## Native checks

Use the Go version from `go.mod` and golangci-lint v2 (`mise.toml` also declares
these tools). Install pre-commit, then validate the repository:

```sh
python3 -m pip install pre-commit==4.3.0
make policy
make lint
go test -race ./internal/configuration -run Test
make build
```

Select the package and test name that exercise your change. `make test`,
`make cover`, and the Gremlins workflow run the full checks in CI. `make cover`
generates `coverage.out` with race detection. The inherited Codecov upload is
removed because this fork is not configured in Codecov; CI needs no OIDC token
permission for coverage reporting. Re-enabling Codecov requires an explicit
decision to configure the fork in that service.

GitHub's default CodeQL setup is enabled for Go on this fork. It owns CodeQL
analysis; a second advanced CodeQL workflow would conflict with that setup.

Pre-commit downloads the public hook sources pinned in
`.pre-commit-config.yaml` and runs them locally. Commit checks validate file
syntax, merge markers, and private keys; Gitleaks scans the staged diff with
redacted output. CI runs file checks and scans the complete checked-out Git
history. No external scanning service is added.

`.gitleaksignore` identifies one historical false positive: a Viper test-table
key named `tvsa.float64.key`. Its exception is limited to that exact commit,
path, rule, and line; it does not disable a rule or exclude a file.

For a clone, install and verify the hook with `make hooks`. If
`git config --get core.hooksPath` names a shared dispatcher, preserve it. This
maintainer host already delegates to `.git/hooks/pre-commit`; its existing hook
loads this configuration. Do not replace the shared hooks or unset
`core.hooksPath`.

To run the history scan explicitly:

```sh
pre-commit run gitleaks-history --hook-stage manual --all-files
```

## Maintainer tooling

The maintainer uses private Ground Control tooling with this repository. Its
source and installation are not public. Its own documentation describes its
installation and registration. This repository gives no installation steps.

The tooling reads `.ground-control.yaml` and `.gc/plan-rules.md`. The Phase E
workflow and the CI transport tests use `.github/ground-control/`.
[Setup and recovery](development/phase-e.md) describes the workflow secret and
recovery.

Both `.env` and `.mcp.json` are ignored. Keep credentials and local server
settings in them, and do not commit them. `make policy` checks the YAML syntax
of the committed settings. CI does not require the tooling.

Jobs that need an OpenRAE secret, service, or branch setting run only in this
repository ([ADR 0010](adr/0010-maintainer-integrations-in-the-canonical-repository.md)).
A fork skips them.

When an agent run finds a `GitGuardian Security Checks` finding, the finding
belongs to the user. Report the check name, status, and URL, then stop for the
user's resolution.

## Architecture references

The package responsibilities and maintainer-tooling boundary are recorded in
[`ADR-001`](../architecture/adrs/001-optional-maintainer-tooling.md) and the
`architecture.vocabulary` section of `.ground-control.yaml`. Update them when
package ownership changes. Native compilation, dependency inspection, and Go
tests remain the source of implementation evidence.
