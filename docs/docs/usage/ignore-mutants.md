# Ignore mutants

You can tell Gremlins to ignore the mutants of a line, a block of code, or a file.
Gremlins does not create an ignored mutant.
It does not test the mutant, show it in the output, or count it in the thresholds.

To keep a reviewed mutant in the report with a reason, use an [equivalent annotation](equivalent-mutants.md) instead.

## Write a directive

Put a directive comment in the Go source file:

```go
//gremlins:ignore [TYPE[,TYPE...]]
```

- Without a type, Gremlins ignores all the mutation types.
- `TYPE` is a mutation type, as Gremlins prints it. For example, `ARITHMETIC_BASE`.
- Use a comma between types. For example, `ARITHMETIC_BASE,INVERT_BITWISE`.

Do not put a space between `//` and `gremlins`.
`gofmt` keeps this comment, and `go doc` does not show it.

## Set the scope

### A line

A directive after code ignores the mutants on that line:

```go
a := b + c //gremlins:ignore
d := e + f //gremlins:ignore ARITHMETIC_BASE,INVERT_BITWISE
```

### A block of code

A directive alone on its line ignores the code that starts on the next line.
Before a function, it ignores the full function:

```go
//gremlins:ignore
func myFunc() {
	a := b + c
}
```

Before a statement, it ignores the full statement. For an `if` statement, that includes all the branches:

```go
//gremlins:ignore CONDITIONALS_BOUNDARY
if a < b {
	return a
}
```

The directive can be in a documentation comment.
`gofmt` moves it to the end of the comment. The scope does not change.

### A file

A directive alone on its line before the `package` clause ignores all the mutants of the file:

```go
//gremlins:ignore
package apackage
```

A directive applies only to its file. It does not apply to the other files of the package.

## Errors

These directives stop the run with exit code 1:

| Directive | Error |
| --- | --- |
| `//gremlins:` followed by a name other than `equivalent` or `ignore` | Unknown directive. |
| A type that Gremlins does not know, for example `arithmetic-base` | Unknown mutation type. |
| An empty type, for example `ARITHMETIC_BASE,` | Empty mutation type. |
| A directive alone on its line, with a blank line or no code after it | No code starts on the line after the comment. |

The error gives the file, line, and column of the directive.

A directive that ignores no mutant is not an error.
For example, a directive for a type that you disable for the run is valid.

## Results

Gremlins removes ignored mutants before it tests the module.
`mutants_total`, the other counts, test efficacy, and mutant coverage do not count them.
The `--output-statuses` filter does not show them.

When the directives ignore all the mutants, Gremlins has no results.
It then does not write the JSON output file, and an old output file stays.

## Review the directives

A directive is a decision of a reviewer. Gremlins does not check that the ignored mutants are unimportant.
Review each directive in code review.
