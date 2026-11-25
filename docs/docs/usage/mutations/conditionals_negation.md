---
title: Conditionals negation
---

# Conditionals negation <small>:material-sign-direction: default</small>

`CONDITIONALS_NEGATION` changes a comparison to its opposite. For example, `==` becomes `!=`.

## Mutation table

|  Original  | Mutated  |
|:----------:|:--------:|
|     ==     |    !=    |
|     !=     |    ==    |
|     \>     |    <=    |
|     <=     |    \>    |
|     <      |   \>=    |
|    \>=     |    <     |

## Examples

=== "Original"

    ```go
    if a == b {
      // Do something
    }
    ```

=== "Mutated"

    ```go
    if a != b {
      // Do something
    }
    ```
