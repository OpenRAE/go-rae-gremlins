# Repository guidance

This is the OpenRAE fork of Gremlins, a Go mutation testing CLI. Keep the
upstream module path `github.com/go-gremlins/gremlins`; `origin` points to
`OpenRAE/go-rae-gremlins`. Pull requests target `main`.

Read [CLAUDE.md](CLAUDE.md) for package responsibilities and
[contributing](docs/CONTRIBUTING.md) for contribution conventions. The Go
version in `go.mod` is authoritative. Preserve upstream compatibility and
reuse the existing Go packages and tooling.

Use `go test -race ./internal/<package> -run '<test>'` for the changed behavior.
Run `make policy` for repository checks and `make lint` for Go linting. CI owns
the full `make test` / `make cover` runs and mutation testing. Do not run the
full test suite locally. For behavior changes, demonstrate the regression and
use mutation testing where practical. Configuration and documentation changes
need their relevant validation rather than unrelated Go tests.

Keep credentials and local MCP settings out of commits. Do not read credential
values while diagnosing setup. Do not introduce services or send source to a
new provider without approval. Preserve existing local changes and the shared
Git hook dispatcher. Never bypass hooks or secret scanning. Commit only the
files belonging to the issue. The user owns PR merges and changes into `main`.

## Ground Control context

Ground Control is optional maintainer tooling. Repository commands and context
live in `.ground-control.yaml`, plan rules in `.gc/plan-rules.md`, and decisions
in `architecture/adrs/`. The MCP context tool is
`gc_get_repo_ground_control_context`. Keep architecture references aligned with
the Go code; do not copy another repository's configuration.

See [maintainer setup](docs/MAINTAINER_SETUP.md) for local registration and
verification. Contributors may use ordinary Git, GitHub, and Go commands.
