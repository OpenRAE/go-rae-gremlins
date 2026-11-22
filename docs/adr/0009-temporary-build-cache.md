# ADR 0009: Use a temporary Go build cache for each run

Status: Accepted when the pull request for #8 merges.

Design guidance: [build-cache boundaries](../development/build-cache-preflight.md).
User guide: [Go command settings](../docs/usage/commands/unleash/index.md#go-command-settings).

## Context

Each mutant runs in a different copy of the module, so each test run adds new entries to the Go build cache.
Before #8, these entries went to the default build cache of the user.
A mutation run with many mutants thus made the default cache grow without a limit.
Issue #8 asks for a separate temporary build cache that Gremlins removes after each run.

The coverage run sets the memory limit and the fallback timeout of each mutant test run. The fallback timeout also caps the timeout of each package.
An empty cache makes the coverage run also compile the standard library and the dependencies.
That compilation increases the elapsed time and the peak memory of the coverage run.

## Decision

- When the `GOCACHE` environment variable is not set or is empty, the run uses the `gocache` directory in its temporary work directory.
- When `GOCACHE` has a value, the run uses that cache.
- The CLI gives the cache path to the coverage step and to the executor dealer.
- Each `go` command of the run gets this path in `GOCACHE`.
- Before the timed coverage run, `go list -export -deps -test` compiles the packages and their dependencies into the cache.
- The existing cleanup of the work directory removes the cache.
- The CLI makes the work directory path absolute. A relative path would point to a different directory for a `go` command in a module copy.
- Gremlins rejects a work directory inside the module, because the module copies would copy the work directory into themselves.
- Gremlins adds no flag and no configuration key for the cache.

## Consequences

- A run without `GOCACHE` does not add entries to the default build cache.
- A run without `GOCACHE` compiles the standard library and the dependencies again. On one small package, a run took 12.2 s instead of 1.0 s.
- A run with `GOCACHE` keeps a full cache. That cache grows as before.
- The cache uses disk space in the temporary directory during the run. When that directory is a `tmpfs`, the cache uses memory.
- The compilation before the timed coverage run keeps the fallback timeout and the memory limit near their values with a full cache.
- A crash or `SIGKILL` can leave the cache in the temporary directory, as it can leave the module copies.
- `GOCACHEPROG` selects an external cache program. Gremlins does not remove the storage of that program.
