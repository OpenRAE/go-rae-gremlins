# Quick start

In this tutorial, you make a small Go module and run Gremlins on it.
Gremlins finds a mutant that the tests do not detect. You then add a test that detects it.

## Prerequisites

- Go 1.25 or later.
- Gremlins. See [Install](install.md).
- A shell on Linux, macOS, or Windows. The commands use POSIX shell syntax.

The module has no dependencies. Gremlins runs `go test` in the module, so use a new, empty directory.

## Make the module

1. Make a directory for the module and go into it:

    ```shell
    mkdir quickstart
    cd quickstart
    ```

2. Make the `go.mod` file:

    ```shell
    go mod init example.com/quickstart
    ```

3. Make the file `discount.go` with this code:

    ```go
    package quickstart

    // Discount returns the price after a discount of 10 percent.
    // Only a price of 100 or more gets the discount.
    func Discount(price int) int {
    	if price >= 100 {
    		return price - price/10
    	}

    	return price
    }
    ```

4. Make the file `discount_test.go` with this code:

    ```go
    package quickstart

    import "testing"

    func TestDiscount(t *testing.T) {
    	tests := []struct {
    		price int
    		want  int
    	}{
    		{price: 50, want: 50},
    		{price: 200, want: 180},
    	}
    	for _, tt := range tests {
    		if got := Discount(tt.price); got != tt.want {
    			t.Errorf("Discount(%d) = %d, want %d", tt.price, got, tt.want)
    		}
    	}
    }
    ```

5. Run the tests:

    ```shell
    go test ./...
    ```

    The tests pass. Gremlins needs tests that pass before it starts.

## Find the mutants

1. Run Gremlins in dry-run mode:

    ```shell
    gremlins unleash --dry-run
    ```

    Gremlins runs the tests once to get the coverage. It does not run the tests of each mutant.
    The output shows four `RUNNABLE` mutants. The order of the lines can change:

    ```text
        RUNNABLE CONDITIONALS_BOUNDARY at discount.go:6:11
        RUNNABLE CONDITIONALS_NEGATION at discount.go:6:11
        RUNNABLE ARITHMETIC_BASE at discount.go:7:16
        RUNNABLE ARITHMETIC_BASE at discount.go:7:23
    ```

    Each line gives the status, the mutation type, the file, the line, and the column of the operator.
    For example, `CONDITIONALS_BOUNDARY` changes `>=` on line 6 to `>`.

## Run the mutation test

1. Run Gremlins:

    ```shell
    gremlins unleash
    ```

    Gremlins applies each mutant to a copy of the module and runs the tests.
    The output shows three `KILLED` mutants and one `LIVED` mutant:

    ```text
           LIVED CONDITIONALS_BOUNDARY at discount.go:6:11
    ```

    The summary shows `Test efficacy: 75.00%`.

A `KILLED` mutant makes a test fail. A `LIVED` mutant does not.
The `LIVED` mutant changes `price >= 100` to `price > 100`.
No test uses a price of exactly 100, so the tests cannot see the change.

## Detect the mutant

1. In `discount_test.go`, add a test case for the boundary value:

    ```go
    		{price: 50, want: 50},
    		{price: 100, want: 90},
    		{price: 200, want: 180},
    ```

2. Run Gremlins again:

    ```shell
    gremlins unleash
    ```

    All four mutants are `KILLED`. The summary shows `Test efficacy: 100.00%`.

You made a mutant live, found it, and added the test that kills it.

## Next steps

- To set a minimum efficacy for a continuous integration run, see [`--threshold-efficacy`](usage/commands/unleash/index.md#threshold-efficacy).
- For the meaning of each status, see [Mutant statuses](usage/commands/unleash/index.md#mutant-statuses).
- To learn which mutants Gremlins tests, see [Coverage](usage/coverage.md).
- For each mutation type, see [Mutations](usage/mutations/index.md).
