# Maintainer setup

Ground Control is optional tooling for maintainers of
`OpenRAE/go-rae-gremlins`. Contributors can use ordinary Git, GitHub, and the
Go commands in the Makefile. Pull requests target `main`; the user approves
merges. The upstream Go module path remains `github.com/go-gremlins/gremlins`.

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

For a standard clone, install the hook with `pre-commit install`. If
`git config --get core.hooksPath` names a shared dispatcher, preserve it. This
maintainer host already delegates to `.git/hooks/pre-commit`; its existing hook
loads this configuration. Do not replace the shared hooks or unset
`core.hooksPath`. For another shared setup, use its documented local hook
installation procedure. Ground Control also runs the configured pre-commit
command before committing.

To run the history scan explicitly:

```sh
pre-commit run gitleaks-history --hook-stage manual --all-files
```

## Local Ground Control registration

Install a released `grndctl` package and the workflow skills using Ground
Control's installation instructions. Do not build or modify a sibling checkout
that another agent is using. The maintainer host already has `grndctl`, Node.js,
`gh`, and `codex`; use `grndctl doctor` to check supported versions and GitHub
authentication.

From this repository root, run:

```sh
grndctl init
grndctl doctor
```

`init` keeps the committed `.ground-control.yaml`, updates only the
`ground-control` entry in local `.mcp.json`, and preserves existing `.env`
values. Review its preview before confirming. The server entry is:

```json
{
  "type": "stdio",
  "command": "grndctl",
  "args": ["mcp"]
}
```

Both `.env` and `.mcp.json` are ignored. Configure only credentials for approved
review providers, following the installed release's documentation. Do not copy
another repository's credentials. SonarCloud is not configured for this fork.
Restart the agent session after registration. Clients that do not read
`.mcp.json` need the equivalent stdio server entry in their own MCP settings,
with this repository as the working directory.

Settings and plan rules are committed so agents share the same commands and
repository identity. `make policy` checks their YAML syntax;
`grndctl doctor` additionally validates Ground Control's schema, repository
identity, and local server registration. CI does not require Ground Control or
review-provider credentials.

## Architecture references

The package responsibilities and maintainer-tooling boundary are recorded in
[`ADR-001`](../architecture/adrs/001-optional-maintainer-tooling.md) and the
`architecture.vocabulary` section of `.ground-control.yaml`. Update them when
package ownership changes. Native compilation, dependency inspection, and Go
tests remain the source of implementation evidence.

The configuration supports the installed Ground Control 1.8.1 release. Newer
development checkouts may require additional design-validation commands,
acceptance adapters, or module mappings. Upgrade the installed release and
configuration together before using those features; do not copy settings from
an unreleased sibling checkout.
