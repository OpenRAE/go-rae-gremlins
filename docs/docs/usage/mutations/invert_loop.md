---
title: Invert loop
---

# Invert loop control

`INVERT_LOOPCTRL` changes a loop control statement. A `continue` becomes a `break`, and a `break` becomes a `continue`.

## Mutation table

[//]: # (@formatter:off)

| Original | Mutated  |
|:--------:|:--------:|
| continue |  break   |
|  break   | continue |

[//]: # (@formatter:on)

## Conditions

- Gremlins mutates a `break` only where a `continue` with the same label is valid.
- Without a label, the `break` must be in a `for` loop of the same function.
  For example, Gremlins mutates a `break` in a `switch` in a loop, but not a `break` in a `switch` outside a loop.
- With a label, the label must name a `for` loop.

## Examples

=== "Original"

    ```go
    for i := 0; i < 3; i++ {
        continue
    }
    ```

=== "Mutated"

    ```go
    for i := 0; i < 3; i++ {
        break
    }
    ```
