# Workers

Gremlins works in parallel mode. It uses some sensible defaults, but it may be necessary to tweak them in your specific
use case. Finding the correct settings mandates a little trial and error, and we are still learning how to get the most
of it.

The first setting you should be aware of is the number of _workers_ (`--workers`). By default, Gremlins uses the number
of available CPU cores. This value is correct most of the time. If too many mutants go into `TIMED OUT`, decrease
this value.

If you decrease this value, you may also try to increase the number of CPU cores available to each test
run (`--test-cpu`). This is equivalent to the `-cpu` flag of the Go test tool, but for each mutation test. Gremlins
does not enforce this by default.

As a general rule, make the sum of _workers_ and _test CPU_ equal to the number of cores of the machine.

The symptom of a run excessively stressed is the number of mutants going into `TIMED OUT`. You should tweak the two
values above until your runs stabilize on a low and constant number of `TIMED OUT` mutants. To understand what could be
your correct value, you can run Gremlins with a single worker and see the results.

## Timeout coefficient

Another setting you may want to tweak is the _timeout coefficient_. This is the multiplier of the coverage run time
that gives the fallback timeout of each test run. A package uses the fallback timeout until one of its test runs
passes. The default coefficient is 5.

If your test suite takes a lot of time to run, you may want to _decrease_ the coefficient.

## Timeout safety margin

After a test run of a package passes, the package uses an adaptive timeout. This timeout is its longest passing test
run multiplied by the _timeout safety margin_ (`--timeout-safety-margin`). The default margin is 2. An increase of the coefficient does
not increase the adaptive timeout. If you see too many mutants in `TIMED OUT`, increase the margin, or decrease the
number of workers.

## Integration mode

_Integration mode_ is quite heavy on the CPU in parallel mode. For this reason, Gremlins halves the values for workers
and _test CPU_ if it is running in _integration mode_. So, if you set, for example, 4 workers, it will run effectively
with 2. And same goes for _test CPU_.
