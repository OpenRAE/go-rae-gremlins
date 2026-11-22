---
title: Invert logical
---

# Invert logical operators

_Invert logical operators_ will perform inversions on logical operators.
For a logical NOT, the mutation adds a second `!` in front of the operand.
The mutant then has the value of the operand.

## Mutation table

[//]: # (@formatter:off)

| Orig | Mutation |
|:----:|:--------:|
| &&   | \|\|     |
| \|\| | &&       |
| !    | !!       |


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
