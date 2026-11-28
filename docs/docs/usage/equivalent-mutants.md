# Equivalent mutants

An equivalent mutant does not change the observable behavior of the code, so no test can kill it.
After a review, you can annotate one equivalent mutant in the source.
Gremlins does not run the tests of an annotated mutant, and the thresholds do not count it.
The other mutants on the same line and in the same file stay active.

## Write an annotation

Put a directive comment in the Go source file:

```go
//gremlins:equivalent TYPE[@COLUMN] REASON
```

- `TYPE` is the mutation type, as Gremlins prints it. For example, `CONDITIONALS_BOUNDARY`.
- `COLUMN` is the column of the operator, as Gremlins prints it. It is optional.
  The operator of an [extreme mutant](mutations/index.md#descartes-extreme-mutations) is the `func` keyword of its function.
- `REASON` is the result of your review. It must contain text.

Do not put a space between `//` and `gremlins`.
`gofmt` keeps this comment, and `go doc` does not show it.

### Find the type and the column

Run Gremlins and find the mutant in the output:

```
       LIVED CONDITIONALS_BOUNDARY at internal/limits/check.go:12:11
```

The type is `CONDITIONALS_BOUNDARY`. The line is 12, and the column is 11.
The JSON report also gives the `type`, `line`, and `column` of each mutant.
The column counts bytes from the start of the line. A tab is one column.

### Put the annotation on the line or above it

A directive after code selects a mutant on the same line.
In this example, the first `<` is at column 11, because the tab is column 1:

```go
func inRange(a, b, c, d int) bool {
	return a < b && c < d //gremlins:equivalent CONDITIONALS_BOUNDARY@11 a never equals b here
}
```

A directive alone on its line selects a mutant on the next line:

```go
//gremlins:equivalent CONDITIONALS_BOUNDARY the caller rejects equal values
return a < b
```

When the line has only one operator that can produce `TYPE`, you can omit `@COLUMN`.
When the line has more than one, you must give `@COLUMN`.
For example, `!`, `&&`, and `||` can all produce `INVERT_LOGICAL`.

## Errors

Gremlins checks each directive before it tests the mutants of the file.
These directives stop the run with exit code 1:

| Directive | Error |
| --- | --- |
| `//gremlins:` followed by a name other than `equivalent` or [`ignore`](ignore-mutants.md) | Unknown directive. |
| No type, or no reason | Missing mutation type, or missing reason. |
| A type that Gremlins does not know | Unknown mutation type. |
| A column that is not a positive integer | Column is not a positive integer. |
| No operator on the line can produce the type at the column | Selects no mutant. |
| More than one operator matches | Selects more than one mutant; add `@COLUMN`. |
| Two directives select the same mutant | Selects the same mutant as another annotation. |
| A `//line` directive moves the selected mutant | Not supported. |

The error gives the file, line, and column of the directive.
When you edit the code, a directive can become stale. Review it again, and update or remove it.

Gremlins does not create some mutants that cannot build, or a second mutant with the same change.
A directive for such a mutant selects no mutant. For example, `ARITHMETIC_BASE` does not mutate a unary `-`.
See the conditions of each [mutation type](mutations/index.md).

Gremlins checks a directive against all the mutation types, also the disabled types.
So a type that you disable for a run does not make its directives stale.
Gremlins does not check the files that it does not scan for mutants.
For example, it does not check test files and the files that `--exclude-files` excludes.

## Results

An annotated mutant has the `EQUIVALENT` status.
An annotated mutant outside the `--diff` changes, or that coverage cannot show, keeps the `SKIPPED` status.
To print only the equivalent mutants, use the `e` letter of [`--output-statuses`](commands/unleash/index.md#statuses-output).

The JSON report adds these fields:

- `equivalent_reason`: the reason of the annotation. Each annotated mutant has it, also a skipped one.
  Other mutants do not have it.
- `mutants_equivalent`: the number of `EQUIVALENT` mutants.

`mutants_total` does not change. It is the sum of the killed, lived, and not viable mutants.

## Thresholds

The calculations do not count `EQUIVALENT` mutants:

| Value | Calculation |
| --- | --- |
| Test efficacy | `KILLED / (KILLED + LIVED)` |
| Mutant coverage | `(KILLED + LIVED) / (KILLED + LIVED + NOT_COVERED)` |
| Mutant coverage in a dry run | `RUNNABLE / (RUNNABLE + NOT_COVERED)` |

For example, 1 killed, 1 lived, 2 not covered, and 3 equivalent mutants give 50% test efficacy and 50% mutant coverage.

When all the mutants are `SKIPPED` or `EQUIVALENT`, Gremlins does not assess the thresholds.
An equivalent mutant does not hide other mutants. For example, a not covered mutant still lowers the mutant coverage.

## Review the reasons

An annotation is a statement of a reviewer. Gremlins does not prove that the mutant is equivalent.
Review each annotation and its reason in code review.

The reason goes into the source and into the JSON report.
Do not write secrets or confidential information in a reason.
