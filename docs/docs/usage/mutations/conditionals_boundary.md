---
title: Conditionals boundary
---

# Conditionals boundary <small>:material-sign-direction: default</small>

`CONDITIONALS_BOUNDARY` changes the boundary of a comparison. An exclusive limit becomes inclusive, and an inclusive limit becomes exclusive.

## Mutation table

|  Original  |  Mutated  |
|:----------:|:---------:|
|     \>     |    \>=    |
|    \>=     |    \>     |
|     <      |    <=     |
|     <=     |     <     |

## Examples

=== "Original"

    ```go
    if a > b {
      // Do something
    }
    ```

=== "Mutated"

    ```go
    if a >= b {
      // Do something
    }
    ```
