---
title: Increment decrement
---

# Increment decrement <small>:material-sign-direction: default</small>

`INCREMENT_DECREMENT` changes an increment statement to a decrement statement, and a decrement statement to an increment statement.

## Mutation table

|  Original  |  Mutated  |
|:----------:|:---------:|
|     ++     |    --     |
|     --     |    ++     |

## Examples

=== "Original"

    ```go
    for i := 0; i < 10; i++ {
      // Do something
    }
    ```

=== "Mutated"

    ```go
    for i := 0; i < 10; i-- {
      // Do something
    }
    ```
