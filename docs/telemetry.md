# Logs, metrics and traces

A service built with gglib produces three kinds of telemetry. This page says
what produces each, how it leaves the process, and what it looks like.

| Signal | Produced by | Leaves the process through | Typically read by |
|---|---|---|---|
| Logs | [zerolog](../zerolog); code writes with `log.Ctx(ctx)` | Standard error, one JSON object per line | Your cluster's log collector |
| Log records (copy) | zerolog's OpenTelemetry bridge: the same lines with all their fields | OTLP/HTTP, only when an endpoint is configured | Your OpenTelemetry backend, next to the trace they belong to |
| Metrics | Prometheus default registry: gRPC calls and latencies, wpgx queries, dcache hits, Go runtime | `:4014/metrics` ([metric](../metric)) | Prometheus: dashboards, alerts, autoscaling |
| Traces | gRPC server and client (otelgrpc), wpgx, Redis (redisotel), your own spans | OTLP/HTTP, only when an endpoint is configured ([otel](../otel)) | Your OpenTelemetry backend |
| OpenTelemetry metrics | Off by default | — | Not exported unless `otel.WithMetrics(true)`; this includes the Redis client's OpenTelemetry metrics |

## A log line

```json
{"level":"warn","request_id":"r-7f3a","user_id":"42","grpc_method":"/user.v1.UserService/GetUser",
 "grpc_code":"NotFound","error":"rpc error: code = NotFound desc = user not found",
 "time":"2026-11-03T08:15:02.481Z","caller":"/src/.../errlog.go:68",
 "message":"grpc server error","trace_id":"4bf92f3577b34da6a3ce929d0e0e4736","span_id":"00f067aa0ba902b7"}
```

- `request_id` and `user_id` come from the request context the caller sent;
  the gRPC server interceptor adds them to the request's logger (see
  [interceptor](../interceptor#request-context-and-header-forwarding)).
- `trace_id` and `span_id` come from the current span (see
  [zerolog](../zerolog)). Search your tracing backend for the `trace_id` to see
  the whole call chain.
- `grpc_method`, `grpc_code` and `error` are added by the error logger (see
  [interceptor](../interceptor#error-logging)); the level is `warn` because
  `NotFound` is the caller's problem.

## gRPC metrics

The gRPC server and client interceptors register the standard
go-grpc-prometheus metrics: `grpc_server_started_total`,
`grpc_server_handled_total{grpc_code,…}`, `grpc_server_handling_seconds`
(histogram) and the client equivalents. Calls stopped by panic recovery are
counted with code `Internal`.

## Turning export on

Set `OTEL_EXPORTER_OTLP_ENDPOINT` (or one endpoint per signal) in the
deployment. Without an endpoint nothing is exported; logs still go to standard
error and metrics are still served for Prometheus. See [otel](../otel) for the
resource attributes every record carries and [configuration.md](configuration.md)
for all variables.
