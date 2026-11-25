---
title: Invert assignments
---

# Invert assignments

`INVERT_ASSIGNMENTS` changes an arithmetic assignment operator to a different one.
An arithmetic assignment, for example `a += b`, applies the operator to the two operands and assigns the result to the left operand.

## Mutation table

| Original | Mutated |
|:--------:|:-------:|
|    +=    |   -=    |
|    -=    |   +=    |
|    *=    |   /=    |
|    /=    |   *=    |
|    %=    |   *=    |

## Conditions

- Gremlins does not mutate a `+=` whose `-=` form does not type-check. For example, Go has no `-=` operator for strings.

## Examples

=== "Original"

    ```go
    a := 1
    a *= 2
    ```

=== "Mutated"

    ```go
    a := 1
    a /= 2
    ```
