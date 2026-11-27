![Gremlins mascot](assets/images/gremlins_mascotte.png){ align=right }

# Welcome to Gremlins

Gremlins is a mutation testing tool for Go.
This documentation is for the OpenRAE fork.
Gremlins works best on small and medium Go modules.
A run on a large module can take hours.

![Example Gremlins report](assets/images/gremlins-term.webp)

- [Run your first mutation test](quick-start.md).
- [Install Gremlins](install.md), [set up continuous integration](usage/ci/github-action.md), or [run in a container](usage/ci/docker.md).
- Find the [command reference](usage/commands/index.md), [configuration keys](usage/configuration.md), and [mutation types](usage/mutations/index.md).

## What is Mutation Testing

Mutation testing checks whether tests detect changes to the code that they run.
See [coverage and mutation testing](usage/coverage.md) for how Gremlins selects the mutants to test.

## Features

Gremlins can limit mutants to changed lines in a [Git diff](usage/commands/unleash/index.md#diff).
It can also check thresholds for [efficacy](usage/commands/unleash/index.md#threshold-efficacy) and [mutant coverage](usage/commands/unleash/index.md#threshold-mutant-coverage) in continuous integration.

## Releases and contributions

During 0.x development, flags and configuration can change between minor releases.
For automated builds, use a specific [release](https://github.com/OpenRAE/go-rae-gremlins/releases).

See the [contributor guide](https://github.com/OpenRAE/go-rae-gremlins/blob/dev/docs/CONTRIBUTING.md) for work on the fork.
For security problems, follow the [private reporting procedure](https://github.com/OpenRAE/go-rae-gremlins/blob/dev/SECURITY.md).
