To get started and run a scenario, see [LilRÆ](https://github.com/openrae/lilrae).

<p align="center" style="text-align: center">
  <img alt="Gremlins mascotte" src="docs/docs/assets/images/gremlins_mascotte.png" />
  <h1 align="center" style="text-align: center">Gremlins</h1>
  <p align="center" style="text-align: center">A mutation testing tool for Go</p>
</p>

[![CI](https://img.shields.io/github/actions/workflow/status/OpenRAE/go-rae-gremlins/ci.yml?branch=dev&logo=github)](https://github.com/OpenRAE/go-rae-gremlins/actions/workflows/ci.yml)
[![Gremlins](https://img.shields.io/github/actions/workflow/status/OpenRAE/go-rae-gremlins/gremlins.yml?branch=dev&label=gremlins&logo=github)](https://github.com/OpenRAE/go-rae-gremlins/actions/workflows/gremlins.yml)

Gremlins is a mutation testing tool for Go. It has been made to work well on _smallish_ Go modules, for example
_microservices_. On these modules, it helps validate the tests and aids the TDD process. You can also use it as a CI
quality gate. As of now, Gremlins does not work very well on very big Go modules, mainly because a run can take hours to
complete.

![](docs/docs/assets/images/gremlins-term.webp)

## Gremlins version

Gremlins is still in its 0.x release, which, as per SemVer, does not guarantee backward compatibility. What this
means for Gremlins is that configuration flags and/or configuration files can change among minor releases. This may
be important if you use Gremlins in automated builds, for example in _continuous integration_ pipelines.

Also, while in 0.x releases, only the current minor release will be maintained. If we are in v0.2.0, no patch
releases will be provided for the v0.1.0 line.
Once Gremlins reaches its 1.x release, it will start to be backward compatible. Then we will decide how many
releases to maintain at the same time (ex. up to two minors before, as Go itself does).

## What is Mutation Testing

Code coverage is unreliable as a measure of test quality. It is too easy to have tests that exercise a piece of code but
do not test anything at all.

_Mutation testing_ works by mutating the code exercised by the tests and verifying if the mutation is caught by
the test suite. Imagine _gremlins_ going into your code and messing around: will your test suit catch their damage?

Here is a nice [intro to mutation testing](https://pedrorijo.com/blog/intro-mutation/).

## How to use Gremlins

See the [documentation](docs/docs/) to install, configure, and use Gremlins.

### Quick start

For all options, see the [documentation](docs/docs/).

Download the pre-built binary for your OS/ARCH from
the [release page](https://github.com/OpenRAE/go-rae-gremlins/releases/latest)
and put it somewhere in the `PATH`, or run
`go install github.com/OpenRAE/go-rae-gremlins/cmd/gremlins@latest`, then:

```shell
gremlins unleash
```

Gremlins will report each mutation as:

- `RUNNABLE`: In _dry-run_ mode, a mutation that can be tested.
- `NOT COVERED`: A mutation not covered by tests; it will not be tested.
- `KILLED`: The mutation has been caught by the test suite.
- `LIVED`: The mutation has not been caught by the test suite.
- `TIMED OUT`: The tests timed out while testing the mutation: the mutation actually made the tests fail, but not
  explicitly.
- `NOT VIABLE`: The mutation makes the build fail.
- `MEMORY ERROR`: The test run used more memory than its limit, so Gremlins stopped it.

## What inspired Gremlins

Mutation testing exists since the early days of computer science, so loads of papers and articles do exists. Gremlins is
inspired from those.

Among the existing mutation testing tools, Gremlins is inspired especially by [PITest](https://pitest.org/).

### Other Mutation Testing tools for Go

There is not much around, except from:

- [go-mutesting](https://github.com/avito-tech/go-mutesting#list-of-mutators)

## Contributing

See [contributing](docs/CONTRIBUTING.md).

## License

Gremlins is licensed under the [Apache 2.0 License](LICENSE).
The Gremlins documentation is licensed
under [CC BY-SA 4.0](http://creativecommons.org/licenses/by-sa/4.0/?ref=chooser-v1).
