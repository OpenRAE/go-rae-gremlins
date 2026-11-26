---
title: Invert logical
---

# Invert logical operators

`INVERT_LOGICAL` changes a logical operator.
For a logical NOT, the mutation adds a second `!` in front of the operand.
The mutant then has the value of the operand. Go evaluates the operand one time.

## Mutation table

[//]: # (@formatter:off)

| Original | Mutated |
|:--------:|:-------:|
| &&       | \|\|    |
| \|\|     | &&      |
| !        | !!      |

[//]: # (@formatter:on)

## Examples

=== "Original"

    ```go
    a := true && false
    ```

=== "Mutated"

    ```go
    a := true || false
    ```

The next example shows the mutation of a logical NOT.

=== "Original"

    ```go
    b := !a
    ```

=== "Mutated"

    ```go
    b := !!a
    ```
