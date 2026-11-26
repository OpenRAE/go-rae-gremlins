---
title: Arithmetic base
---

# Arithmetic base <small>:material-sign-direction: default</small>

`ARITHMETIC_BASE` changes a basic arithmetic operator to a different one.

## Mutation table

|  Original  |  Mutated  |
|:----------:|:---------:|
|     +      |     -     |
|     -      |     +     |
|     *      |     /     |
|     /      |     *     |
|     %      |     *     |

## Conditions

- The table applies to binary operators, for example `a + b`, and to the unary `+`, for example `+a`.
- Gremlins does not mutate the unary `-`, for example `-a`. [INVERT_NEGATIVES](invert_negatives.md) makes that change.
- Gremlins does not mutate a `+` whose `-` form does not type-check. For example, Go has no `-` operator for strings.

## Examples

=== "Original"

    ```go
    a := 1 + 2
    ```

=== "Mutated"

    ```go
    a := 1 - 2
    ```
