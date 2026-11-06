# Configuration

Every environment variable a service started with [app](../app) reads. The gRPC
server, Postgres and Redis groups apply when the service uses the
environment-based constructors (`NewServerWithDefaultEnvPrefix`,
`NewWPGXWithDefaultEnvPrefix`, `NewCacheWithDefaultEnvPrefix`, …).

| Variable | Default | Read by | Meaning |
|---|---|---|---|
| `ENV` | empty = `local` | [env](../env) | Environment; `OTEL_DEPLOYMENT_ENVIRONMENT` when empty. Unknown names count as production. |
| `SERVICE_NAME`, `OTEL_SERVICE_NAME` | `UnknownService` | env, otel | Service name. `env.ServiceName()` prefers `SERVICE_NAME`; telemetry's `service.name` prefers `OTEL_SERVICE_NAME`. |
| `SERVICE_VERSION`, `GIT_SHA` | the binary's VCS revision | env, otel | Service version, in that order. |
| `LOG_LEVEL` | `info` | [zerolog](../zerolog) | Log level. The older `LOGGER_LEVEL` is still read. |
| `LOG_FORMAT` | automatic | zerolog | `json` or `console`. Automatic: console in `local`, JSON elsewhere. |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | empty = no export | [otel](../otel) | OTLP/HTTP endpoint; or `…_TRACES_ENDPOINT`, `…_LOGS_ENDPOINT`, `…_METRICS_ENDPOINT` per signal. |
| `OTEL_SDK_DISABLED` | false | otel | `true` exports nothing. |
| `OTEL_TRACES_SAMPLER`, `OTEL_TRACES_SAMPLER_ARG` | parent-based, always on | OpenTelemetry SDK | Sampling. |
| `OTEL_RESOURCE_ATTRIBUTES` | empty | OpenTelemetry SDK | Extra resource attributes. |
| `OTEL_EXPORTER_OTLP_HEADERS` | empty | OpenTelemetry SDK | Headers for the exporter, e.g. authentication. |
| `POD_NAME`, `POD_NAMESPACE` | empty | otel | `k8s.pod.name`, `k8s.namespace.name`. Set them from the Downward API. |
| `METRIC_PORT` | 4014 | [metric](../metric) | Metrics port. |
| `HEALTHCHECK_PORT` | 8080 | [health](../health) | Probe port. |
| `HEALTHCHECK_PROBEINTERVAL`, `HEALTHCHECK_PROBETIMEOUT` | 5s, 5s | health | Check interval and per-round timeout. |
| `HEALTHCHECK_READYCOUNT` | 0 | health | Failed rounds still reported as ready. |
| `HEALTHCHECK_STUCKAFTER` | 3m | health | Failing time after which liveness fails. |
| `HEALTHCHECK_READY`, `HEALTHCHECK_ALIVE` | true, true | health | Initial flags. |
| `GRPC_PORT` | 9090 | [grpc](../grpc) | gRPC port. |
| `GRPC_DEBUG` | false | grpc | Record request and response bodies in spans. |
| `GRPC_VERBOSE` | false | grpc | Log the start and end of every call (server and client). |
| `GGLIB_GRPC_METADATA_FORWARDING` | empty = log only | [interceptor](../interceptor) | `allowlist`: forward only allowlisted headers. |
| `POSTGRES_*` | see [resource/wpgx](../resource/wpgx#configuration-prefix-postgres_-defined-by-wpgx) | resource/wpgx | Postgres pool, including replicas. |
| `REDIS_*`, `DCACHE_*` | see [resource/cache](../resource/cache#configuration) | resource/cache | Redis connection and the in-memory cache. |
| `DEBUG_PPROF_ENABLED`, `DEBUG_PPROF_ADDR`, `DEBUG_PPROF_TOKEN` | false, `:6060`, empty | [pprof](../pprof) | Profiling server on/off, address and optional token. |
| `DEBUG_PPROF_CPU_PROFILE_SECONDS`, `DEBUG_PPROF_CPU_PROFILE_MAX_SECONDS` | 10, 10 | pprof | Default and maximum CPU profile length. |

Durations use Go syntax (`5s`, `3m`, `6h`); booleans accept `true`/`false`.
