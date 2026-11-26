---
title: Invert bitwise assignments
---

# Invert bitwise assignments

`INVERT_BWASSIGN` changes a bitwise assignment operator to a different one.
A bitwise assignment, for example `a &= b`, applies the operator to the two operands and assigns the result to the left operand.

## Mutation table

| Original | Mutated |
|:--------:|:-------:|
|    &=    |   \|=   |
|    \|=   |    &=   |
|    ^=    |   &=    |
|   &^=    |   &=    |
|   >>=    |   <<=   |
|   <<=    |   >>=   |

## Examples

=== "Original"

    ```go
    a := 1
    a &= 1
    ```

=== "Mutated"

    ```go
    a := 1
    a |= 1
    ```
