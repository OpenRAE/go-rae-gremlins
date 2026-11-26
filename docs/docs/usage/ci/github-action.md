---
title: GitHub Actions
---

# GitHub Actions

go-rae-gremlins has no GitHub Action of its own.
In a workflow, install Gremlins with `go install` and then run `gremlins unleash`.

## Example usage

```yaml
name: gremlins

on:
  pull_request:
  push:

permissions:
  contents: read

jobs:
  gremlins:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: go.mod
      - name: Install Gremlins
        run: go install github.com/OpenRAE/go-rae-gremlins/cmd/gremlins@v<version>
      - name: Run Gremlins
        run: gremlins unleash
```

Replace `<version>` with a version from the [release page](https://github.com/OpenRAE/go-rae-gremlins/releases).
Gremlins needs Go 1.25 or later. If your `go.mod` selects an earlier Go version, set `go-version` to 1.25 or later.

To set flags, add them to the `gremlins unleash` command. For example, `gremlins unleash --tags="tag1,tag2"`.
A threshold that is not met makes the step fail. See [exit codes](../commands/index.md#exit-codes).

## Test only the changes of a pull request

The [`--diff`](../commands/unleash/index.md#diff) flag needs the Git history of the base branch.
GitHub sets `GITHUB_BASE_REF` only for `pull_request` events, so use this step only in a workflow for those events.

1. In the `actions/checkout` step, set `fetch-depth: 0`.
2. Run Gremlins with the base branch of the pull request:

    ```yaml
          - name: Run Gremlins
            run: gremlins unleash --diff "origin/$GITHUB_BASE_REF"
    ```
