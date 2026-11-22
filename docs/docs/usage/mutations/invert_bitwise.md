---
title: Invert bitwise
---

# Invert bitwise

_Invert bitwise_ will perform inversions on basic bit operations.

Gremlins does not mutate the `|` of a type union in a type constraint, for example `~int | ~string`.
That `|` is a type term and not a bitwise operation.

## Mutation table

| Original | Mutated |
|:--------:|:-------:|
|    &     |    \|   |
|    \|    |    &    |
|    ^     |    &    |
|    &^    |    &    |
|    >>    |   <<    |
|    <<    |   >>    |

## Examples

=== "Original"

    ```go
    a := 1 & 2
    ```

=== "Mutated"

    ```go
    a := 1 | 2
    ```
