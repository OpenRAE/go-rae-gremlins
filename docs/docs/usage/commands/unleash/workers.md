# Workers

This guide helps you tune the parallel test runs of Gremlins.
The defaults are correct for most modules. Change them when too many mutants are `TIMED OUT`.
For the exact behavior of each flag, see the [unleash reference](index.md#flags).

## Find the correct settings

A machine with too much load makes test runs slower, so more mutants get the `TIMED OUT` status.
Tune the settings until the number of `TIMED OUT` mutants is low and the same in each run.
To get a reference result, run Gremlins with one worker:

```shell
gremlins unleash --workers=1
```

## Workers and test CPU

The number of workers (`--workers`) is the number of mutants that Gremlins tests at the same time.
By default, Gremlins uses the number of logical CPUs. This value is correct most of the time.
If too many mutants are `TIMED OUT`, decrease this value.

The test CPU value (`--test-cpu`) sets `GOMAXPROCS` for the tests of each test run, through the `-cpu` flag of `go test`.
By default, Gremlins does not set it. It is not a limit on the CPU use of the process.
If you decrease the number of workers, you can increase the test CPU value.

Each worker runs one test run, and each test run can use up to the test CPU value of CPUs.
So make the product of the workers and the test CPU value not more than the number of CPUs of the machine.

## Timeout coefficient

The timeout coefficient (`--timeout-coefficient`) multiplies the time of the coverage run.
The result is the fallback timeout of each test run.
A package uses the fallback timeout until one of its test runs passes. The default coefficient is 5.

If your tests take a long time, a lower coefficient can make a mutant that does not end stop sooner.

## Timeout safety margin

After a test run of a package passes, the package uses an adaptive timeout.
This timeout is the longest passing test run of the package multiplied by the timeout safety margin (`--timeout-safety-margin`).
The default margin is 2. A larger coefficient does not increase the adaptive timeout.
If too many mutants are `TIMED OUT`, increase the margin or decrease the number of workers.

## Integration mode

Integration mode runs the tests of all the packages for each mutant, so it uses more CPU.
For this reason, Gremlins halves the workers and the test CPU values that are more than 1.
For example, if you set 4 workers, Gremlins runs 2.
