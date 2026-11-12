GO_CMD=go
GO_TEST=${GO_CMD} test
GO_BUILD=${GO_CMD} build
RELEASER_CMD=goreleaser
RELEASE=${RELEASER_CMD} release
LINTER_CMD=mise exec -- golangci-lint
LINT_BASE ?= origin/dev
LINT_ADOPTION_BASE ?=
BINARY_NAME=gremlins
COVER_OUT=coverage.out
TARGET=dist
BIN=${TARGET}/bin

.PHONY: all snap
all: lint test build
snap: lint test snapshot

build: ${BIN}/${BINARY_NAME}

${BIN}/${BINARY_NAME}:
	mkdir -p ${BIN}
	${GO_BUILD} -o ${BIN}/${BINARY_NAME} cmd/gremlins/main.go

.PHONY: test
test:
	${GO_TEST} -race ./...

.PHONY: cover
cover: ${COVER_OUT}

${COVER_OUT}:
	${GO_TEST} -race -covermode=atomic -cover -coverprofile ${COVER_OUT} ./...

.PHONY: requirements
requirements:
	mise install
	go install golang.org/x/tools/go/analysis/passes/fieldalignment/cmd/fieldalignment@latest

.PHONY: lint
lint:
	python3 scripts/lint.py --base "${LINT_BASE}" --adoption-base "${LINT_ADOPTION_BASE}"

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

# Prove that the cgroup-v2 memory budget stops a runaway allocator. Linux only.
.PHONY: memcheck
memcheck:
	bash scripts/memory-budget-check.sh
