# ci: per-module CI, coverage floors, sibling replaces and a release script

Tracking: alva-ai/mono-meta#1115 (layer L1, gglib). Fourth PR before
`v0.1.0`, stacked on #30.

## 1. Current state

- gglib has no CI. Checks run only when someone runs `make` locally.
- Internal requires point at a pseudo-version of `main` (`b474e43`). A module
  is built against the siblings at that commit, not against the siblings in
  the same checkout. A change that spans modules (the coming G5 work touches
  otel, zerolog and app at once) therefore cannot be checked in one PR, and a
  fix in a lower module is invisible to the modules above it until the
  pseudo-versions are bumped.
- There is no release procedure, no per-module changelog and no dependency
  update automation.

## 2. Behavior after this change

- **CI** (`.github/workflows/ci.yml`), on every pull request, every push to
  `main`, weekly, and on demand:
  - a repository job lists the modules, checks that each has a coverage floor
    and a Dependabot entry, and that the sibling replaces are complete;
  - one job per module: `go mod tidy -diff`, build, vet, `go test -race` with
    coverage, the coverage floor, golangci-lint v2.14.0 and govulncheck
    v1.9.0;
  - `ci-ok`, the single check to require in the `main` ruleset.
  Actions are pinned to commit SHAs, the token is read-only, and checkout does
  not persist credentials.
- **Sibling replaces**: every go.mod has a `replace` block that points each
  gglib module in its build graph at the directory in this repository
  (`scripts/replaces.sh`, `make replaces`). Builds and tests always use the
  code of the same commit. Consumers ignore `replace` and resolve `require`.
- **Coverage floors** (`<module>/.coverage-floor`, one file per module so
  that module PRs never conflict): today's coverage per module
  (app 16.7, interceptor 43.8, zerolog 2.8, the rest 0). CI fails below a
  floor; each module PR raises its floor towards the targets (80, or 90 for
  app, health, mctx, resource and the interceptor).
- **Release script** (`scripts/release.sh prepare|tag|verify`): prepares a
  release branch that points internal requires at the new version, refuses a
  release whose modules require unreleased sibling versions or lack a
  changelog section, tags after merge, and verifies the published modules
  from the module proxy. It never pushes.
- **Changelogs**: every module has a `CHANGELOG.md` with an `Unreleased`
  section that records the changes since `b474e43`.
- **Dependabot**: weekly grouped updates for Go modules (all module
  directories, gglib modules excluded) and GitHub Actions, with a 7-day
  cooldown on new releases.

## 3. Decisions

- Replace directives instead of pseudo-version bumps. This is how other
  multi-module repositories that release modules together work (for example
  opentelemetry-go). It matches D5 (every module tagged `v0.1.0` on one
  commit). The consumer view (no replaces) is checked by the release script:
  `prepare`/`tag` require released sibling versions, and `verify` builds the
  published modules.
- `make work` is removed: with replaces, a workspace adds nothing.
- A self-contained workflow rather than `ggsrc/public-ci`: public-ci's
  workflows assume a single module at the repository root and require a PAT
  secret, which a public library does not need.
- govulncheck blocks pull requests. The weekly run surfaces new advisories
  on `main` without a code change.
- Go version in CI: the latest 1.27 patch (`1.27.x`); the `go` directive
  stays `go 1.27.0`.

## 4. Changes

- New: `.github/workflows/ci.yml`, `*/.coverage-floor`,
  `.github/dependabot.yml`, `scripts/replaces.sh`,
  `scripts/coverage-check.sh`, `scripts/release.sh`, `*/CHANGELOG.md`.
- All `go.mod`/`go.sum`: replace block for sibling modules; go.sum entries for
  the pseudo-versioned siblings are gone.
- `Makefile`: `replaces`, `cover`, `vulncheck`; `work` removed.
- `README.md`: Development and Releasing sections.

## 5. Verification

Go 1.27.3, `GOWORK=off`:

- `make tidy-check build vet test lint cover vulncheck` — all 13 modules pass.
- `scripts/replaces.sh` run twice leaves go.mod files unchanged.
- `actionlint` v1.7.7 on the workflow: no findings.
- Release script, in a scratch clone with a local bare remote:
  - `prepare v0.1.0` (all modules) sets internal requires to `v0.1.0`, passes
    the checks and commits; `tag v0.1.0` after a fast-forward of `main`
    creates 13 tags and prints the push command.
  - `prepare v0.1.1 zerolog` after that bumps only the zerolog requires.
  - `prepare v0.1.0 app` without released siblings fails ("requires …grpc
    v0.0.0-…, which is not a released version") and removes the branch.
  - A module without a `## v0.1.0` changelog section is refused.
  - `verify` was not run: it needs pushed tags.
- The workflow itself runs for the first time on this PR.

## 6. Human decisions

None for this PR. Repository settings come after CI has run once: the `main`
ruleset (require `ci-ok` and a review) and a tag ruleset for `*/v*`. Who can
approve PRs opened from the Stumble account is asked before enabling them.

## 7. Outcome

- [x] Implemented and verified locally.
- [ ] CI green on this PR.
- [ ] Reviewed by Yumin, merged after #30.

## 8. Remaining work

- Per-module PRs raise the coverage floors and remove the temporary lint
  exclusions.
- Rulesets for `main` and tags, after Yumin confirms the approver.
