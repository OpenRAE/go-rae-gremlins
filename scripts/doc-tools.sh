#!/usr/bin/env bash
# Install the pinned doc-tools release and run its STE check on explicit Markdown paths.
# Release: https://github.com/OpenRAE/doc-tools/releases/tag/v0.1.0
# To upgrade, set VERSION and SHA256 from the SHA256SUMS file of the new release.
set -euo pipefail

VERSION=0.1.0
SHA256=125104c0cc535698d1cc9bba3e57883bdf9d87db6da346ff583967c4ce42de6c
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
PARENT="$ROOT/.tools/doc-tools"
DEST="$PARENT/$VERSION"
LINT="$DEST/tools/ste-lint.sh"
CLEANUP=()
cleanup() {
  if [[ ${#CLEANUP[@]} -gt 0 ]]; then
    rm -rf -- "${CLEANUP[@]}"
  fi
}
trap cleanup EXIT

usage() {
  cat >&2 <<'EOF'
Usage:
  bash scripts/doc-tools.sh install          Install and verify the pinned package and ste-cli.
  bash scripts/doc-tools.sh lint FILE...     Check explicit Markdown paths.
  bash scripts/doc-tools.sh changed BASE     Check Markdown that changed since the merge base with BASE.
  bash scripts/doc-tools.sh samples          Show that a bad sample fails and its corrected form passes.
EOF
  exit 2
}

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | cut -d' ' -f1
  else
    echo 'No SHA-256 tool is available. The installation stops.' >&2
    exit 2
  fi
}

install_package() {
  if [[ ! -f "$LINT" ]]; then
    local work archive actual staging
    work="$(mktemp -d)"
    CLEANUP+=("$work")
    archive="$work/doc-tools-$VERSION.tar.gz"
    curl --proto '=https' --fail --silent --show-error --location -o "$archive" \
      "https://github.com/OpenRAE/doc-tools/releases/download/v$VERSION/doc-tools-$VERSION.tar.gz"
    actual="$(sha256_of "$archive")"
    if [[ "$actual" != "$SHA256" ]]; then
      echo "doc-tools $VERSION checksum mismatch. The installation stops." >&2
      exit 1
    fi
    mkdir -p "$PARENT"
    # Extract into a private directory and rename it, so a partial extraction is never used.
    staging="$(mktemp -d "$PARENT/.partial.XXXXXX")"
    CLEANUP+=("$staging")
    tar -xzf "$archive" -C "$staging" --strip-components=1
    if [[ ! -e "$DEST" ]]; then
      mv "$staging" "$DEST"
    fi
  fi
  # The package installer verifies the ste-cli archive and uses an installed binary.
  bash "$DEST/tools/install-ste.sh" >&2
}

lint_paths() {
  install_package
  local paths=() path
  for path in "$@"; do
    # A relative path that starts with a hyphen must not become an option.
    [[ "$path" == /* || "$path" == ./* ]] || path="./$path"
    paths+=("$path")
  done
  bash "$LINT" "${paths[@]}"
}

# Repository rules: release-please writes CHANGELOG.md, the Code of Conduct is adopted
# policy text, and testdata files are byte-exact test inputs.
excluded() {
  case "$1" in
    CHANGELOG.md | CODE_OF_CONDUCT.md | testdata/* | */testdata/*) return 0 ;;
  esac
  return 1
}

lint_changed() {
  local base="$1" merge_base path paths=()
  merge_base="$(git -C "$ROOT" merge-base "$base" HEAD)"
  cd "$ROOT"
  while IFS= read -r -d '' path; do
    if [[ -f "$path" ]] && ! excluded "$path"; then
      paths+=("$path")
    fi
  done < <(
    {
      git diff --name-only --diff-filter=ACMRT -z "$merge_base" -- '*.md'
      git ls-files --others --exclude-standard -z -- '*.md'
    } | sort -zu
  )
  if [[ ${#paths[@]} -eq 0 ]]; then
    echo 'No changed Markdown files.'
    return 0
  fi
  lint_paths "${paths[@]}"
}

expect_rule() {
  if ! grep -qF "[$1]" "$2"; then
    echo "The bad sample did not report $1." >&2
    exit 1
  fi
}

check_samples() {
  install_package
  local work
  work="$(mktemp -d)"
  CLEANUP+=("$work")
  cat >"$work/bad.md" <<'EOF'
You can't start the lab until you have read all of the instructions and verified each prerequisite on the host that will run this exercise on the supported installation.

The lab has a network. It has a server. It has a client. It has a router. It has a log. It has a clock. It has a disk.
EOF
  cat >"$work/corrected.md" <<'EOF'
Read all of the instructions. Verify each prerequisite on the host that runs the exercise.
Then start the lab.

The lab has a network, a server, a client, and a router. It also has a log, a clock, and a disk.
EOF
  if bash "$LINT" "$work/bad.md" >"$work/bad.out" 2>&1; then
    cat "$work/bad.out"
    echo 'The bad sample must fail.' >&2
    exit 1
  fi
  cat "$work/bad.out"
  expect_rule STE-4.2 "$work/bad.out"
  expect_rule STE-5.1 "$work/bad.out"
  expect_rule STE-6.6 "$work/bad.out"
  bash "$LINT" "$work/corrected.md"
  echo 'The bad sample fails and its corrected form passes.'
}

[[ $# -ge 1 ]] || usage
command="$1"
shift
case "$command" in
  install)
    [[ $# -eq 0 ]] || usage
    install_package
    ;;
  lint)
    [[ $# -ge 1 ]] || usage
    lint_paths "$@"
    ;;
  changed)
    [[ $# -eq 1 ]] || usage
    lint_changed "$1"
    ;;
  samples)
    [[ $# -eq 0 ]] || usage
    check_samples
    ;;
  *) usage ;;
esac
