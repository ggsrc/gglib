# app

```sh
go get github.com/ggsrc/gglib/app
```

`app` runs a service process from start to finish: it sets up logging and
telemetry, keeps the Kubernetes probes truthful while dependencies come and
go, serves gRPC, and shuts everything down in the right order. A service's
`main` calls three methods: `New`, `Init` and `Run`.

Depends on [env](../env), [grpc](../grpc), [health](../health),
[metric](../metric), [otel](../otel), [resource](../resource) and
[zerolog](../zerolog). For a walk-through of a whole service, see
[docs/lifecycle.md](../docs/lifecycle.md).

## Usage

```go
a, err := app.New(ctx, app.WithServerName("my-service"))
if err != nil {
	return err // a port is taken, or telemetry is misconfigured
}

db := wpgx.NewWPGXWithDefaultEnvPrefix()
rm := resource.NewResourceManager([]resource.Resource{db})
if err := a.Init(ctx, rm); err != nil { // retries until the database is up
	return err
}

srv := grpc.NewServerWithDefaultEnvPrefix()
pb.RegisterMyServiceServer(srv.Server(), handler)

return a.Run(ctx, srv, rm) // blocks until SIGTERM/SIGINT, then shuts down
```

A background worker without a gRPC API passes `nil`:
`a.Run(ctx, nil, rm)`. It still gets the metrics server, the health server,
signal handling and the ordered shutdown.

## API

| Name | What it does |
|---|---|
| `New(ctx, opts...) (*App, error)` | Sets up logging and telemetry, starts the metrics server and the health server. When it returns, the process is alive but not ready. |
| `(*App) Init(ctx, rm) error` | Initializes a set of resources, retrying with backoff until it succeeds or `ctx` ends. |
| `(*App) Run(ctx, srv, rms...) error` | Starts the resources and the gRPC server, reports ready, blocks until it is time to stop, then calls `Stop`. `srv` may be `nil`. |
| `(*App) Stop(ctx) error` | Ordered shutdown. `Run` calls it; only the first call has an effect, later calls return the same result. |
| `DefaultResourceShutDownTimeout` | Default shutdown timeout, 30 s. A variable, read by `New`. |

### Options

| Option | Effect |
|---|---|
| `WithServerName(name)` | Service name for telemetry, used only when neither `SERVICE_NAME` nor `OTEL_SERVICE_NAME` is set. Deployment configuration wins, so service names in your tracing backend do not change because of code. |
| `WithServerVersion(v)` | Service version for telemetry, used only when neither `SERVICE_VERSION`, `GIT_SHA` nor the binary's VCS revision is available. |
| `WithDebug(bool)` | `true` sets the log level to debug. It does not record request bodies; that is the gRPC server's `WithServerDebug`. |
| `WithTelemetry(otel.Option...)` | Passed to [`otel.Setup`](../otel), for example `otel.WithRuntimeMetrics(true)`. |
| `WithLogging(zerolog.Option...)` | Passed to [`zerolog.Setup`](../zerolog), for example `zerolog.WithOTLP(false)`. |
| `WithShutdownTimeout(d)` | Bounds the gRPC drain and the resource shutdown together. Default `DefaultResourceShutDownTimeout`. |

## Behaviour

### New

1. Calls `zerolog.Setup` with the `WithLogging` options (and debug level if
   `WithDebug(true)`).
2. Calls `otel.Setup` with the `WithTelemetry` options. If something in the
   process already called it (`otel.ErrAlreadySetup`), `New` logs a warning
   and leaves telemetry, including its final flush, to that caller. Any other
   error is returned.
3. Starts the [metrics server](../metric) from `METRIC_*` variables (port 4014
   by default). A port that cannot be bound is returned as an error.
4. Starts the [health server](../health) from `HEALTHCHECK_*` variables (port
   8080 by default), with the metrics server as its first check. It is marked
   not ready before the port is bound; a port that cannot be bound is returned
   as an error.
5. Logs `app: starting` with the environment and service name.

### Init

- Calls the resource manager's `Init`. On failure it logs
  `app: resources not initialized; retrying` (with the attempt number and the
  delay) and tries again after 1 s, 2 s, 4 s, … up to 30 s between attempts.
- Resources that already initialized are not initialized again; the next
  attempt resumes at the one that failed (see [resource](../resource)). Each
  resource's `Init` must therefore be safe to retry; the gglib resources are.
- Returns an error only when `ctx` ends; the error includes the last failure.
- `Init` does not add health checks, and `Run` does not initialize. You can
  initialize one set of resources and run another, for example initialize the
  database first and pass a manager that also holds background workers to
  `Run`.

### Run

1. Starts listening for SIGINT and SIGTERM.
2. Starts each resource manager in order and adds its checks to the health
   server. If one fails to start, everything is stopped and the error is
   returned.
3. If a gRPC server was given, binds its port (an error stops everything and
   is returned) and serves it in the background.
4. Marks the process ready and logs `app: ready`.
5. Waits for the first of: a signal (`app: shutting down`), `ctx` ending, the
   gRPC server stopping (`app: grpc server: …`), or the health server failing
   (`app: health server: …`).
6. Calls `Stop`. The result is the reason for stopping plus any shutdown
   errors; a normal stop on a signal returns an error only if shutdown failed.

The context used for shutdown is detached from the `ctx` passed to `Run`, so
cancelling that `ctx` does not cut the shutdown short.

### Stop

1. Readiness fails, so Kubernetes stops sending new requests.
2. The gRPC server stops gracefully: no new connections or calls, in-flight
   calls finish. When the shutdown timeout runs out, remaining calls are cut.
3. Resources stop in reverse registration order: what depends on the database
   stops before the database pool closes, so no handler or worker uses a
   closed connection.
4. The metrics and health servers stop.
5. Buffered logs and spans are flushed, with their own 5 s timeout, so the
   flush happens even if the steps above used the whole shutdown timeout, and
   the shutdown's own log lines are exported.
6. Logs `app: stopped`, or `app: stopped with errors`.

Steps 1–4 share the shutdown timeout (`WithShutdownTimeout`, default 30 s).
Liveness keeps passing during the whole shutdown, so a long drain is not
mistaken for a hung process.

## Things to know

- **Registration order is shutdown order, reversed.** Register what others
  depend on (the database) first and its users (workers that write to it)
  after them.
- **Set the pod's termination grace period above the shutdown timeout plus
  5 s.** Kubernetes kills the process `terminationGracePeriodSeconds` (default
  30 s) after SIGTERM; with the default 30 s shutdown timeout the last log
  lines and spans may otherwise be lost.
- `WithDebug` only changes the log level.

## See also

- [resource](../resource) — the `Resource` and `ResourceManager` interfaces
  `Init` and `Run` take.
- [health](../health) — how the probes answer.
- [docs/lifecycle.md](../docs/lifecycle.md) — the same flow from the
  point of view of Kubernetes, with failure scenarios.
