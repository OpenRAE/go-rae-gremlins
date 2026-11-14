# Memory budget for mutation runs

A mutation run starts many `go test` process trees.
A fast allocating mutant can exhaust the host before the time limit.
`scripts/run-bounded.sh` bounds the whole run with a Linux cgroup-v2 memory controller.
[ADR 0006](../adr/0006-bounded-memory-for-mutation-runs.md) records the decision.

## What it does

The script runs a command inside a systemd transient service.
The service sets a finite memory budget, no swap, and group-wide OOM termination.
The budget covers Gremlins, coverage, compilers, every mutant, and all descendants.
The kernel ends the whole unit when the budget is reached.
A child stays bound even when it starts a new process group.

The script stays outside the unit.
On an OOM result it prints a clear message, deletes any partial output, and exits with code 137.
On any other result it passes the command exit code through unchanged.
So the Gremlins threshold exits 10 and 11 stay intact.

## Requirements

- Linux with the unified cgroup-v2 hierarchy.
- A per-user systemd manager with the memory controller delegated to the user slice.

Start the per-user manager with `loginctl enable-linger "$(id -un)"`.
This needs no sudo, because polkit authorizes a user for their own account.

The script uses only the per-user manager, so the command runs as the unprivileged user.
It never uses sudo, it refuses to run as root, and it never runs the command as root.
It builds the command environment from scratch, so the manager environment and its secrets are not inherited.
It forwards your `GREMLINS_*` configuration and non-secret Go settings, so a bounded run honors the same settings as a direct command.

It verifies a finite memory limit and a zero swap limit before the run.
It rejects a budget that is not smaller than host memory.
It fails with a clear message when no effective controller is available, and never runs the command without a budget.
Native macOS and Windows runs keep their current behavior; this guarantee is Linux only.

## Use

Run a bounded command:

```shell
scripts/run-bounded.sh --budget 8G -- go run ./cmd/gremlins unleash --silent
```

The budget uses the systemd quantity format, for example `512M`, `8G`, or a byte count.
Choose a budget that includes compiler, cache, and worker overhead, and that leaves host headroom.
Use the `--workers` and `--test-cpu` flags for throughput; neither bounds memory.

To drop a stale report after an OOM, pass the Gremlins `--output` path to the wrapper with `--output-file`:

```shell
scripts/run-bounded.sh --budget 8G --output-file report.json -- \
  go run ./cmd/gremlins unleash --output report.json
```

Without the matching `--output-file`, the wrapper cannot delete a partial report.

## Verify the enforcement

```shell
make memcheck
```

`make memcheck` runs `scripts/memory-budget-check.sh`.
The check runs a small controlled allocator inside an independently bounded environment.
It proves that the budget produces a clear OOM result without exhausting the host.
The CI `memory` job runs the same check and fails when the runner lacks an effective controller.
A skipped check is not acceptance evidence.
