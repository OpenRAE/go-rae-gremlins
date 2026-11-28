---
design: 1
issue: 229
title: The initial go-archcheck model
---

# Design 0001: The initial go-archcheck model

## Problem

CI must check the module code against an architecture model with go-archcheck.
The model must give each package of the module one layer and one component.

## Design

Each top-level package directory is a layer: `cmd`, each package directory under `internal`, and each tool under `tools`.
A layer directory also holds the packages below it, so `internal/engine` holds `workdir` and `workerpool`, and `internal/report` holds its `internal` package.

Each package is a component, except `internal/report/internal`, which belongs to the `report` component.
The uses of each layer and the dependencies of each component are the imports of the code.

Each external entry names one package with its reason.
The test modules are the external modules that test files import in layers with no external entry for them.

Two containers run the components: the `gremlins` command and the repository tools.
The `log` component runs in both containers, because the architecture tool imports it.

The model has no rules.
