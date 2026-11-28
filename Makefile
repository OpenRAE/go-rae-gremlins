GO_CMD=go
GO_TEST=${GO_CMD} test
GO_BUILD=${GO_CMD} build
RELEASER_CMD=goreleaser
RELEASE=${RELEASER_CMD} release
LINTER_CMD=mise exec -- golangci-lint
LINT_BASE ?= origin/dev
LINT_ADOPTION_BASE ?=
GOVULNCHECK_VERSION=v1.9.0
# The golang.org/x/tools version that go.mod requires. Review the tool before a change.
FIELDALIGNMENT_VERSION=v0.48.0
BINARY_NAME=gremlins
COVER_OUT=coverage.out
TARGET=dist
BIN=${TARGET}/bin

.PHONY: all snap
all: lint test build
snap: lint test snapshot

# Always run the Go command. Go's build cache skips unchanged work.
.PHONY: build
build:
	mkdir -p ${BIN}
	${GO_BUILD} -o ${BIN}/${BINARY_NAME} cmd/gremlins/main.go

.PHONY: test
test:
	${GO_TEST} -race ./...

.PHONY: cover
cover:
	${GO_TEST} -race -covermode=atomic -cover -coverprofile ${COVER_OUT} ./...

.PHONY: requirements
requirements:
	mise install
	mise exec -- go install golang.org/x/tools/go/analysis/passes/fieldalignment/cmd/fieldalignment@${FIELDALIGNMENT_VERSION}

.PHONY: lint
lint:
	python3 scripts/lint.py --base "${LINT_BASE}" --adoption-base "${LINT_ADOPTION_BASE}"

# Scans the module against the Go vulnerability database. The hosted CI job and
# the manual-stage pre-commit hook both call this target.
.PHONY: vuln
vuln:
	${GO_CMD} run golang.org/x/vuln/cmd/govulncheck@${GOVULNCHECK_VERSION} ./...

.PHONY: lint-shrink lint-test lint-cover
lint-shrink:
	python3 scripts/lint.py --base "${LINT_BASE}" --adoption-base "${LINT_ADOPTION_BASE}" --shrink

lint-test:
	python3 -m unittest discover -s scripts -p test_lint.py

lint-cover:
	python3 -m coverage erase
	python3 -m coverage run -m unittest discover -s scripts -p test_lint.py
	python3 -m coverage run -m unittest discover -s .github/scripts -p test_regression_proof.py
	python3 -m coverage run -m unittest discover -s .github/scripts -p test_merge_queue_policy.py
	python3 -m unittest discover -s .github/scripts -p test_workflow_applicability.py
	python3 -m coverage combine
	python3 -m coverage xml -o python-coverage.xml

.PHONY: policy
policy:
	pre-commit validate-config
	pre-commit run --hook-stage pre-commit --all-files

# Reads the live branch rules of the repository with gh.
.PHONY: merge-queue-policy
merge-queue-policy:
	python3 .github/scripts/merge_queue_policy.py --repository OpenRAE/go-rae-gremlins

.PHONY: hooks
hooks:
	bash scripts/install-hooks.sh

# Documentation checks use the pinned doc-tools release. See docs/CONTRIBUTING.md.
DOCS_BASE ?= origin/dev
.PHONY: docs-tools docs-changed docs-samples
docs-tools:
	bash scripts/doc-tools.sh install

docs-changed:
	bash scripts/doc-tools.sh changed "${DOCS_BASE}"

docs-samples:
	bash scripts/doc-tools.sh samples

.PHONY: goimports
goimports:
	${LINTER_CMD} fmt ./...

.PHONY: fieldalignment
fieldalignment:
	fieldalignment -fix ./...

.PHONY: snapshot
snapshot:
	${RELEASE} --snapshot --clean

.PHONY: clean
clean:
	go clean
	rm -rf -- ${TARGET}
	rm -f -- coverage.out

# Architecture policy has no runtime CLI or configuration impact.
ARCHITECTURE_BASE ?= origin/dev
.PHONY: architecture architecture-design
architecture:
	${GO_CMD} run ./tools/architecture -base "${ARCHITECTURE_BASE}"

architecture-design:
	bash scripts/check-design.sh

# go-archcheck checks the module code against architecture/model.yaml. Each design record in
# architecture/designs needs its design/NNNN reservation tag in origin. With ARCHCHECK_BASE, a
# structural model change needs a new design record. An empty ARCHCHECK_BASE, or a base with no
# model, skips that comparison. See docs/development/architecture.md.
ARCHCHECK_VERSION=v0.0.0-20261128001710-5746a1ce4bef
# go-archcheck requires a newer Go than go.mod. CI sets GOTOOLCHAIN=local, so name the toolchain.
ARCHCHECK_GO=go1.27.4
ARCHCHECK=GOTOOLCHAIN=${ARCHCHECK_GO} ${GO_CMD} run github.com/OpenRAE/go-archcheck/cmd/archcheck@${ARCHCHECK_VERSION}
ARCHCHECK_BASE ?= origin/dev
.PHONY: archcheck archcheck-reserve
archcheck:
	mkdir -p .cache
	git fetch --quiet --prune origin '+refs/tags/design/*:refs/tags/design/*'
	git for-each-ref --format='%(refname)%09%(objecttype)%09%(contents:subject)' refs/tags/design/ > .cache/reservations.txt
	@base='${ARCHCHECK_BASE}'; \
	if [ -n "$$base" ]; then git rev-parse --quiet --verify "$$base^{commit}" > /dev/null || { echo "unknown ARCHCHECK_BASE $$base" >&2; exit 1; }; fi; \
	if [ -n "$$base" ] && git cat-file -e "$$base:architecture/model.yaml" 2>/dev/null; then \
		git show "$$base:architecture/model.yaml" > .cache/base-model.yaml; \
		git ls-tree --name-only "$$base" architecture/designs/ > .cache/base-designs.txt; \
		echo "archcheck: compare with $$base"; \
		${ARCHCHECK} -reservations .cache/reservations.txt -base-model .cache/base-model.yaml -base-designs .cache/base-designs.txt; \
	else \
		echo "archcheck: no base model to compare"; \
		${ARCHCHECK} -reservations .cache/reservations.txt; \
	fi

# Reserve the next design number for issue ISSUE. GH_TOKEN needs write access to the contents
# of the repository, for example GH_TOKEN="$(gh auth token)".
archcheck-reserve:
	@if [ -z "${ISSUE}" ]; then echo "set ISSUE to the issue number" >&2; exit 2; fi
	${ARCHCHECK} reserve -repo OpenRAE/go-rae-gremlins -issue "${ISSUE}"

# Prove that the cgroup-v2 memory budget stops a runaway allocator. Linux only.
.PHONY: memcheck
memcheck:
	bash scripts/memory-budget-check.sh

# Run the Phase E transport harness. Set GROUND_CONTROL_TEST_ROOT to the installed
# node_modules/grndctl directory of the pinned utility (docs/development/phase-e.md).
.PHONY: phase-e-test
phase-e-test:
	node --test .github/ground-control/phase-e.integration.test.mjs
