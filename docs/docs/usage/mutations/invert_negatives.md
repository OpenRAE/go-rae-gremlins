---
title: Invert negatives
---

# Invert negatives  <small>:material-sign-direction: default</small>

`INVERT_NEGATIVES` changes the unary `-` to the unary `+`. The mutant then has the value of the operand, not its negation.

## Mutation table

|  Original  |  Mutated  |
|:----------:|:---------:|
|     -      |     +     |

## Conditions

- The table applies only to the unary `-`, for example `-a`.
  [ARITHMETIC_BASE](arithmetic_base.md) mutates the binary `-`, for example `a - b`.

## Examples

=== "Original"

    ```go
    func makeNegative(i int) int {
      return -i
    }
    ```

=== "Mutated"

    ```go
    func makeNegative(i int) int {
      return +i
    }
    ```
