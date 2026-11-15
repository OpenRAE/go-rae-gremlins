# Memory-budget architecture preflight (#127)

Status: design guidance; no runtime protection or allocator experiment is delivered here.
Reviewed repository revision: `425a996`.
The issue body supplies the contract; no formal requirement is attached.

## Execution boundary and outcome

Use an established Linux cgroup-v2 runner to bound one complete Gremlins invocation before any target code starts.
The reference setup is an existing systemd manager running a transient **service**, with these explicit properties:

- `MemoryMax`: a finite, positive, operator-selected aggregate budget.
- `MemorySwapMax=0`: no swap allowance for this workload.
- `OOMPolicy=kill`: terminate the entire unit on an OOM kill.
- `Restart=no`: do not repeat the failing workload.
- `KillMode=control-group`: terminate remaining descendants when the unit stops.

Use synchronous execution, such as `systemd-run --wait --pipe`, so the caller observes completion and failure.
Require effective memory-controller support and permission to create the bounded service before executing target tests.
Declare the supported systemd version and manager mode in the delivered setup; executable presence alone proves neither capability.
A user manager needs delegated memory control; a system manager must launch the workload as the unprivileged caller.
Do not introduce automatic privilege escalation, install a daemon, or fall back to unbounded execution.
Native macOS and Windows invocations retain their existing behavior and are not covered by this Linux guarantee.

The budget covers Gremlins, coverage, module downloads, compilers, every concurrent mutant, and all test descendants.
Containment begins at launch; moving an already running parent misses children created before the move.
The supervising caller remains outside the affected unit.

It reports a nonzero, explicit resource-exhaustion result from the runner's OOM state after group termination.
Preserve ordinary application exit statuses, including threshold failures; runner failure is separate from Gremlins' exit-code contract.
Do not infer OOM from exit 137, SIGKILL, output text, or a timeout alone.
Retain runner result evidence before collecting the transient unit.
An OOM run is incomplete: do not accept partial or stale JSON output as a successful mutation report.

The kernel's hard limit can have transient overshoot; leave host headroom and bound the number of simultaneous runs.
Choose a budget that includes compiler, cache, and worker overhead; no universal safe default follows from the issue.
Executor output buffers and coverage `CombinedOutput` also consume the aggregate budget; it bounds the parent as well as target allocations.
Use existing `--workers` and `--test-cpu` controls for throughput tuning, without claiming either bounds memory.

## Existing concerns to reuse

| Concern | Canonical incumbent and constraint |
| --- | --- |
| CLI composition | `cmd/unleash.go` composes coverage before the mutation engine. Enclose this whole invocation instead of adding separate limits to each call site. |
| Subprocesses | `internal/coverage/coverage.go` runs downloads and coverage. `internal/engine/executor.go` runs mutant tests. `internal/diff` also invokes Git. All remain inside the outer budget. |
| Cleanup and deadlines | `internal/engine/process_unix.go`, `process_windows.go`, and executor `run` provide existing process cleanup. Process groups are not memory containment. Windows cleanup is best effort. Preserve these timeout paths. |
| Mutation results | Executor `runTests` maps most child exit failures to `Killed` or `NotViable`. Group-wide termination prevents an OOM victim becoming successful efficacy evidence. Do not add a mutation status for runner exhaustion. |
| Configuration keys | `internal/configuration/configuration.go`, `cmd/internal/flags/flags.go`, `cmd/unleash.go`, and `docs/docs/schema/configuration.json` own application settings. Keep the memory budget in the runner configuration. |
| Logger and errors | `internal/log/log.go` owns application console output, including error output under silent mode. Go failures use `%w` and `errors.Is`/`As`. `internal/execution/execution.go` owns threshold exits 10 and 11; main maps other application errors to 1. Keep runner diagnostics outside this mapping. |
| Work directory copy | `internal/engine/workdir/workdir.go` owns module copies and `GOTMPDIR`; `cmd/unleash.go` owns temporary-root cleanup. Copies protect source files, not memory, credentials, or network access. |
| Reports and persistence | `internal/report/internal/structure.go` owns JSON shape. No database or second result schema is needed. Use isolated per-run output paths and consume reports only after successful completion. |
| Workflow | `Makefile`, `.github/workflows/gremlins.yml`, `.github/workflows/ci.yml`, and `.pre-commit-config.yaml` own existing invocation and verification. Reuse one bounded invocation in its supported job and operator guidance. |
| Architecture | `.architecture/model.json`, `.architecture/design.json`, `scripts/check-design.sh`, and ADR 0004 own design validation. No new Go package, dependency, model rule, or baseline allowance is needed for this runner boundary. |

The next variation is a different aggregate budget or worker count.
Put the budget parameter at the runner invocation, with one documented units/default policy shared by CI and operator usage.
Reject missing, zero, unlimited, malformed, or excessive budgets before starting target code.
Use the runner's native quantity parser; any wrapper should only narrow its accepted finite range, without creating another application configuration schema.
Per-mutant limits and continuation after one mutant exhausts memory require a separate decision and result-propagation design.

## Security and whole-repository gates

