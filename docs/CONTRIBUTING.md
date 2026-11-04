# Contributing to Gremlins

First, thanks for you wanting to contribute, the Gremlins project welcomes contributors!

## What can I contribute?

### Report bugs

Bug reports are always welcome. Before submitting one, please verify that a similar bug hasn't been already reported. If
it already exists, consider to comment there instead of opening a new one.

#### To submit a good bug report

Use the appropriate template to submit bugs.

- Use a clear and descriptive title.
- Describe each step to reproduce using as much detail as you can.
- Describe the behaviour you observed after following the steps.
- Explain why it is different from the behaviour you would expect.

### Suggest enhancements

Before making a feature request/enhancement request, please verify that there isn't already a request for the same or
very similar feature. If a similar enhancement request already exists, you can expand on it via comments.
There is a specific issue type for feature requests.

### Send pull requests

Pull request are welcome, but it's not guaranteed they will be accepted. We are quite strict on code quality, style and
code metrics, bear with us if we ask you to make changes before accepting your PR.

## Becoming a contributor

This fork's optional maintainer tooling and repository checks are documented in
[Maintainer setup](https://github.com/OpenRAE/go-rae-gremlins/blob/main/docs/MAINTAINER_SETUP.md).
Ground Control is not required to contribute.

Gremlins is fully developed on GitHub and the best way to contribute is by forking the repository and, once you complete
your work, opening a _pull request_.

### Before contributing

All contributions are welcome, but, before submitting any significant change, it is better to coordinate with the
Gremlins' team before starting the work. It is a good idea to start at
the [issue tracker](https://github.com/go-gremlins/gremlins/issues) and file a new issue or claim an existing one.

### Open an issue

Apart from trivial changes, every contribution to the Gremlins should be linked to an issue. Feel free to propose a
change and expose your plans, so that everyone can contribute in its validation, and the chances of your _pull request_
being accepted will increase.

### Submit a contribution

Gremlins is released with semantic versioning and
follows [GitHub flow](https://docs.github.com/en/get-started/quickstart/github-flow), with the only difference that we
open release branches when there is a _fix version_ to release.

When you open a _pull request_, a series of automatic checks kicks off. You can verify if your change makes those checks
fail and adjust the code accordingly.

At this point, a member of the Gremlins team will review your code, possibly will
discuss with you to understand it better, maybe ask for some changes and so on. Please expect that this process will be
more thorough if you are a first time contributor: we need to know each other.

Feel free to make more than one commit in your _pull request_, but bear in mind that once it has been properly reviewed
and accepted, all the commits will be squashed into one before merging. This way we can maintain a linear commit
history.

Before a release, the code base will be frozen and no _pull request_ will be accepted until the release is done, with
the only exception of bug fixes. If you send a _pull request_ during a code freeze, you will have to wait a little more
before seeing it merged.

### Pull request test gates

The Gremlins workflow builds this fork and mutation tests the added Go lines
relative to the pull request's merge base. The existing `.gremlins.yaml`
thresholds apply. Lines outside the diff are skipped; a change without mutation
candidates passes without applying thresholds to an empty selection. Pushes to
`main` still run mutation testing across the repository.

Classify every new Go test by name: `TestRegression...` for a fix,
or `TestFeature...` for new behavior. CI rejects unclassified new tests, so
omitting a regression prefix cannot silently bypass the proof check. Existing
test names need no changes. A new named subtest inherits its parent's
classification. Put a new regression under a `TestRegression...` parent;
new subtests under an unclassified historical test fail CI too.

Name each new Go regression test `TestRegression` followed by its behavior,
for example `TestRegressionDiffSeparatedAdditions`. Give each regression a
distinct test or `t.Run` name; unnamed table rows cannot prove separate
regressions. CI runs the head and base suites to discover test and subtest names,
then proves each new regression leaf separately with race detection and caching
disabled. A failing sibling cannot establish proof for a passing case.
Each case must pass on the PR head and fail an assertion against the merge base
with the PR's test files, `testdata` fixtures, and Go dependency files overlaid.
A case that passes on base, fails to compile, panics, times out, skips, or does
not run fails the proof check. An existing regression name is not treated as a
new case; give each new regression its own name. `TestFeature...` cases run
through the normal test suite and do not need to prove a regression.

Keep regression files and helpers compatible with base production signatures.
New APIs used by other tests in the same package can prevent the base tests
from compiling; separate those changes when necessary. The base production
files are never replaced by PR production files. Test-file and fixture symlinks
are rejected rather than copied. A PR without new regression names reports
that no proof cases are required.

CI invokes `python .github/scripts/regression_proof.py "$BASE_COMMIT"` with
Python 3.12, the Go version in `go.mod`, and full Git history. Test files and
fixtures must be tracked by Git. The helper creates and removes a temporary
base snapshot; it leaves the working tree and Git index intact. Keep local
verification targeted: the full-suite discovery runs belong in CI.

### Commit messages

Commit messages follow a convention. Here is an example:

```
area: do a specific thing

Expand on what and how it is done, possibly spanning multiple lines and
being descriptive.

Fixes #123
```

#### First line

This is a short one line summary of what has been done in the commit, prefixed by the package affected (ex. `mutator`
, `docs`, etc.). The commit message should be written as it is answering the question "This commit changes Gremlins to
..."

The first line is separated from the rest by a blank line.

#### Message body

The message body expands on the first line, adding details in a descriptive way. Try to use correct grammar and
punctuation, and don't use Markdown, HTML or other markup languages.

#### Reference

If the commit is related to an issue (most of the time it does), you can add a reference in the
format `KEYWORD #ISSUE_NUMBER`. This
helps [GitHub link](https://docs.github.com/en/issues/tracking-your-work-with-issues/linking-a-pull-request-to-an-issue#linking-a-pull-request-to-an-issue-using-a-keyword)
the commit to the appropriate issue and update its status.

The recognized keywords are:

- close
- closes
- closed
- fix
- fixes
- fixed
- resolve
- resolves
- resolved

Sometimes the team members as well forget to respect all of these rules, but we do our best to be consistent. If you
forget or make mistakes, we will help during the review process.
