---
title: Invert bitwise
---

# Invert bitwise

`INVERT_BITWISE` changes a binary bitwise operator to a different one.

## Mutation table

| Original | Mutated |
|:--------:|:-------:|
|    &     |    \|   |
|    \|    |    &    |
|    ^     |    &    |
|    &^    |    &    |
|    >>    |   <<    |
|    <<    |   >>    |

## Conditions

- The table applies only to binary operators, for example `a & b`.
- Gremlins does not mutate the unary `&`, which takes an address, and the unary `^`, which is the bitwise complement.
  The changed code cannot build.
- Gremlins does not mutate the `|` of a type union in a type constraint, for example `~int | ~string`.
  That `|` is a type term and not a bitwise operation.

## Examples

=== "Original"

    ```go
    a := 1 & 2
    ```

=== "Mutated"

    ```go
    a := 1 | 2
    ```
