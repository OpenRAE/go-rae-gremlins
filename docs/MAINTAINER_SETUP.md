# Maintainer setup

This guide sets up the local checks for maintainers of `OpenRAE/go-rae-gremlins`, and it describes the maintainer tooling.
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

## Maintainer tooling

The maintainer uses private Ground Control tooling with this repository.
Its source and installation are not public, and this repository gives no installation steps.
The tooling reads `.ground-control.yaml` and `.gc/plan-rules.md`.
Git ignores `.env` and `.mcp.json`, which hold the local credentials and server settings.
Do not commit them.

Jobs that need an OpenRAE secret, service, person, or branch setting run only in this repository.
A fork skips them; see [Forks](development/checks.md#forks).

When an agent run finds a `GitGuardian Security Checks` finding, the finding belongs to the user.
Report the check name, the status, and the URL, then stop until the user resolves the finding.

## Configuration reference

The settings and the plan rules are committed, so all agents use the same commands and repository identity.

| Item | Location and check |
| --- | --- |
| Ground Control settings | `.ground-control.yaml`. `make policy` checks its YAML syntax. |
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
