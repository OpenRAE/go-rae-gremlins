---
title: Remove self-assignments
---

# Remove self-assignments

`REMOVE_SELF_ASSIGNMENTS` changes an assignment operator, for example `+=`, to a plain assignment, `=`.
The left operand then gets the value of the right operand, not the result of the operation.

## Mutation table

|  Original  | Mutated |
|:----------:|:-------:|
|     +=     |    =    |
|     -=     |    =    |
|     *=     |    =    |
|     /=     |    =    |
|     %=     |    =    |
|     &=     |    =    |
|  &#124;=   |    =    |
|     ^=     |    =    |
|    <<=     |    =    |
|    \>>=    |    =    |
|    &^=     |    =    |

## Conditions

- Gremlins does not mutate a `<<=` or `>>=` whose shift count has a type that the left operand cannot hold.
  For example, with `var a int64` and `var n uint`, `a = n` cannot build.

## Examples

=== "Original"

    ```go
    a := 1
    a += 2
    ```

=== "Mutated"

    ```go
    a := 1
    a = 2
    ```
