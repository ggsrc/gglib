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

# Sibling gglib modules in the module graph of the module in the current
# directory (requirements of requirements included).
graph_siblings() {
  go list -m -f '{{if not .Main}}{{.Path}}{{end}}' all | grep "^$prefix/" || true
}

replaced_siblings() {
  go mod edit -json | jq -r --arg p "$prefix/" '(.Replace // [])[].Old.Path | select(startswith($p))'
}

for m in $modules; do
  up=$(echo "$m" | sed -E 's|[^/]+|..|g')
  while :; do
    added=0
    for dep in $(cd "$m" && comm -23 <(graph_siblings | sort -u) <(replaced_siblings | sort -u)); do
      dir=${dep#"$prefix/"}
      if [ ! -f "$dir/go.mod" ]; then
        echo "$m needs $dep, which is not a module in this repository" >&2
        exit 1
      fi
      (cd "$m" && go mod edit -replace="$dep=$up/$dir")
      added=1
    done
    (cd "$m" && go mod tidy)
    [ "$added" = 1 ] || break
  done

  # Drop replaces for siblings no longer in the graph.
  for stale in $(cd "$m" && comm -13 <(graph_siblings | sort -u) <(replaced_siblings | sort -u)); do
    (cd "$m" && go mod edit -dropreplace="$stale")
  done
  (cd "$m" && go mod tidy)

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
