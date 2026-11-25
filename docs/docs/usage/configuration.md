# Configuration

You can configure Gremlins with command flags, environment variables, and a configuration file.
When two sources set the same key, Gremlins uses the value of the source with the higher precedence:

1. Command flags
2. Environment variables
3. The configuration file
4. The defaults

## Command flags

Flags have the highest precedence. Each flag has a configuration key.
For the flags of a command, see its [command reference](commands/index.md).

## Configuration files

The configuration file is a YAML file with the name `.gremlins.yaml`.
The file is optional. If Gremlins finds a file that it cannot read or parse, it stops with an error.

### Location

Gremlins searches these locations in this order, and reads only the first file that it finds:

1. `./.gremlins.yaml` (the current directory)
2. `.gremlins.yaml` in the root of the Go module of the current directory
3. `/etc/gremlins/.gremlins.yaml` (not on Windows)
4. `$XDG_CONFIG_HOME/gremlins/.gremlins.yaml`
5. `$HOME/.gremlins.yaml`

So a file in the project has precedence over a global file or a user file.
Gremlins does not merge the values of two files.

[//]: # (@formatter:off)
!!! hint
    When `XDG_CONFIG_HOME` is not set, or is not an absolute path, Gremlins uses `~/.config`.

[//]: # (@formatter:on)

### Override

The `--config` flag sets the configuration file. Gremlins then does not search the locations above.

```shell
gremlins unleash --config=myConfig.yaml
```

A value with a file extension is the file. If the file does not exist, Gremlins stops with an error.
A value without an extension is a directory. Gremlins searches only that directory for `.gremlins.yaml`.
If the directory has no such file, Gremlins uses no configuration file.

### Reference

This file sets all the keys to their defaults:

```yaml
silent: false
unleash:
  integration: false
  dry-run: false
  tags: ""
  coverpkg: ""
  output: ""
  diff: ""
  output-statuses: ""
  output-diff-statuses: ""
  workers: 0 #(1)
  test-cpu: 0 #(2)
  timeout-coefficient: 0 #(3)
  timeout-safety-margin: 0 #(4)
  memory-budget: ""
  threshold: #(5)
    efficacy: 0
    mutant-coverage: 0
  exclude-files: [] #(6)

mutants:
  arithmetic-base:
    enabled: true
  conditionals-boundary:
    enabled: true
  conditionals-negation:
    enabled: true
  increment-decrement:
    enabled: true
  invert-assignments:
    enabled: false
  invert-bitwise:
    enabled: false
  invert-bwassign:
    enabled: false
  invert-negatives:
    enabled: true
  invert-logical:
    enabled: false
  invert-loopctrl:
    enabled: false
  remove-self-assignments:
    enabled: false

```

1. `0` uses the number of logical CPUs. A negative value is not valid.
2. `0` does not set the CPUs of each test run. A negative value is not valid.
3. `0` uses the default coefficient of `5`. A negative value is not valid.
4. `0` uses the default safety margin of `2`. A negative value is not valid.
5. `0` disables a threshold. A threshold must be from `0` to `100`.
6. An empty list excludes only the test files.

For the meaning of each key, see the flag with the same name in the [unleash reference](commands/unleash/index.md#flags).
The `threshold` keys are the `--threshold-efficacy` and `--threshold-mcover` flags.
Each `mutants` key is the flag of its [mutation type](mutations/index.md).

[//]: # (@formatter:off)
!!! tip
    You can validate the configuration file with the [JSON Schema](../schema/configuration.json), for example in your
    editor. Gremlins does not read the schema. It checks the values itself when the command starts.

[//]: # (@formatter:on)

## Environment variables

An environment variable sets one configuration key.
The name is `GREMLINS_` and the key in upper case, with each `.` and `-` changed to `_`.

For example, this key:

```yaml
mutants:
  arithmetic-base:
    enabled: true
```

has this environment variable:

```shell
export GREMLINS_MUTANTS_ARITHMETIC_BASE_ENABLED=true
```

Gremlins converts each value to the type of its key: boolean, integer, number, string, or list of strings.
For a list, separate the items with spaces. For example, `GREMLINS_UNLEASH_EXCLUDE_FILES="_mock.go$ ^gen/"`.
If a value does not convert, `unleash` stops with an error that names the configuration key.
This check also applies to values from flags and from the configuration file.
