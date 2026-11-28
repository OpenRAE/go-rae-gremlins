---
title: Extreme zero
---

# Extreme zero

_Extreme zero_ is a Descartes extreme mutation.
It removes the behavior of a full function or method and returns the zero value of each result.
The mutation adds a `return` statement before the first statement of the body.
The original statements stay in the source, so their imports and variables stay in use.

The mutant is at the `func` keyword of the declaration.

## Mutation table

[//]: # (@formatter:off)

| Result type | Value |
|:---|:---:|
| pointer, slice, map, channel, function, or interface type, `error`, `any` | `nil` |
| `bool` | `false` |
| predeclared numeric type | `0` |
| `string` | `""` |

[//]: # (@formatter:on)

Each result must have a type of the table.
A function with a result of another type, for example a struct, a named type, or a type parameter, does not get this mutant.
A predeclared name counts only when the package, the imports of the file, and the function do not use the same name.
A function whose body is only a `return` of the same values does not get this mutant, because it is equivalent.

## Examples

=== "Original"

    ```go
    func Parse(s string) (int, error) {
        return strconv.Atoi(s)
    }
    ```

=== "Mutated"

    ```go
    func Parse(s string) (int, error) {
        return 0, nil
        return strconv.Atoi(s)
    }
    ```
