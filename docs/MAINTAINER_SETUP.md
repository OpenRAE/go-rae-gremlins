# Maintainer setup

This guide sets up the local checks and the optional Ground Control registration for maintainers of `OpenRAE/go-rae-gremlins`.
Contributors can use ordinary Git, GitHub, and the Go commands in the Makefile.
They need no Ground Control installation and no review-provider credentials.

The module path is `github.com/OpenRAE/go-rae-gremlins`, as [ADR 0001](adr/0001-module-path.md) records.
Pull requests target `dev`.
The maintainer merges them; see [Hosted checks](development/checks.md#merge-queue).

## Set up the local checks

1. Install the pinned Go toolchain and golangci-lint from `mise.toml`, and the hooks:

   ```sh
   python3 -m pip install pre-commit==4.3.0
   make requirements
   make hooks
   ```

2. If `git config --get core.hooksPath` names a shared dispatcher, keep it.
   Do not replace the shared hooks or unset `core.hooksPath`.
3. Run the commit-stage checks on all files, and a targeted test for your change:

   ```sh
   make policy
   go test -race ./internal/configuration -run Test
   ```

4. Select the package and test name that exercise your change.

`make hooks` runs `scripts/install-hooks.sh`.
The script writes a managed pre-commit hook into the hook path that Git resolves for this clone.
It then makes Git run the hook, which proves that commits reach it, also through a shared `core.hooksPath` dispatcher.
It does not overwrite an unmanaged hook unless you give `--force`.
Ground Control also runs the configured pre-commit command before it commits.

`make policy` runs the commit-stage hooks of `.pre-commit-config.yaml` on all files.
These hooks check file syntax, merge markers, private keys, and unpinned `@latest` installs.
Gitleaks scans the staged diff with redacted output.
The lint, vulnerability, and race test hooks are at the `manual` stage.
CI runs the full checks; see [Hosted checks](development/checks.md).

To scan the full Git history for secrets, run this command:

```sh
pre-commit run gitleaks-history --hook-stage manual --all-files
```

## Register Ground Control locally

1. Install a released `grndctl` package and the workflow skills, as the installation instructions of Ground Control describe.
2. Do not build or change a sibling checkout that another agent uses.
3. From the root of this repository, run these commands:

   ```sh
   grndctl init
   grndctl doctor
   ```

4. Review the preview of `init` before you confirm it.
5. Configure only the credentials for approved review providers, as the documentation of the installed release describes.
6. Do not copy the credentials of another repository.
7. Restart the agent session after the registration.

`grndctl doctor` checks the supported versions of `grndctl`, Node.js, `gh`, and `codex`, and the GitHub authentication.
`init` keeps the committed `.ground-control.yaml`.
It changes only the `ground-control` entry in the local `.mcp.json`, and it keeps the existing `.env` values.
Git ignores both `.env` and `.mcp.json`.
The server entry is:

```json
{
  "type": "stdio",
  "command": "grndctl",
  "args": ["mcp"]
}
```

A client that does not read `.mcp.json` needs the same stdio server entry in its own MCP settings.
Use this repository as the working directory.

## Configuration reference

The settings and the plan rules are committed, so all agents use the same commands and repository identity.

| Item | Location and check |
| --- | --- |
| Ground Control settings | `.ground-control.yaml`. `make policy` checks its YAML syntax. `grndctl doctor` also checks the Ground Control schema, the repository identity, and the local server registration. |
| Plan rules | `.gc/plan-rules.md`. |
| SonarCloud | `sonar-project.properties` and the `sonarcloud` block of `.ground-control.yaml`. CI runs the analysis. |
| Coverage | `make cover` writes `coverage.out` with race detection. CI sends the Go and Python coverage to SonarCloud. |
| CodeQL | The GitHub default setup for CodeQL is on. It analyzes Actions, Go, and Python. Do not add an advanced CodeQL workflow, because it conflicts with the default setup. |
| Secret scanning | Gitleaks at commit time and in the Repository policy workflow. The GitGuardian workflow runs `ggshield`. |
| Gitleaks exception | `.gitleaksignore` has one historical false positive: the Viper test-table key `tvsa.float64.key`. The exception applies only to that commit, path, rule, and line. It does not turn off a rule or exclude a file. |
| Phase E utility | The Phase E workflow and CI install a pinned `grndctl` release; see [Automatic Phase E finalization](development/phase-e.md). This pin does not set the version of the local `grndctl`. |

The pre-commit hooks come from public hook sources at the commits that `.pre-commit-config.yaml` pins, and they run locally.
No external scanning service is added for the hooks.

Codecov is not used.
The fork removed the inherited Codecov upload, because this fork is not configured in Codecov.
CI needs no OIDC token permission for coverage.
To use Codecov again, a decision to configure the fork in that service is necessary.

## Architecture references

[ADR-001](../architecture/adrs/001-optional-maintainer-tooling.md) records the maintainer-tooling boundary.
The `architecture.vocabulary` section of `.ground-control.yaml` names the patterns and helpers that agents read.
Update them when package ownership changes.
Native compilation, dependency inspection, and Go tests stay the source of implementation evidence.

The configuration supports the installed Ground Control release.
A newer release can need more design-validation commands, acceptance adapters, or module mappings.
Before you use those features, upgrade the installed release and the configuration together.
Do not copy settings from an unreleased sibling checkout.
