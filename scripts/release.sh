#!/usr/bin/env bash
# Releases gglib modules. Each module is tagged on its own as <dir>/vX.Y.Z.
#
#   scripts/release.sh prepare v0.1.0 [module...]
#       On a clean checkout of main: creates branch release/<version>, sets
#       every internal require on the released modules to <version>, checks
#       that each released module requires only released sibling versions,
#       runs the checks and commits. Push the branch and open a PR.
#   scripts/release.sh tag v0.1.0 [module...]
#       After the release PR is merged, on main at origin/main: checks the
#       go.mod files and changelogs again and creates annotated tags. It
#       prints the push command; it never pushes.
#   scripts/release.sh verify v0.1.0 [module...]
#       After the tags are pushed: builds every released package the way a
#       consumer does (no replace directives, versions from the module proxy).
#
# Without a module list, every module is released. Each released module needs
# a "## <version>" section in its CHANGELOG.md. A pushed tag is never moved or
# deleted; a bad release is fixed by the next patch release.
set -euo pipefail
cd "$(dirname "$0")/.."
export GOWORK=off

prefix=github.com/ggsrc/gglib

die() { echo "release: $*" >&2; exit 1; }

all_modules() {
  find . -name go.mod -not -path './.git/*' -exec dirname {} \; | sed 's|^\./||' | sort
}

internal_requires() { # <module dir> -> "path version" lines
  (cd "$1" && go mod edit -json | jq -r --arg p "$prefix/" \
    '.Require[]? | select(.Path | startswith($p)) | "\(.Path) \(.Version)"')
}

[ $# -ge 2 ] || die "usage: $0 prepare|tag|verify vX.Y.Z [module...]"
cmd=$1 version=$2
shift 2
[[ $version =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "version must look like v1.2.3, got $version"

if [ $# -gt 0 ]; then
  selected=$(printf '%s\n' "$@" | sort -u)
else
  selected=$(all_modules)
fi
for m in $selected; do
  [ -f "$m/go.mod" ] || die "$m is not a module"
done
is_selected() { printf '%s\n' $selected | grep -qxF "$1"; }

check_changelogs() {
  for m in $selected; do
    grep -qE "^## \\[?${version//./\\.}\\]?( |$)" "$m/CHANGELOG.md" 2>/dev/null ||
      die "$m/CHANGELOG.md has no \"## $version\" section"
  done
}

check_tags_free() {
  git fetch -q --tags origin
  for m in $selected; do
    ! git rev-parse -q --verify "refs/tags/$m/$version" >/dev/null || die "tag $m/$version already exists"
  done
}

# Every internal require of a released module must name a version a consumer
# can resolve: one released now, or an existing tag.
check_consumer_view() {
  for m in $selected; do
    while read -r path ver; do
      [ -n "$path" ] || continue
      dir=${path#"$prefix/"}
      if is_selected "$dir"; then
        [ "$ver" = "$version" ] || die "$m requires $path $ver, expected $version"
      else
        git rev-parse -q --verify "refs/tags/$dir/$ver" >/dev/null ||
          die "$m requires $path $ver, which is not a released version; release $dir too"
      fi
    done < <(internal_requires "$m")
  done
}

require_clean() {
  [ -z "$(git status --porcelain)" ] || die "the working tree is not clean"
}

case $cmd in
prepare)
  require_clean
  [ "$(git rev-parse --abbrev-ref HEAD)" = main ] || die "run prepare on main"
  check_changelogs
  check_tags_free
  git switch -q -c "release/$version"
  # On any failure, drop the release branch and return to a clean main.
  prepared=no
  cleanup() {
    [ "$prepared" = yes ] && return
    git checkout -q -- . && git switch -q main && git branch -q -D "release/$version"
    echo "release: prepare failed; release/$version removed" >&2
  }
  trap cleanup EXIT
  for m in $(all_modules); do
    while read -r path _; do
      [ -n "$path" ] || continue
      if is_selected "${path#"$prefix/"}"; then
        (cd "$m" && go mod edit -require="$path@$version")
      fi
    done < <(internal_requires "$m")
  done
  for m in $(all_modules); do (cd "$m" && go mod tidy); done
  check_consumer_view
  make tidy-check build vet test
  git commit -q -am "release: $(printf '%s ' $selected)$version"
  prepared=yes
  echo "Committed on release/$version. Push it and open a PR; after it is merged run:"
  echo "  $0 tag $version $*"
  ;;
tag)
  require_clean
  [ "$(git rev-parse --abbrev-ref HEAD)" = main ] || die "run tag on main"
  git fetch -q origin main
  [ "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)" ] || die "main is not at origin/main"
  check_changelogs
  check_tags_free
  check_consumer_view
  tags=()
  for m in $selected; do
    git tag -a "$m/$version" -m "$prefix/$m $version"
    tags+=("$m/$version")
  done
  echo "Created ${#tags[@]} tags on $(git rev-parse --short HEAD). Push them with:"
  echo "  git push origin ${tags[*]}"
  echo "then run: $0 verify $version $*"
  ;;
verify)
  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' EXIT
  (
    cd "$tmp"
    go mod init example.com/gglib-release-verify >/dev/null 2>&1
    for m in $selected; do go get "$prefix/$m@$version"; done
    for m in $selected; do go build "$prefix/$m/..."; done
  )
  echo "All $(printf '%s\n' $selected | wc -l) modules build at $version from the module proxy."
  ;;
*)
  die "unknown command $cmd"
  ;;
esac
