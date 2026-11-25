---
title: The gremlins command
---

# Gremlins

The `gremlins` command has commands and flags:

```shell
gremlins <command> [flags]
```

| Command | Purpose |
| --- | --- |
| [`unleash`](unleash/index.md) | Run a mutation test. The aliases are `run` and `r`. |
| `help` | Show the help of a command: `gremlins help <command>`. |
| `completion` | Write a shell completion script: `gremlins completion <shell>`. |

Without a command, `gremlins` shows a usage summary.
The `--help`/`-h` flag shows the help of a command.

## Global flags

The global flags apply to all commands.

### Config

:material-flag:`--config` · :material-sign-direction: Default: empty

Sets the configuration file. A value with a file extension is the file.
A value without an extension is a directory that Gremlins searches for `.gremlins.yaml`.
Gremlins then does not search the default locations. See [Configuration](../configuration.md#override).

```shell
gremlins <command> --config=config.yml
```

### Silent

:material-flag:`--silent`/`-s` · :material-sign-direction: Default: false

Stops the information output of Gremlins, which includes the mutant lines and the summary.
Gremlins still writes error messages to standard error, and the JSON output file.
Output of the `go` command, for example from `go mod download`, can still show.

Use silent mode in continuous integration with a [threshold](unleash/index.md#threshold-efficacy) and an [output file](unleash/index.md#output).
Then read the exit code and the output file.

```shell
gremlins <command> --silent
```

!!! warning
    In silent mode, a run that has no errors shows nothing until it ends.

### Version

:material-flag:`--version`/`-v`

Shows the version, the operating system, and the architecture of the binary.

```shell
gremlins --version
```

## Exit codes

| Code | Condition |
| --- | --- |
| `0` | The command completed, and all the thresholds are met. |
| `1` | An error occurred. For example, a configuration value is not valid, the coverage run failed, the run was cancelled, or Gremlins cannot write the output file. |
| `10` | The test efficacy is less than [`--threshold-efficacy`](unleash/index.md#threshold-efficacy). |
| `11` | The mutant coverage is less than [`--threshold-mcover`](unleash/index.md#threshold-mutant-coverage). |

When both thresholds are not met, the exit code is `10`.