| Layer | Required guardrail |
| --- | --- |
| Configuration and environment | Viper binds `GREMLINS_*` names by replacing dots and hyphens with underscores. Do not add an undocumented override or bypass this binding with a new application key. The JSON schema provides editor guidance; it is not a runtime validator. |
| Manager environment | Transient services do not inherit the caller's complete environment or working directory. Preserve the selected Go toolchain, working directory, writable temporary/cache paths, and required non-secret settings explicitly. Avoid silently testing another module or toolchain. |
| Arguments and parsing | Pass executable and arguments separately. Avoid shell interpolation, `eval`, or runner-side environment expansion of user arguments. Treat spaces, dollar signs, and leading dashes as data. |
| Authorization and secrets | No HTTP auth/controller/DTO layer exists here. Local OS permissions and CI permissions are the authorization boundaries. Target tests execute code: keep credentials, manager sockets, and writable cgroup controls unavailable to them. Do not forward the whole host environment, secret files, or tokens through argv or unit properties. |
| OS enforcement | Verify finite effective limits, swap policy, group OOM behavior, and inherited descendant membership. Do not delegate cgroup management to target tests or grant OOM immunity. A process can leave a process group; that must not remove its cgroup limit. |
| CI policy | Preserve pinned actions, `contents: read`, and `persist-credentials: false`. Run the allocator check without secrets in existing CI. Verify manager/controller availability on the actual runner; `ubuntu-latest` alone is insufficient evidence. Missing capability must fail the required bounded job. |
| Diagnostics | Keep the runner's OOM result visible even when Gremlins uses `--silent`. Report the budget and bounded operation, without dumping environment, source, credentials, or arbitrary host journal contents. No new error-envelope hierarchy is needed. |
| Storage and lifecycle | SIGKILL bypasses Go defers. Own temporary-root cleanup outside the killed unit, with paths confined to that run. Stop the unit on cancellation, bound shutdown waits, and remove only its own resources. Do not claim mutation rollback or report completion after OOM. |
| Distribution | `Dockerfile` defines a nonroot image but sets no runtime memory policy. `.goreleaser.yaml` and `docs/development/release.md` show no fork image publication. The upstream image in `docs/docs/usage/ci/docker.md` is not a fork artifact. Do not substitute it or publish an image. |

The existing mutation workflow currently runs `go run ./cmd/gremlins unleash --silent` without a budget.
Documentation alone does not deliver #127: the supported execution setup and its controlled proof must actually run through the bounded entry point.
Keep source and logs local or in established repository CI; do not add an external runner or upload destination.

## Proof obligations and non-goals

Follow T01–T10 in [the Go test standard](test-standard.md), especially process ownership, platform evidence, and honest measurement.
Table-driven tests and existing `WithExecContext`/`NewWithCmd` seams remain appropriate if application code changes become necessary.
Mocked command arguments cannot prove kernel enforcement.

The acceptance experiment needs a small controlled allocator inside an independently bounded test environment.
Establish and verify the outer safety bound before starting the allocator; use a smaller inner budget with ample outer headroom.
Exercise a descendant allocator, including a changed process group, to prove child-tree coverage.
Verify that both coverage and mutant commands inherit the enforced boundary.

Retain and touch allocated pages; virtual reservation alone does not prove the memory controller's behavior.
Distinguish fixture compilation from allocator execution so compiler exhaustion cannot satisfy the experiment accidentally.
Prove group termination, OOM-specific runner evidence, nonzero completion, cleanup, and rejection of successful partial reports.
Also distinguish normal success, application threshold failure, ordinary test failure, timeout/cancellation, and unavailable enforcement.

Use finite allocation/time limits and bounded waits; never size an experiment to exhaust the host.
A skipped capability test is not acceptance evidence; record the actual supported environment and result.
Use targeted checks locally and existing hosted verification; assess mutation testing where executable repository logic changes.
No allocator experiment has been run during this preflight.

Avoid custom RSS polling, PID enumeration, per-process `ulimit`/address-space limits, or `GOMEMLIMIT` as child-tree enforcement.
Avoid soft limits alone, OOM-killer disabling, or a container limit that permits partial OOM kills and continued reporting.
Do not create a generic process-runner framework, duplicate worker pool, exception hierarchy, or configuration schema.
No new mutation type, CLI flag, application key, report field, exit code, per-mutant recovery policy, or cross-platform containment claim is in scope.
This memory boundary does not promise filesystem, network, disk, CPU, or hostile-code isolation.
No downstream repository edits, release, hosting, or upstream import is authorized by this preflight.

## Decision references

- [Linux cgroup-v2 memory controller](https://docs.kernel.org/admin-guide/cgroup-v2.html#memory-interface-files): hard limits, descendant accounting, group OOM, and event evidence.
- [systemd resource-control reference](https://github.com/systemd/systemd/blob/main/man/systemd.resource-control.xml): `MemoryMax` and `MemorySwapMax`.
- [systemd service reference](https://github.com/systemd/systemd/blob/main/man/systemd.service.xml): `OOMPolicy=kill` and the `oom-kill` result.
- [systemd-run reference](https://github.com/systemd/systemd/blob/main/man/systemd-run.xml): service environment, synchronous waiting, and exit propagation.
- [Docker resource constraints](https://docs.docker.com/engine/containers/resource_constraints/): no default limits and hard versus soft budgets.

These references establish the proposed mechanism, not successful enforcement on a particular host.
ADR 0004 governs design evidence; ADR 0002 governs the binary/distribution boundary.
ADR 0003 applies only if implementation imports upstream changes.

## Preflight verification

`make architecture-design` passed with `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off` and a temporary Go build cache.
The available local Go version was `go1.27.3 linux/amd64`.
Model SHA256: `f50ccf387b1870da4a12fa781ffc2ba38db3dad4ed6d0ced9b70e19f3e1c41f0`.
Rules: ARCH001 (ownership), ARCH002 (imports), ARCH003 (contracts), and ARCH004 (development-check trust).
The design declares no application package or contract changes; OS enforcement and runner capabilities remain explicit review and delivery obligations.
This check does not prove runtime containment or acceptance-test success.
