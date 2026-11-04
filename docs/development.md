# Development and releases

## Local checks

Every module is checked on its own with `GOWORK=off`. Sibling gglib modules
resolve to the directories in this repository through `replace` directives, so
a change that spans modules is built and tested as a whole. Consumers ignore
`replace` and use the versions in `require`, which the release script sets.
After adding or removing a dependency on another gglib module, run
`make replaces`.

The Makefile runs each target in every module:

| Command | What it does |
|---|---|
| `make tidy-check` | `go mod tidy -diff` |
| `make build` | `go build -mod=readonly ./...` |
| `make vet` | `go vet` |
| `make test` | `go test -race`. The resource/wpgx and resource/cache tests start real Postgres and Redis with Testcontainers, so Docker must be running. |
| `make cover` | Tests with coverage, checked against `<module>/.coverage-floor` |
| `make lint`, `make lint-fix` | golangci-lint, configured in `.golangci.yml` |
| `make vulncheck` | govulncheck |

## CI

`.github/workflows/ci.yml` runs the checks per module on every pull request,
on every push to `main`, and weekly so that new vulnerability advisories show
up. `ci-ok`, which summarizes all modules, is the single required check.
Dependabot proposes grouped dependency updates weekly.

Coverage may not drop below a module's floor; raise the floor when you raise
coverage.

| Module | Floor | Module | Floor |
|---|---|---|---|
| env, mctx, metric, resource, resource/wpgx, utils | 100.0 | health | 98.0 |
| grpc | 98.9 | resource/cache | 97.9 |
| zerolog | 97.5 | app | 96.0 |
| interceptor | 95.8 | otel | 90.0 |

## Branch and tag rules

- `main`: changes go through a pull request; `ci-ok` must pass; squash merges
  only; no force pushes or deletion.
- Tags `<module>/vX.Y.Z` can be created but never moved or deleted.

## Versioning

- Every module is tagged on its own as `<module directory>/vX.Y.Z`, for example
  `grpc/v0.1.0` or `resource/wpgx/v0.1.0`. There are no repository-wide tags.
- All modules are at major version 0. A minor bump (`v0.1.0` → `v0.2.0`) may
  contain breaking changes; a patch bump does not. Each module's
  `CHANGELOG.md` lists its behaviour changes.
- A bad release is fixed by the next patch release, which `retract`s the bad
  version in its `go.mod`.
- When a change spans modules, the lower module is released first, and the
  higher module then requires that new version.
- Every module declares `go 1.27.0`, the oldest Go release gglib supports.

Release order, lowest first:

1. env, health, mctx, metric, resource, resource/cache
2. zerolog, utils
3. interceptor, resource/wpgx, otel
4. grpc
5. app

## Releasing

Add entries under `## Unreleased` in a module's `CHANGELOG.md` with every
change. To release:

1. Rename `## Unreleased` to `## vX.Y.Z - YYYY-MM-DD` in each module being
   released, through a normal pull request.
2. On an up-to-date `main`, run `scripts/release.sh prepare vX.Y.Z [module...]`.
   It creates `release/vX.Y.Z`, points the internal requires at the new
   version, checks that each released module only requires released sibling
   versions, runs the checks and commits. Push the branch and merge its pull
   request.
3. On `main` after the merge, run `scripts/release.sh tag vX.Y.Z [module...]`
   and push the tags it prints.
4. Run `scripts/release.sh verify vX.Y.Z [module...]` to build the released
   modules from the module proxy, as a consumer would.

Without a module list, every module is released. Release lower modules first
when only some modules change.

## Documentation

Each module's `README.md` covers that module completely: API, configuration,
behaviour and failure cases. Topics that span modules live in this `docs/`
directory. Update the README in the same pull request as a behaviour change.
