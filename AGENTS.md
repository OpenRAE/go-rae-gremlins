# Repository guidance

This is the OpenRAE fork of Gremlins, a Go mutation testing CLI. The module
path is `github.com/OpenRAE/go-rae-gremlins`, as
[ADR 0001](docs/adr/0001-module-path.md) records. `origin` points to
`OpenRAE/go-rae-gremlins`. Pull requests target `dev`.

Read [CLAUDE.md](CLAUDE.md) for package responsibilities and
[contributing](docs/CONTRIBUTING.md) for contribution conventions. The Go
version in `go.mod` is authoritative. Preserve upstream compatibility and
reuse the existing Go packages and tooling.

Use `go test -race ./internal/<package> -run '<test>'` for the changed behavior.
Run `make policy` for repository checks and `make lint` for Go linting. CI owns
the full `make test` / `make cover` runs and mutation testing. Do not run the
full test suite locally. For behavior changes, demonstrate the regression and
use mutation testing where practical.

Configuration and documentation changes need their relevant validation rather
than unrelated Go tests. For Markdown, run `make docs-changed` or
`bash scripts/doc-tools.sh lint <path>`, and use the shared `documentation`
skill. See [documentation](docs/CONTRIBUTING.md#documentation).

Keep credentials and local MCP settings out of commits. Do not read credential
values while diagnosing setup. Do not introduce services or send source to a
new provider without approval.

Preserve existing local changes and the shared
Git hook dispatcher. Never bypass hooks or secret scanning. Commit only the
files belonging to the issue. The user owns PR merges and changes into `dev`
and `main`.

## Maintainer tooling

The maintainer uses private tooling with this repository. Contributors and
agents do not need it. Use the Git, GitHub, Go, and Make commands above.
`.ground-control.yaml`, `.gc/`, and `.github/ground-control/` belong to that
tooling. Change them only when the issue asks for it.

Some workflows run only in `OpenRAE/go-rae-gremlins`. A fork skips them, as
[ADR 0010](docs/adr/0010-maintainer-integrations-in-the-canonical-repository.md)
records.
