---
title: Mutations
---

# About mutations

A mutation is a small change to one operator of the source code. For example, `>=` becomes `>`.
A mutant is one mutation at one position in the code.
Each mutation belongs to a mutation type. The type name is the name that the report shows.

Gremlins does not create a mutant when it can find before the test that the changed code cannot build.
It also creates only one mutant for each change. Each type page gives its conditions.

Some mutants can still fail to build. For example, a change to a constant expression can divide by zero. These mutants get the `NOT VIABLE` status.
Gremlins does not mutate test files.

You can enable or disable each mutation type with its flag or its configuration key.
The configuration key is `mutants.<flag name>.enabled`. For example, `mutants.arithmetic-base.enabled`.

| Mutation type | Flag | Default |
|---|---|:---:|
| [ARITHMETIC_BASE](arithmetic_base.md) | `--arithmetic-base` | Enabled |
| [CONDITIONALS_BOUNDARY](conditionals_boundary.md) | `--conditionals-boundary` | Enabled |
| [CONDITIONALS_NEGATION](conditionals_negation.md) | `--conditionals-negation` | Enabled |
| [INCREMENT_DECREMENT](increment_decrement.md) | `--increment-decrement` | Enabled |
| [INVERT_NEGATIVES](invert_negatives.md) | `--invert-negatives` | Enabled |
| [INVERT_LOGICAL](invert_logical.md) | `--invert-logical` | Disabled |
| [INVERT_LOOPCTRL](invert_loop.md) | `--invert-loopctrl` | Disabled |
| [INVERT_ASSIGNMENTS](invert_assignments.md) | `--invert-assignments` | Disabled |
| [INVERT_BITWISE](invert_bitwise.md) | `--invert-bitwise` | Disabled |
| [INVERT_BWASSIGN](invert_bitwise_assignments.md) | `--invert-bwassign` | Disabled |
| [REMOVE_SELF_ASSIGNMENTS](remove_self_assignments.md) | `--remove-self-assignments` | Disabled |

One operator can have mutants of more than one type. For example, `<` has a `CONDITIONALS_BOUNDARY` and a `CONDITIONALS_NEGATION` mutant.
