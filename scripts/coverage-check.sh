#!/usr/bin/env bash
# Checks a module's coverage profile against its floor, the number in
# <module>/.coverage-floor (minimum statement coverage in percent). Raise the
# floor in the PR that raises coverage; never lower it. Targets for v0.1.0:
# 80 for every module, 90 for app, health, mctx, resource and the
# interceptor (request context, recovery).
#
# Usage, from the module directory:
#   go test -coverprofile=coverage.out ./...
#   ../scripts/coverage-check.sh <module> coverage.out
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
module=${1:?module}
profile=${2:?coverage profile}

total=$(go tool cover -func="$profile" | awk '/^total:/ { sub("%", "", $NF); print $NF }')
floor=$(tr -d '[:space:]' < "$root/$module/.coverage-floor" 2>/dev/null || true)
if [ -z "$floor" ]; then
  echo "no coverage floor for $module: create $module/.coverage-floor" >&2
  exit 1
fi
echo "$module: coverage ${total}% (floor ${floor}%)"
if ! awk -v t="$total" -v f="$floor" 'BEGIN { exit !(t + 0 >= f + 0) }'; then
  echo "$module: coverage ${total}% is below the floor ${floor}%" >&2
  exit 1
fi
