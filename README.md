<p align="center">
  <img alt="Gremlins mascot" src="docs/docs/assets/images/gremlins_mascotte.png" />
</p>

# Gremlins

[![CI](https://img.shields.io/github/actions/workflow/status/OpenRAE/go-rae-gremlins/ci.yml?branch=dev&logo=github)](https://github.com/OpenRAE/go-rae-gremlins/actions/workflows/ci.yml)
[![Gremlins](https://img.shields.io/github/actions/workflow/status/OpenRAE/go-rae-gremlins/gremlins.yml?branch=dev&label=gremlins&logo=github)](https://github.com/OpenRAE/go-rae-gremlins/actions/workflows/gremlins.yml)

Gremlins is a mutation testing tool for Go.
This repository is the OpenRAE fork.
Gremlins works best on small and medium Go modules.
A run on a large module can take hours.

![Example Gremlins report](docs/docs/assets/images/gremlins-term.webp)

## Gremlins version

During 0.x development, flags and configuration can change between minor releases.
For automated builds, use a specific [release](https://github.com/OpenRAE/go-rae-gremlins/releases).
See the [fork release decision](docs/adr/0002-binary-name-and-versions.md) and the [generated change history](CHANGELOG.md).

## What is Mutation Testing

Mutation testing checks whether tests detect changes to the code that they run.
See [coverage and mutation testing](docs/docs/usage/coverage.md) for how Gremlins selects the mutants to test.

## How to use Gremlins

- [Install Gremlins](docs/docs/install.md).
- [Set up continuous integration](docs/docs/usage/ci/github-action.md) or [run in a container](docs/docs/usage/ci/docker.md).
- Find the [commands](docs/docs/usage/commands/index.md), [configuration keys](docs/docs/usage/configuration.md), and [mutation types](docs/docs/usage/mutations/index.md).
- See [LilRÆ](https://github.com/OpenRAE/lilrae) to run a scenario.

### Quick start

Follow the [first mutation test tutorial](docs/docs/quick-start.md).
See the [mutant statuses](docs/docs/usage/commands/unleash/index.md#mutant-statuses) to read the result.

## What inspired Gremlins

[PITest](https://pitest.org/) was an inspiration for Gremlins.
For an introduction to the method, see [Introduction to mutation testing](https://pedrorijo.com/blog/intro-mutation/).

### Other Mutation Testing tools for Go

See [go-mutesting](https://github.com/avito-tech/go-mutesting#list-of-mutators).

## Contributing

- Follow the [contributor guide](docs/CONTRIBUTING.md) and the adopted [Code of Conduct](CODE_OF_CONDUCT.md).
- For security problems, follow the [private reporting procedure](SECURITY.md).
- For repository documentation and its source records, see the [documentation inventory](docs/development/diataxis-adoption.md#complete-scope-and-ownership).
- For agent instructions, see [AGENTS.md](AGENTS.md).

## License

The source code has the [Apache 2.0 License](LICENSE).
The documentation has the [CC BY-SA 4.0 License](https://creativecommons.org/licenses/by-sa/4.0/).
