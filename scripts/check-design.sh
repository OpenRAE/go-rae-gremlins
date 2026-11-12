#!/usr/bin/env bash
# Run at the design step, before you change implementation files.
# With a path argument, check that single design file.
# With no argument, check the design files changed against the comparison branch.
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."
if [ "$#" -ge 1 ]; then
  exec go run ./tools/architecture -design "$1"
fi
exec go run ./tools/architecture -design-changed -base "${ARCHITECTURE_BASE:-origin/dev}"
