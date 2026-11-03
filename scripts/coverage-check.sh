#!/usr/bin/env bash
# Checks a module's coverage profile against its floor in .github/coverage.txt.
#
# Usage, from the module directory:
#   go test -coverprofile=coverage.out ./...
#   ../scripts/coverage-check.sh <module> coverage.out
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
module=${1:?module}
profile=${2:?coverage profile}

total=$(go tool cover -func="$profile" | awk '/^total:/ { sub("%", "", $NF); print $NF }')
floor=$(awk -v m="$module" '!/^#/ && $1 == m { print $2 }' "$root/.github/coverage.txt")
if [ -z "$floor" ]; then
  echo "no coverage floor for $module in .github/coverage.txt" >&2
  exit 1
fi
echo "$module: coverage ${total}% (floor ${floor}%)"
if ! awk -v t="$total" -v f="$floor" 'BEGIN { exit !(t + 0 >= f + 0) }'; then
  echo "$module: coverage ${total}% is below the floor ${floor}%" >&2
  exit 1
fi
