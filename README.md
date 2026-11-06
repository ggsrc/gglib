# gglib

gglib is a set of small Go modules for building gRPC services. A service's
`main` usually does three things with it: start the process with `app`, manage
its database and Redis connections with `resource`, and serve gRPC with `grpc`.
Logging, tracing, metrics and health checks — the parts every service needs —
are set up the same way everywhere, so each service does not write its own.

- Go 1.27 or later. Apache License 2.0.
- Each directory with a `go.mod` is its own module, versioned and released on
  its own. Depend only on the modules you use.

## A complete service

```go
package main

import (
	"context"
	"os"

	"github.com/ggsrc/gglib/app"
	"github.com/ggsrc/gglib/grpc"
	"github.com/ggsrc/gglib/resource"
	"github.com/ggsrc/gglib/resource/cache"
	"github.com/ggsrc/gglib/resource/wpgx"
	"github.com/ggsrc/gglib/zerolog/log"
)

func main() {
	if err := run(context.Background()); err != nil {
		log.Error().Err(err).Msg("exiting")
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	// 1. Logging, telemetry, the metrics server (:4014) and the health
	//    server (:8080). The process is now alive but not ready.
	a, err := app.New(ctx, app.WithServerName("my-service"))
	if err != nil {
		return err
	}

	// 2. Dependencies. Register what others depend on first: shutdown runs
	//    in reverse order. Init retries until every resource is up.
	db := wpgx.NewWPGXWithDefaultEnvPrefix()               // POSTGRES_*
	kv := cache.NewCacheWithDefaultEnvPrefix("my-service") // REDIS_*, DCACHE_*
	rm := resource.NewResourceManager([]resource.Resource{db, kv})
	if err := a.Init(ctx, rm); err != nil {
		return err
	}

	// 3. Your service (pb is your generated gRPC package, newHandler your
	//    implementation).
	srv := grpc.NewServerWithDefaultEnvPrefix() // GRPC_PORT, default 9090
	pb.RegisterMyServiceServer(srv.Server(), newHandler(db.GetPool(), kv.GetDCache()))

	// 4. Start everything, report ready, block until SIGTERM, shut down in order.
	return a.Run(ctx, srv, rm)
}
```

[docs/lifecycle.md](docs/lifecycle.md) walks through what each step does, how
the health probes answer at every stage, what happens when a dependency fails,
and the shutdown order.

## Modules

| Module | What it provides | Depends on (gglib) |
|---|---|---|
| [`app`](app) | Process lifecycle: logging and telemetry, metrics and health servers from the start, resource initialization with retries, an optional gRPC server, and an ordered shutdown | env, grpc, health, metric, otel, resource, zerolog |
| [`env`](env) | Environment name (prd, stg, beta, test, local), service name and service version | — |
| [`zerolog`](zerolog) | One-call logging setup (level, format, trace correlation, OpenTelemetry bridge) and a context-aware `log` package | env |
| [`otel`](otel) | One-call OpenTelemetry setup for traces, logs and (optional) metrics over OTLP/HTTP | env |
| [`metric`](metric) | Prometheus metrics server | — |
| [`health`](health) | Kubernetes liveness and readiness endpoints driven by background checks | — |
| [`resource`](resource) | Lifecycle for dependencies: init with resume, ordered start, reverse-order stop, health checks | — |
| [`resource/wpgx`](resource/wpgx) | Postgres connection pool as a resource, built on [wpgx](https://github.com/stumble/wpgx) | — |
| [`resource/cache`](resource/cache) | Redis client and two-level (in-memory + Redis) cache as resources | — |
| [`resource/goroutine`](resource/goroutine) | Long-lived background goroutines as a resource: canceled and awaited on shutdown, panics recovered | resource, safego, zerolog |
| [`safego`](safego) | Goroutines that recover their own panics: fire-and-forget, async results and fan-out groups | zerolog |
| [`ratelimit`](ratelimit) | Redis-backed per-key rate limiting (GCRA): admit or reject, with the exact retry-after | — |
| [`snowflake`](snowflake) | Unique 64-bit IDs, with each process's node number taken from a Redis counter | — |
| [`pprof`](pprof) | Guarded on-demand profiling server: goroutine, heap and bounded CPU profiles, off by default | — |
| [`grpc`](grpc) | gRPC server and client constructors with a standard interceptor chain | interceptor |
| [`interceptor`](interceptor) | gRPC interceptors: request-context propagation and header forwarding, error logging, panic recovery, request and response bodies in spans, metadata helpers | mctx, zerolog |
| [`mctx`](mctx) | The request context passed between services: the calling user and the request's common parameters | — |
| [`utils`](utils) | Tracing helpers. Deprecated: start spans with `go.opentelemetry.io/otel` directly | — |

Each module's README covers its whole API, its configuration and its
behaviour, including failure cases. The modules form layers; higher layers
depend only on lower ones:

```text
5  app
4  grpc          resource/goroutine
3  interceptor   resource/wpgx   otel   safego
2  zerolog       utils
1  env   health   mctx   metric   resource   resource/cache
   ratelimit   snowflake   pprof
```

The layers are also the release order. A few modules (utils, resource/wpgx)
sit above layer 1 for historical reasons and depend on no gglib module.

## Install

```sh
go get github.com/ggsrc/gglib/app@latest
go get github.com/ggsrc/gglib/resource/wpgx@latest
```

Every module is tagged on its own as `<module directory>/vX.Y.Z`, for example
`grpc/v0.1.0` or `resource/wpgx/v0.1.0`. There is no repository-wide tag.

## Documentation

| Topic | |
|---|---|
| [Lifecycle of a service](docs/lifecycle.md) | Startup, probes at each stage, failure scenarios, shutdown order, deployment notes |
| [Logs, metrics and traces](docs/telemetry.md) | What each signal contains, where it leaves the process, what a log line looks like |
| [Configuration](docs/configuration.md) | Every environment variable a service built with `app` reads |
| [Migrating to v0.1.0](docs/migrating-to-v0.1.0.md) | Behaviour changes from earlier commits, and removed modules |
| [Development and releases](docs/development.md) | Local checks, CI, versioning rules and the release procedure |

API reference: [pkg.go.dev/github.com/ggsrc/gglib](https://pkg.go.dev/github.com/ggsrc/gglib).
Each module's `CHANGELOG.md` lists its changes per release.

## Versioning

All modules are at major version 0. A minor bump (`v0.1.0` → `v0.2.0`) may
contain breaking changes; a patch bump does not. Tags are never moved or
deleted; a bad release is fixed by the next patch release, which `retract`s it.
See [docs/development.md](docs/development.md).

## Security

See [SECURITY.md](SECURITY.md).

## License

[Apache License 2.0](LICENSE)
