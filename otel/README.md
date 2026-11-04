# otel

```sh
go get github.com/ggsrc/gglib/otel
```

One call sets up OpenTelemetry for the process: where traces, logs and
(optionally) metrics are exported, and the resource attributes — service name,
version, environment, pod — that every record carries. [app](../app) calls it
in `app.New`.

Depends on [env](../env). Exports over OTLP/HTTP with the OpenTelemetry Go SDK.

## Usage

```go
shutdown, err := otel.Setup(ctx, otel.WithServiceName("my-worker"))
if err != nil {
	return err
}
defer shutdown(context.Background()) // flush buffered data before exiting
```

- Call it **before** creating gRPC servers and clients and database pools:
  they capture the global tracer provider when they are created.
  `app.New` already does this in the right order.
- Call it once per process. A second call returns `ErrAlreadySetup`.

## What is exported by default

| Signal | Default | Why |
|---|---|---|
| Traces | on | A tracer provider is always installed, so spans have real trace IDs (which logs pick up) even when nothing is exported. |
| Logs | on (`DefaultLogs = true`) | A trace in your tracing backend shows the log records written while it ran. |
| Metrics | off (`DefaultMetrics = false`) | Metrics are served for Prometheus by [metric](../metric). Turn OpenTelemetry metrics on with `WithMetrics(true)` if your backend reads them instead. |

"On" means data is handed to an exporter. Data actually leaves the process only
when an OTLP endpoint is configured: `OTEL_EXPORTER_OTLP_ENDPOINT`, or
`OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`, `…_LOGS_ENDPOINT`,
`…_METRICS_ENDPOINT` per signal. A signal without an endpoint is not exported.
`OTEL_SDK_DISABLED=true` exports nothing. Other standard variables (for example
`OTEL_EXPORTER_OTLP_HEADERS` for authentication) are read by the SDK.

## Resource attributes

Later rows override earlier ones:

| Source | Attributes |
|---|---|
| SDK defaults | `service.name=unknown_service:<binary>`, SDK name and version |
| Environment, through [env](../env) | `service.name` (`SERVICE_NAME` or `OTEL_SERVICE_NAME`), `service.version` (see env), `deployment.environment.name` and the older `deployment.environment` (both `env.Env()`; some backends still group by the older name), `k8s.pod.name` (`POD_NAME`), `k8s.namespace.name` (`POD_NAMESPACE`) |
| Host and standard variables | `host.name`; attributes from `OTEL_SERVICE_NAME` and `OTEL_RESOURCE_ATTRIBUTES` |
| `Setup` options | `WithServiceName`, `WithServiceVersion`, `WithEnvironment`, `WithAttributes` |

So when both `SERVICE_NAME` and `OTEL_SERVICE_NAME` are set,
`env.ServiceName()` returns the first, while `service.name` in telemetry is the
second.

## Options

| Option | Effect |
|---|---|
| `WithTraces(bool)`, `WithLogs(bool)`, `WithMetrics(bool)` | Turn export of each signal on or off. |
| `WithRuntimeMetrics(bool)` | Record Go runtime metrics (GC, goroutines, …). Only when a meter provider is installed. |
| `WithSampler(s)` | Trace sampler. Without it the SDK reads `OTEL_TRACES_SAMPLER` and `OTEL_TRACES_SAMPLER_ARG`; the default is parent-based, always sampling root spans. |
| `WithPropagator(p)` | Cross-service propagation format. Default: W3C trace context and baggage. |
| `WithSpanProcessor(p)` | An extra span processor, for example to collect spans in tests. |
| `WithMetricReader(r)`, `WithLogProcessor(p)` | An extra metric reader or log processor. Giving one installs that provider even when its export is off. |
| `WithServiceName`, `WithServiceVersion`, `WithEnvironment`, `WithAttributes` | Override resource attributes (table above). |

## Shutdown

The function `Setup` returns works once; later calls do nothing. It shuts down
traces, then metrics, then logs, so log records written while traces shut down
are still exported. If `Setup` fails halfway, the parts it already installed
are shut down in reverse order.

## See also

- [zerolog](../zerolog) — the log records exported here.
- [docs/telemetry.md](../docs/telemetry.md) — every signal and where it goes.
