---
title: Extreme constant
---

# Extreme constant

_Extreme constant_ is a Descartes extreme mutation.
It removes the behavior of a full function or method and returns a constant for each result.
The mutation adds a `return` statement before the first statement of the body.
The original statements stay in the source, so their imports and variables stay in use.

The mutant is at the `func` keyword of the declaration.
With [extreme zero](extreme_zero.md), it gives the two constants of each result type of the Descartes method.

## Mutation table

[//]: # (@formatter:off)

| Result type | Value |
|:---|:---:|
| pointer, slice, map, channel, function, or interface type, `error`, `any` | `nil` |
| `bool` | `true` |
| predeclared numeric type | `1` |
| `string` | `"A"` |

[//]: # (@formatter:on)

Each result must have a type of the table, and one or more results must be `bool`, numeric, or `string`.
A function with a result of another type, for example a struct, a named type, or a type parameter, does not get this mutant.
A predeclared name counts only when the package, the imports of the file, and the function do not use the same name.
A function whose body is only a `return` of the same values does not get this mutant, because it is equivalent.

## Examples

=== "Original"

    ```go
    func IsAdult(age int) bool {
        return age >= 18
    }
    ```

=== "Mutated"

    ```go
    func IsAdult(age int) bool {
        return true
        return age >= 18
    }
    ```
