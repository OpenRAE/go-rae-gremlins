# Unleash

The `unleash` command runs a mutation test of a Go module. The aliases are `run` and `r`.

```shell
gremlins unleash [path] [flags]
```

`path` is a directory in a Go module. The default is the current directory.
Gremlins finds the module root from `path` and tests the mutants in `path` and its subdirectories.

```shell
gremlins unleash
gremlins unleash ./internal/engine
```

If the module build needs tags, use [`--tags`](#tags):

```shell
gremlins unleash --tags "tag1,tag2"
```

## Run steps

1. Gremlins runs the tests of the scanned directory once with a coverage profile. The tests must pass.
2. It loads the types of the packages, to select only the mutants that can build.
3. It finds the mutants and gives each one a [status](#mutant-statuses). See [Coverage](../../coverage.md).
4. It applies each runnable mutant to a copy of the module and runs `go test`. Mutants run in parallel. See [Workers](workers.md).
5. It writes the report to the console and, if set, to the [output file](#output).

Gremlins does not look for mutants in test files, `testdata` and `vendor` directories, nested modules, and files or directories whose names start with `.` or `_`.

## Mutant statuses

| Status | Filter letter | Meaning |
| --- | --- | --- |
| `RUNNABLE` | `r` | In [dry-run mode](#dry-run), the tests cover the mutant. Gremlins did not test it. |
| `NOT COVERED` | `c` | The tests do not run the code of the mutant. Gremlins does not test it. |
| `SKIPPED` | `s` | The mutant is outside the [`--diff`](#diff) changes, or [coverage cannot show](../../coverage.md#code-that-coverage-cannot-show) its code. Gremlins does not test it. |
| `KILLED` | `k` | A test failed with the mutant. |
| `LIVED` | `l` | All the tests passed with the mutant. |
| `TIMED OUT` | `t` | The test run took longer than its [timeout](#timeout-coefficient), so Gremlins stopped it. |
| `NOT VIABLE` | `v` | The module does not build with the mutant. |
| `MEMORY ERROR` | `m` | The test run used more memory than its [limit](#memory-budget), so Gremlins stopped it. |
| `EQUIVALENT` | `e` | An [annotation](../../equivalent-mutants.md) marks the mutant as equivalent. Gremlins does not test it. |

A `TIMED OUT` mutant often stops a test, but the status does not show that a test detected it.

## Go command settings

Gremlins runs `go test` for the coverage step and for each mutant.
Each `go` command gets the environment of Gremlins, so it uses the `GOFLAGS` environment variable.
Use `GOFLAGS` to give the same build flags to all test runs.
For example, monkey-patching libraries need tests that the compiler builds without inlining:

```shell
GOFLAGS='-gcflags=all=-l' gremlins unleash
```

This example uses POSIX shell syntax.
For the `GOFLAGS` format, see the [Go command documentation](https://pkg.go.dev/cmd/go#hdr-Environment_variables).
Use the [`--tags`](#tags) flag for build tags. Gremlins does not read build tags from `GOFLAGS` when it finds mutants.

When the `GOCACHE` environment variable is not set, each run uses a new Go build cache in its temporary directory.
Thus the run does not add to your default build cache.
Gremlins removes the cache at the end of the run, also after an error or a cancellation.
Each run compiles the standard library and the dependencies again.
To use a full cache and get faster runs, set `GOCACHE`. That cache then grows as before.

The cache can use much disk space during the run.
Set the temporary directory (`TMPDIR` on Linux and macOS) to a location with space.
The temporary directory must be outside the module.

## Cancellation

An interrupt (`SIGINT`) or a termination signal (`SIGTERM`) stops the run.
Gremlins stops the coverage step and the running tests, and removes its temporary files.
It does not write a report. It reports the cancellation as an error and exits with code 1.
More signals during the shutdown do not stop it abruptly.

## Flags

Each flag has a [configuration key](../../configuration.md#reference).
A flag name also accepts `_` or `.` in place of `-`. For example, `--dry_run` is `--dry-run`.

### Arithmetic base

:material-flag: `--arithmetic-base` · :material-sign-direction: Default: `true`

Enables or disables the [ARITHMETIC_BASE](../../mutations/arithmetic_base.md) mutation type.

```shell
gremlins unleash --arithmetic-base=false
```

### Conditionals-boundary

:material-flag: `--conditionals-boundary` · :material-sign-direction: Default: `true`

Enables or disables the [CONDITIONALS_BOUNDARY](../../mutations/conditionals_boundary.md) mutation type.

```shell
gremlins unleash --conditionals-boundary=false
```

### Conditionals negation

:material-flag: `--conditionals-negation` · :material-sign-direction: Default: `true`

Enables or disables the [CONDITIONALS_NEGATION](../../mutations/conditionals_negation.md) mutation type.

```shell
gremlins unleash --conditionals-negation=false
```

### Cover packages

:material-flag: `--coverpkg` · :material-sign-direction: Default: empty

A comma-separated list of package patterns. Each test run records the coverage of the packages that match.
Gremlins gives the value to the `-coverpkg` flag of the coverage run.
When it is empty, each test records the coverage of its own package only.

```shell
gremlins unleash --coverpkg "./internal/...,./pkg/..."
```

### Exclude files

:material-flag: `--exclude-files`/`-E` · :material-sign-direction: Default: empty

A Go regular expression. Gremlins does not find mutants in a file whose path matches it.
The path is relative to the scanned directory and uses `/`.
Excluded files have no mutants, so the thresholds do not count them.
Gremlins always excludes test files.
To exclude one mutant and not a file, see [equivalent mutants](../../equivalent-mutants.md) and [ignore mutants](../../ignore-mutants.md).

```shell
gremlins unleash --exclude-files "_(gen|wrap)\.go$"
```

Use the flag more than once to give more expressions. Gremlins excludes a file that matches one of them.

```shell
gremlins unleash -E "_(gen|wrap)\.go$" -E "^(generate|wrap)/" -E "internal/super_old/"
```

An expression that does not compile stops the run with an error.

### Diff

:material-flag: `--diff`/`-D` · :material-sign-direction: Default: empty

A Git branch or commit. Gremlins tests only the mutants in the code that changed since the merge base of this reference and the current commit.
The changes include the changes that are not committed.
Other covered mutants get the `SKIPPED` status.
When it is empty, Gremlins tests all the covered mutants.

Gremlins reads only the changes in the directory that it scans.
The module can be in a subdirectory of the Git repository.
A change outside the scanned directory selects no mutant.

#### Branch merge base

```shell
gremlins unleash --diff "origin/main"
```

#### Commit

```shell
gremlins unleash --diff "b62af323"
```

#### PR

```shell
gremlins unleash --diff "origin/$GITHUB_BASE_REF"
```

The checkout must include the history of the base branch.
For GitHub Actions, see [Test only the changes of a pull request](../../ci/github-action.md#test-only-the-changes-of-a-pull-request).

### Dry run

:material-flag:`--dry-run`/`-d` · :material-sign-direction: Default: false

Finds and reports the mutants, but does not run the tests of each mutant.
The coverage run still runs the tests of the module, because the report needs the coverage of each mutant.
A dry run does not assess the thresholds.

```shell
gremlins unleash --dry-run
```

### Statuses output

:material-flag: `--output-statuses`/`-S` · :material-sign-direction: Default: empty (show all)

Prints only the mutant lines with these statuses to the console. Give one [filter letter](#filter-letters) for each status.
The flag does not change the summary, the JSON output file, or the thresholds.
For an invalid letter, Gremlins prints a warning and shows all the statuses.

#### Examples

##### Show only `LIVED` and `NOT COVERED`

```shell
gremlins unleash --output-statuses "lc"
```

Output:

```text
       LIVED CONDITIONALS_BOUNDARY at aFolder/aFile.go:12:3
 NOT COVERED CONDITIONALS_BOUNDARY at aFolder/aFile.go:12:3
```

##### Filter out `SKIPPED` and `KILLED`

```shell
gremlins unleash -S lctmvre
```

Output:

```text
       LIVED CONDITIONALS_BOUNDARY at aFolder/aFile.go:12:3
 NOT COVERED CONDITIONALS_BOUNDARY at aFolder/aFile.go:12:3
  NOT VIABLE CONDITIONALS_BOUNDARY at aFolder/aFile.go:12:3
   TIMED OUT CONDITIONALS_BOUNDARY at aFolder/aFile.go:12:3
```

#### Filter letters

The filter letters are in the [mutant statuses](#mutant-statuses) table: `l`, `c`, `t`, `m`, `k`, `v`, `s`, `r`, and `e`.

### Diff statuses output

:material-flag: `--output-diff-statuses` · :material-sign-direction: Default: empty (show no diff)

Prints a unified diff of the original and the mutated code for each printed mutant with these statuses.
Only `l` (`LIVED`) and `k` (`KILLED`) are valid, because only a mutant that ran has a result to compare.
For an invalid letter, Gremlins prints a warning and shows no diff.

The flag works with `--output-statuses`.
A mutant that `--output-statuses` hides has no output: no status line and no diff.

#### Examples

##### Show diff for survived mutants

```shell
gremlins unleash --output-diff-statuses l
```

Output:

```text
       LIVED CONDITIONALS_BOUNDARY at aFolder/aFile.go:12:3
       -x > y
       +x >= y
```

##### Show diff for both lived and killed mutants

```shell
gremlins unleash --output-diff-statuses lk
```

### Increment decrement

:material-flag: `--increment-decrement` · :material-sign-direction: Default: `true`

Enables or disables the [INCREMENT_DECREMENT](../../mutations/increment_decrement.md) mutation type.

```shell
gremlins unleash --increment-decrement=false
```

### Integration mode

:material-flag:`--integration`/`-i` · :material-sign-direction: Default: false

In normal mode, Gremlins runs only the tests of the package of the mutant. Each test run is then shorter.
Gremlins does not see when a mutant in one package makes the tests of another package fail.
Usually, this is acceptable, because each package must have its own tests.

In integration mode, Gremlins runs the tests of all the packages for each mutant.
Use it, for example, for integration or end-to-end tests in a different package.
Integration mode changes these settings:

- The coverage run uses all the packages of the module, not only the scanned directory.
- Each test run uses the fallback [timeout](#timeout-coefficient). The adaptive timeout does not apply.
- Gremlins halves the [workers](#workers) and the [test CPU](#test-cpu) values that are more than 1.

Integration mode is usually much slower. The results can be different from normal mode.

```shell
gremlins unleash --integration
```

### Invert assignments

:material-flag: `--invert-assignments` · :material-sign-direction: Default: `false`

Enables or disables the [INVERT_ASSIGNMENTS](../../mutations/invert_assignments.md) mutation type.

```shell
gremlins unleash --invert-assignments
```

### Invert bitwise

:material-flag: `--invert-bitwise` · :material-sign-direction: Default: `false`

Enables or disables the [INVERT_BITWISE](../../mutations/invert_bitwise.md) mutation type.

```shell
gremlins unleash --invert-bitwise
```

### Invert bitwise assignments

:material-flag: `--invert-bwassign` · :material-sign-direction: Default: `false`

Enables or disables the [INVERT_BWASSIGN](../../mutations/invert_bitwise_assignments.md) mutation type.

```shell
gremlins unleash --invert-bwassign
```

### Invert logical operators

:material-flag: `--invert-logical` · :material-sign-direction: Default: `false`

Enables or disables the [INVERT_LOGICAL](../../mutations/invert_logical.md) mutation type.

```shell
gremlins unleash --invert-logical
```

### Invert loop control

:material-flag: `--invert-loopctrl` · :material-sign-direction: Default: `false`

Enables or disables the [INVERT_LOOPCTRL](../../mutations/invert_loop.md) mutation type.

```shell
gremlins unleash --invert-loopctrl
```

### Invert negatives

:material-flag: `--invert-negatives` · :material-sign-direction: Default: `true`

Enables or disables the [INVERT_NEGATIVES](../../mutations/invert_negatives.md) mutation type.

```shell
gremlins unleash --invert-negatives=false
```

### Output

:material-flag: `--output`/`-o` · :material-sign-direction: Default: empty

The file for the JSON report. When it is empty, Gremlins writes no file.
Gremlins resolves a relative path from the current directory.
If Gremlins cannot create or write the file, the command fails with exit code 1.
When the run has no mutants, Gremlins does not write the file, and an old file stays.

```shell
gremlins unleash --output=output.json
```

The file has one JSON object on one line. This example is formatted for reading:

```json
{
  "go_module": "example.com/quickstart",
  "files": [
    {
      "file_name": "discount.go",
      "mutations": [
        {"type": "CONDITIONALS_BOUNDARY", "status": "LIVED", "line": 6, "column": 11},
        {"type": "CONDITIONALS_NEGATION", "status": "KILLED", "line": 6, "column": 11},
        {"type": "ARITHMETIC_BASE", "status": "KILLED", "line": 7, "column": 16},
        {"type": "ARITHMETIC_BASE", "status": "KILLED", "line": 7, "column": 23}
      ]
    }
  ],
  "test_efficacy": 75,
  "mutations_coverage": 100,
  "mutants_total": 4,
  "mutants_killed": 3,
  "mutants_lived": 1,
  "mutants_not_viable": 0,
  "mutants_not_covered": 0,
  "mutants_equivalent": 0,
  "elapsed_time": 0.43,
  "mutator_statistics": {
    "arithmetic_base": 2,
    "conditionals_boundary": 1,
    "conditionals_negation": 1
  }
}
```

| Field | Value |
| --- | --- |
| `go_module` | The module path. |
| `files` | Each file with mutants. `file_name` is relative to the scanned directory. |
| `mutations` | Each mutant of the file: `type`, `status`, `line`, and `column`. A mutant with an [equivalent annotation](../../equivalent-mutants.md) also has `equivalent_reason`. |
| `test_efficacy` | The [test efficacy](#threshold-efficacy) as a percentage. |
| `mutations_coverage` | The [mutant coverage](#threshold-mutant-coverage) as a percentage. |
| `mutants_total` | The sum of the killed, lived, and not viable mutants. |
| `mutants_killed`, `mutants_lived`, `mutants_not_viable`, `mutants_not_covered`, `mutants_equivalent` | The number of mutants with each status. |
| `elapsed_time` | The time of the mutant tests, in seconds. |
| `mutator_statistics` | The number of mutants of each mutation type, with all statuses. A type without mutants is not in the object. |

### Remove self-assignments

:material-flag: `--remove-self-assignments` · :material-sign-direction: Default: `false`

Enables or disables the [REMOVE_SELF_ASSIGNMENTS](../../mutations/remove_self_assignments.md) mutation type.

```shell
gremlins unleash --remove-self-assignments
```

### Tags

:material-flag: `--tags`/`-t` · :material-sign-direction: Default: empty

A comma-separated list of build tags. Gremlins gives them to the `go` commands.
The mutants of a file that the tags exclude from the build are `SKIPPED`.

```shell
gremlins unleash --tags "tag1,tag2"
```

### Test CPU

:material-flag: `--test-cpu` · :material-sign-direction: Default: `0`

!!! tip
    To tune this flag, see [Workers](workers.md).

The number of CPUs for each test run. Gremlins gives it to the `-cpu` flag of `go test`, which sets `GOMAXPROCS` for the tests.
`0` does not set the flag.
In [integration mode](#integration-mode), Gremlins halves a value that is more than 1.
A negative value is not valid.

```shell
gremlins unleash --test-cpu=1
```

### Threshold efficacy

:material-flag: `--threshold-efficacy` · :material-sign-direction: Default: 0

The minimum test efficacy, as a percentage from 0 to 100.
When the test efficacy is less than this value, Gremlins exits with code 10.
`0` disables the check.

The test efficacy is `KILLED / (KILLED + LIVED) × 100`. It shows how well the tests detect the covered mutants.
The calculation uses only the `KILLED` and `LIVED` mutants. It does not count [`EQUIVALENT`](../../equivalent-mutants.md) mutants.
When no mutant is `KILLED`, the test efficacy is 0.

Gremlins does not assess the thresholds in a dry run, or when all the mutants are `SKIPPED` or `EQUIVALENT`.

```shell
gremlins unleash --threshold-efficacy 80
```

### Threshold mutant coverage

:material-flag: `--threshold-mcover` · :material-sign-direction: Default: 0

The minimum mutant coverage, as a percentage from 0 to 100.
When the mutant coverage is less than this value, Gremlins exits with code 11.
`0` disables the check.

The mutant coverage is `(KILLED + LIVED) / (KILLED + LIVED + NOT COVERED) × 100`. It shows how many mutants the tests cover.
In a dry run, the mutant coverage is `RUNNABLE / (RUNNABLE + NOT COVERED) × 100`.
The calculation uses only the statuses in the formula. It does not count `SKIPPED` and [`EQUIVALENT`](../../equivalent-mutants.md) mutants.

```shell
gremlins unleash --threshold-mcover 80
```

### Timeout coefficient

:material-flag: `--timeout-coefficient` · :material-sign-direction: Default: `0` (uses `5`)

!!! tip
    To tune this flag, see [Workers](workers.md).

The coefficient that multiplies the time of the coverage run to give the fallback timeout of each test run.
`0` uses the default coefficient of `5`. A negative value is not valid.

Gremlins uses a minimum base time of 1 second before it applies the coefficient, so a very fast coverage run still gives a usable timeout. For example:

- The coverage run takes 500 ms. The timeout is max(500 ms, 1 s) × 5 = 5 seconds.
- The coverage run takes 2 s. The timeout is 2 s × 5 = 10 seconds.

A package uses the fallback timeout until one of its test runs passes.
After that, the package uses an adaptive timeout. See [timeout safety margin](#timeout-safety-margin).
In [integration mode](#integration-mode), every test run uses the fallback timeout.

```shell
gremlins unleash --timeout-coefficient=10
```

### Timeout safety margin

:material-flag: `--timeout-safety-margin` · :material-sign-direction: Default: `0` (uses `2`)

The multiplier of the longest passing test run of a package that gives the adaptive timeout of that package.
`0` uses the default margin of `2`. A negative value is not valid.

When the tests of a mutant pass (`LIVED`), Gremlins records the time of that test run for the package.
It keeps the longest recorded time of each package.
A later test run in that package uses this time, multiplied by the margin, as its timeout.
Gremlins uses a minimum of 1 second before it applies the margin.
The adaptive timeout is never longer than the fallback timeout.
For example, with a fallback timeout of 300 seconds and the default margin:

- The longest passing run takes 100 ms. The timeout is max(100 ms, 1 s) × 2 = 2 seconds.
- The longest passing run takes 45 s. The timeout is 45 s × 2 = 90 seconds.

A `KILLED`, `NOT VIABLE`, `TIMED OUT`, or `MEMORY ERROR` test run records no time.
A failing test run can stop early, so its time does not show the time of all the tests.
A package with no passing test run keeps the fallback timeout.
Gremlins keeps the recorded times only during one run.

Outside [integration mode](#integration-mode), Gremlins runs `go test` with `-count=1`, so a cached test result does not give a time.

A shorter timeout stops a mutant that does not end sooner.
It does not change the time of a test run that completes.
On a busy machine, a test run can take longer than its recorded time.
If you see more `TIMED OUT` mutants, increase the margin.
A very large margin gives the fallback timeout to every test run.

```shell
gremlins unleash --timeout-safety-margin=3
```

### Memory budget

:material-flag: `--memory-budget` · :material-sign-direction: Default: empty (uses the smaller of `1G` and half of the available memory)

The memory that all the mutant test runs can use together.
Write it as a positive byte count with an optional binary unit: `K`, `M`, `G`, or `T`.
For example, `2G` is 2 GiB. Gremlins rejects other values before it starts the tests.

In a cgroup, the available memory is the smaller of the system value and the free memory of the cgroup.

Gremlins gives each mutant test run a memory limit of two times the peak memory of the largest process of the coverage run.
The limit is never larger than the budget.
Each test run reserves its limit from the budget before it starts.
When the budget has no free memory, a worker waits until another test run completes.

A watchdog measures the total memory of the test run every 50 ms.
This total includes the `go` command, the compiler, the linker, and the test binary.
On Linux and macOS, the watchdog measures the resident memory.
On Windows, it measures the peak committed memory of the job object.
When the total is more than the limit, Gremlins stops the test run and reports the mutant as `MEMORY ERROR`.
The memory can go above the limit between two measurements.
The budget does not apply to the coverage run.

```shell
gremlins unleash --memory-budget=2G
```

!!! note "Result consistency"
    The number of killed, lived, and timed-out mutants can change by a small number between runs. These are some causes:

    - **Race conditions**: Mutants can add or remove race conditions that change between runs.
    - **Timing-sensitive tests**: Tests of timeouts, concurrency, or I/O timing.
    - **System variations**: CPU scheduling, system load, and other operating system factors.

    Large variations, or results that become worse on each run, can show a problem.

### Workers

:material-flag: `--workers` · :material-sign-direction: Default: `0`

!!! tip
    To tune this flag, see [Workers](workers.md).

The number of mutants that Gremlins tests at the same time.
`0` uses the number of logical CPUs. A negative value is not valid.
In [integration mode](#integration-mode), Gremlins halves a value that is more than 1.
The [memory budget](#memory-budget) can also make a worker wait.

```shell
gremlins unleash --workers=4
```
