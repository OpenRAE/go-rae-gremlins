---
title: GitHub Actions
---

# GitHub Actions

go-rae-gremlins has no GitHub Action of its own. Install the fork with `go install` in a workflow step, then run
`gremlins unleash`.

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
