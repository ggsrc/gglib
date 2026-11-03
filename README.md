# gglib

gglib is a set of small Go modules for building gRPC/HTTP services: application
lifecycle, gRPC server and client setup, interceptors, logging, tracing,
metrics, health checks, and wrappers for Postgres and Redis.

Each directory with a `go.mod` is its own module, versioned and released on its
own. Depend only on the modules you use.

## Modules

| Module | What it provides | Depends on (gglib) |
|---|---|---|
| [`app`](app) | Application lifecycle: starts the gRPC, metrics and health servers, handles signals, drains RPCs, then closes resources | grpc, health, metric, resource, zerolog |
| [`env`](env) | Environment name, host name, service name and build information | — |
| [`grpc`](grpc) | gRPC server and client constructors with the standard interceptor chain | env, interceptor |
| [`health`](health) | HTTP liveness and readiness endpoints | — |
| [`interceptor`](interceptor) | gRPC interceptors: context propagation, logging, error logging, panic recovery, metadata helpers | mctx, zerolog |
| [`mctx`](mctx) | Request context values (application context, user, location) | — |
| [`metric`](metric) | Prometheus metrics server | — |
| [`otel`](otel) | OpenTelemetry setup for traces, metrics and logs over OTLP | env, zerolog |
| [`resource`](resource) | Shared-resource registry: init, start, stop and health checks | — |
| [`resource/cache`](resource/cache) | Redis and two-level (Redis + in-memory) cache resources | — |
| [`resource/wpgx`](resource/wpgx) | Postgres pool resource built on [wpgx](https://github.com/stumble/wpgx) | zerolog |
| [`utils`](utils) | Tracing helpers | env |
| [`zerolog`](zerolog) | zerolog setup with trace correlation, and a context-aware `log` package | env |

Install a module with:

```sh
go get github.com/ggsrc/gglib/grpc@latest
```

## Versioning

- Every module is tagged on its own as `<module directory>/vX.Y.Z`, for example
  `grpc/v0.1.0` or `resource/wpgx/v0.1.0`. There are no repository-wide `v*`
  tags.
- All modules are at major version 0. A minor bump (`v0.1.0` → `v0.2.0`) may
  contain breaking changes; a patch bump does not. Behaviour changes are listed
  in each module's changelog.
- A pushed tag is never moved or deleted. A bad release is fixed by the next
  patch release, which `retract`s the bad version in its `go.mod`.
- When a change spans modules, the lower module is released first, and the
  higher module then requires that new version.

Release order, lowest first:

1. env, health, mctx, metric, resource, resource/cache
2. zerolog, utils
3. interceptor, resource/wpgx, otel
4. grpc
5. app

## Go version

Every module declares `go 1.27.0`, the oldest Go release gglib supports.

## Development

Every module is checked on its own with `GOWORK=off`. Sibling gglib modules
resolve to the directories in this repository through `replace` directives, so
a change that spans modules is built and tested as a whole. Consumers ignore
`replace` and use the versions in `require`, which the release script sets.
After adding or removing a dependency on another gglib module, run
`make replaces`.

The Makefile runs a target in every module:

```sh
make tidy-check   # go mod tidy -diff
make build        # go build -mod=readonly ./...
make vet
make test         # go test -race
make cover        # tests with coverage, checked against <module>/.coverage-floor
make lint         # golangci-lint (configuration in .golangci.yml)
make lint-fix     # golangci-lint --fix
make vulncheck    # govulncheck
```

CI (`.github/workflows/ci.yml`) runs the same checks per module on every pull
request and push to `main`, and weekly so that new vulnerability advisories
show up. `ci-ok` is the single required check. Coverage may not drop below a
module's floor in `<module>/.coverage-floor`; raise the floor when you raise
coverage. Dependabot proposes grouped dependency updates weekly.

## Releasing

Each module has a `CHANGELOG.md`; add entries under `## Unreleased` with every
change. To release:

1. Rename `## Unreleased` to `## vX.Y.Z - YYYY-MM-DD` in each module being
   released, through a normal PR.
2. On an up-to-date `main`, run `scripts/release.sh prepare vX.Y.Z [module...]`.
   It creates `release/vX.Y.Z`, points the internal requires at the new version,
   checks that each released module only requires released sibling versions,
   runs the checks and commits. Push the branch and merge its PR.
3. On `main` after the merge, run `scripts/release.sh tag vX.Y.Z [module...]`
   and push the tags it prints.
4. Run `scripts/release.sh verify vX.Y.Z [module...]` to build the released
   modules from the module proxy, as a consumer would.

Without a module list, every module is released. Release lower modules first
when only some modules change (see the release order above).

## Security

See [SECURITY.md](SECURITY.md).

## License

[Apache License 2.0](LICENSE)
