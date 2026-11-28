This file provides guidance to Claude Code (claude.ai/code) when
working with code in this repository.

## Project Overview

Gremlins is a mutation testing tool for Go that validates test
effectiveness by introducing mutations to source code and verifying if
tests catch them. It works best on small to medium-sized Go modules
(microservices).

## Essential Commands

### Build & Test

```bash
# Build the binary
make build                    # Creates dist/bin/gremlins

# Run all tests with race detector
make test                     # go test -race ./...

# Run specific package tests
go test -race ./internal/engine/...

# Run single test
go test -race ./internal/engine/ -run TestEngineFindMutations

# Generate coverage report
make cover                    # Creates coverage.out
```

### Linting & Formatting

```bash
# Run linter (golangci-lint)
make lint

# Format Go files and imports with the pinned tool
make goimports

# Fix struct field alignment
make fieldalignment
```

### Documentation Checks

```bash
# Check explicit Markdown paths with the pinned doc-tools release
bash scripts/doc-tools.sh lint docs/CONTRIBUTING.md

# Check the Markdown that changed since the merge base with origin/dev
make docs-changed
```

Use the shared `documentation` skill of doc-tools v0.1.0. See
[Documentation](docs/CONTRIBUTING.md#documentation) for setup.

### Running Gremlins

```bash
# Basic mutation testing
gremlins unleash              # Or: gremlins run, gremlins r

# Dry-run mode: run the coverage tests and find mutants, but run no mutant tests
gremlins unleash --dry-run    # Or: -d

# Configuration file
# Uses .gremlins.yaml in project root
```

### Development Workflow

```bash
# Install required tools
make requirements

# Full cycle (lint + test + build). CI owns this gate; run targeted
# tests locally (docs/CONTRIBUTING.md#verify-the-change)
make all

# Create snapshot release
make snap                     # lint + test + snapshot

# Clean build artifacts
make clean
```

## Architecture

### Core Components

**Engine (`internal/engine/`)**

- Main orchestrator for mutation testing
- Walks AST to find mutation opportunities
- Coordinates mutant execution via worker pool
- Entry point: `Engine.Run(ctx)` traverses files, finds mutants,
  executes tests
- Mutation registry: one spec for each mutation type gives eligibility
  and a reversible AST edit (`spec.go`, `spec_expression.go`,
  `specs.go`). The registry validates the specs and the factory binds
  mutants (`registry.go`). One mutant implementation owns the file
  lifecycle (`mutant.go`). See
  [the mutation registry](docs/development/mutation-registry.md)

**Mutator (`internal/mutator/`)**

- Defines mutation types and their interface
- Types: `ArithmeticBase`, `ConditionalsBoundary`,
  `ConditionalsNegation`, `IncrementDecrement`, `InvertAssignments`,
  `InvertBitwise`, `InvertBitwiseAssignments`, `InvertLogical`,
  `InvertLoopCtrl`, `InvertNegatives`, `RemoveSelfAssignments`,
  `ExtremeVoid`, `ExtremeZero`, `ExtremeConstant`
- `Type.IsDescartes` gives the Descartes group of the extreme types
  (ADR 0012). `--descartes` runs only that group
- Status values: `NotCovered`, `Runnable`, `Skipped`, `Lived`,
  `Killed`, `NotViable`, `TimedOut`, `MemoryError`, `Equivalent`
- Each mutator implements: `Apply()` (mutate code), `Rollback()`
  (restore original)

**Coverage (`internal/coverage/`)**

- Determines which mutations are covered by tests
- Only covered mutations are tested (tests cannot catch uncovered
  mutations)

**Executor (`internal/engine/executor.go`)**

- Applies single mutation, runs tests, captures results
- Handles test timeouts and viability checks
- Works with temporary directories to isolate mutations

**Report (`internal/report/`)**

- Formats and outputs mutation testing results
- Supports console output and machine-readable formats
- Calculates efficacy and mutation coverage metrics

**Configuration (`internal/configuration/`)**

- Manages settings via flags, config file (.gremlins.yaml), and
  environment
- Uses Viper for configuration handling
- Controls which mutation types are enabled

### Key Flows

**Mutation Testing Flow:**

1. `cmd/unleash.go` → `run()` initializes components
2. Coverage analysis determines testable code. Without `GOCACHE`, all `go`
   commands of a run use one temporary build cache in the run work directory
3. `internal/typeinfo` loads the package types once and gives operand
   facts, for example a `+` without a valid `-` form. See
   [ADR 0011](docs/adr/0011-mutant-eligibility-type-information.md)
4. `Engine.Run()` walks file tree, parses Go AST
5. For each operator, the registry gives the eligible mutation types
   in registration order. A spec selects only operators whose mutants
   can build
6. Create mutators with status (`Runnable`, `NotCovered`, `Skipped`,
   `Equivalent`). A `//gremlins:equivalent` directive selects an
   `Equivalent` mutant (see `internal/engine/equivalent.go`). A
   `//gremlins:ignore` directive removes mutants of a line, a block,
   or a file before they are created (see `internal/engine/ignore.go`)
7. Worker pool processes runnable mutants:
   - Copy code to temp workdir
   - Apply mutation via `Mutator.Apply()`
   - Run tests
   - Set status: `Killed` (tests failed), `Lived` (tests passed),
     `TimedOut`, `MemoryError` (memory limit passed), `NotViable`
     (build failed)
   - Rollback mutation
8. Report results with efficacy/coverage metrics

**Adding New Mutation Type:**

1. Add new constant to `mutator.Type` in `internal/mutator/mutator.go`
2. Update `Types` slice and `String()` method
3. Register a spec for the type in `internal/engine/specs.go`. Use a
   `tokenSpec` table for a token replacement. Put the spec at its
   candidate position in the registration order
4. Set the default in `internal/configuration/mutantenabled.go`. The
   configuration key and the flag come from `mutator.Types`
5. Add the type to `docs/docs/schema/configuration.json`. Add its page
   to `docs/docs/usage/mutations/`, a row to
   `docs/docs/usage/mutations/index.md`, and a `nav` entry to
   `docs/mkdocs.yml`

Registration alone does not make a public type. A new type changes the
CLI, configuration, and report contracts.

### Important Patterns

- **Spec-based mutations**: Each spec edits the shared Go AST at an
  original operator position and restores the AST before `Apply`
  returns. An extreme spec edits a function body; its position is the
  `func` keyword. Worker writes use the physical file name, not `//line`
  positions
- **Workdir isolation**: Each mutation runs in a temporary copy of the
  module, so mutants do not interfere. The copy is not a sandbox: the
  tests of the target module run with the permissions of the user
- **Worker pool**: Concurrent mutation execution with configurable
  parallelism
- **Configuration cascade**: Flags override config file, which
  overrides defaults
- **Context cancellation**: `signalContext()` in `cmd/gremlins/main.go`
  cancels the run context on SIGINT/SIGTERM. Coverage, scheduling, and
  mutant execution stop on it, and a cancelled run returns an error

## Testing Conventions

Follow the [Go test standard](docs/development/test-standard.md) for all Go packages.
The [test assessment](docs/development/test-assessment.md) records the gaps found against that standard, their repairs, and the evidence limits.
Use the [contributor guide](docs/CONTRIBUTING.md#verify-the-change) for local and hosted verification.

## Module Structure

```text
cmd/                    - CLI commands (gremlins.go, unleash.go, cobra setup)
  gremlins/            - Binary entry point (main.go)
  internal/flags/      - Shared flag binding
internal/
  configuration/        - Config handling (Viper-based)
  coverage/            - Test coverage analysis
  diff/                - Git diff integration for incremental testing
  engine/              - Core mutation engine and AST traversal
  exclusion/           - File/pattern exclusion rules
  execution/           - Test execution primitives
  gomodule/            - Go module detection and info
  log/                 - Logging utilities
  mutator/             - Mutation definitions and interface
  process/             - Command runs that stop the process group
  report/              - Result formatting and output
  typeinfo/            - Operand type facts for mutant eligibility
tools/
  architecture/        - Architecture model and design checker (make architecture)
  lintcontext/         - Declaration identities for the lint baseline (make lint)
```

## Quality Standards

- Use the Go version and toolchain in `go.mod`.
- Follow [the Go coding standard](docs/development/coding-standards.md).
  `make lint` checks strict rules, formatting, and the shrink-only baseline.
  Provision the mise-pinned linter and Go, and run `go mod download` before offline analysis.
  Set `LINT_BASE` to the fetched comparison commit when needed.
  Fix debt, then use `make lint-shrink`; never add baseline allowances.
- CI sends test coverage to SonarCloud
- All commits must pass: lint, tests, build
- Mutation testing runs on itself (`.gremlins.yaml` configured)

## Architecture design check

Before coding, write your design to `.architecture/designs/<issue>.json` and run `make architecture-design`.
Each change uses its own design file, so open pull requests do not conflict on the design input.
Record the model digest, affected rules, and result in the issue plan.

Resolve conflicts before implementation. Repeat after model or design changes.
See [the architecture model and design check](docs/development/architecture.md).
CI runs the same architecture check as `make architecture`. It rejects baseline growth and missing design coverage.
