# metric

```sh
go get github.com/ggsrc/gglib/metric
```

An HTTP server that exposes every metric in the Prometheus default registry on
`/metrics`. The gRPC metrics from [grpc](../grpc), the query metrics of
[resource/wpgx](../resource/wpgx) and the cache metrics of
[resource/cache](../resource/cache) are all registered there, so they all
leave the process through this server. [app](../app) starts it in `app.New`.

## Usage

```go
srv := metric.NewWithDefaultEnvPrefix() // METRIC_PORT, default 4014
if err := srv.Start(ctx); err != nil {
	return err // the port is taken
}
defer srv.Stop(context.Background())
```

## API

| Name | What it does |
|---|---|
| `NewWithDefaultEnvPrefix()` | Configured from `METRIC_PORT` (default `DefaultPort`, 4014). This is what `app` uses. |
| `New(prefix)` | Configured from `<prefix>_PORT`. Panics if the variable cannot be parsed. |
| `NewWithOptions(WithPort(p))` | Configured in code; default port 4014. |
| `(*Server) Start(ctx) error` | Binds the port synchronously (failure: `metric: listen: …`), then serves in the background. A second call returns an error. Port 0 picks a free port. |
| `(*Server) Addr() net.Addr` | The address actually listened on; `nil` before `Start`. |
| `(*Server) Stop(ctx) error` | Graceful shutdown; nothing to do if never started. |
| `(*Server) OK(ctx) error` | The error the server stopped with, if any. `app` uses it as a health check. |

## Things to know

- The endpoint has no authentication. Expose the port only inside the cluster.
- The server waits at most 5 s for request headers, so slow clients cannot hold
  it.
- Register your own metrics with `prometheus.MustRegister` (or `promauto`) on
  the default registry and they are served here.

## See also

- [health](../health) — the other HTTP server `app` runs.
- [docs/telemetry.md](../docs/telemetry.md) — which metrics exist.
