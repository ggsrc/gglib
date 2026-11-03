#!/usr/bin/env bash
# Points every module at the sibling modules in this repository.
#
# Each go.mod gets a `replace github.com/ggsrc/gglib/<dir> => <relative dir>`
# for every gglib module in its build graph, so builds, tests and linters in
# this repository always use the code in the same commit. Consumers ignore
# replace directives and resolve the versions listed in `require`, which the
# release script sets (scripts/release.sh).
#
# Run after adding or removing an internal dependency; `make tidy-check`
# fails while a replace is missing.
set -euo pipefail
cd "$(dirname "$0")/.."
export GOWORK=off

prefix=github.com/ggsrc/gglib
modules=$(find . -name go.mod -not -path './.git/*' -exec dirname {} \; | sed 's|^\./||' | sort)

for m in $modules; do
  up=$(echo "$m" | sed -E 's|[^/]+|..|g')
  while :; do
    added=0
    for dep in $(cd "$m" && go mod edit -json | jq -r --arg p "$prefix/" \
      '[.Require[]?.Path | select(startswith($p))] - [(.Replace // [])[].Old.Path] | .[]'); do
      dir=${dep#"$prefix/"}
      if [ ! -f "$dir/go.mod" ]; then
        echo "$m requires $dep, which is not a module in this repository" >&2
        exit 1
      fi
      (cd "$m" && go mod edit -replace="$dep=$up/$dir")
      added=1
    done
    (cd "$m" && go mod tidy)
    [ "$added" = 1 ] || break
  done

  # Rewrite the gglib replaces as one sorted block.
  reps=$(cd "$m" && go mod edit -json | jq -r --arg p "$prefix/" \
    '(.Replace // [])[] | select(.Old.Path | startswith($p)) | "\t\(.Old.Path) => \(.New.Path)"' | sort)
  if [ -n "$reps" ]; then
    for old in $(echo "$reps" | awk '{print $1}'); do
      (cd "$m" && go mod edit -dropreplace="$old")
    done
    printf '\n// Sibling modules in this repository (scripts/replaces.sh).\nreplace (\n%s\n)\n' "$reps" >> "$m/go.mod"
    (cd "$m" && go mod edit -fmt)
  fi
done
