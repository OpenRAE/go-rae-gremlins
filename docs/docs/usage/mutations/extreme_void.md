---
title: Extreme void
---

# Extreme void

_Extreme void_ is a Descartes extreme mutation.
It removes the behavior of a full function or method that has no results.
The mutation adds a `return` statement before the first statement of the body.
The original statements stay in the source, so their imports and variables stay in use.

The mutant is at the `func` keyword of the declaration.
A function literal does not get this mutant.
A function with an empty body or a body that is only `return` does not get this mutant, because it is equivalent.

## Examples

=== "Original"

    ```go
    func (c *Cache) Put(key string, value int) {
        c.items[key] = value
    }
    ```

=== "Mutated"

    ```go
    func (c *Cache) Put(key string, value int) {
        return
        c.items[key] = value
    }
    ```
