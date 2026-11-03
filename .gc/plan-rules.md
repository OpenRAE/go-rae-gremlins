# Plan rules

- Use the GitHub issue's scope and acceptance criteria; keep one issue per PR.
- Target OpenRAE/go-rae-gremlins on main and preserve the upstream Go module path.
- Reuse the existing command, engine, configuration, coverage, and report packages.
- Select local tests for the behavior changed; CI owns the full Go and mutation runs.
- Keep pre-commit fast and retain secret scanning and the shared hook dispatcher.
- Keep credentials and MCP configuration local; add no provider or service without approval.
- Preserve unrelated local work; stage only the issue's files.
- Obtain the user's approval before merging a PR or changing main.
