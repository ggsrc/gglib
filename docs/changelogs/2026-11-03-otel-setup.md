# feat(otel): one Setup call for traces, metrics and logs (G5)

Tracking: alva-ai/mono-meta#1115. Stacked on #32 (env reads the environment
on every call, which the resource relies on). Design and Alva usage survey:
"G5 方案" on the gglib status page; report in
`/home/forge/refactor-handoff/2026-11-02/g5/`.

## 1. Current state

Five different setups across services (uptrace via gglib app, gopkg/otel,
a hand-written worker setup, nothing at all). Inconsistent resource
(environment as `deployment.environment` or `env`; version `1.0.0`, GIT_SHA
or empty), almost no flush on exit. `gglib/otel` was an unused fork with
proven config bugs.

## 2. Behavior after this change

`otel.Setup(ctx, opts...) (ShutdownFunc, error)`; see the module changelog
for the full list. Highlights: always an SDK tracer provider (trace ids for
logs); OTLP/HTTP from standard env; traces on, metrics and logs off by
default (G10, G11 recommendations; `DefaultMetrics`/`DefaultLogs`); one
resource contract; composable options; ordered, idempotent shutdown;
Prometheus untouched.

No consumer uses `gglib/otel` today; services adopt it through `gglib/app`
(next PR) or by calling `Setup` in `main`.

## 3. Decisions

- G5 = A. G10 and G11 are still open; if Yumin chooses differently, only
  `DefaultMetrics`/`DefaultLogs` change.
- No Uptrace DSN path: every deployment already points
  `OTEL_EXPORTER_OTLP_ENDPOINT` at the in-cluster collector.
- `resource.Default()` is not used: it caches the environment of its first
  call.

## 4. Changes

- `otel/{otel,options,resource}.go` replace the old files.
- `otel/otel_test.go`: fake OTLP/HTTP receiver; 90.0% coverage.
- `.golangci.yml`: the temporary otel exclusion is removed (utils remains).
- `scripts/replaces.sh`: same graph-based version as #36.

## 5. Verification

- Tests check what reaches the receiver: traces with the standard resource
  (`service.name`, `service.version`, both environment keys, k8s, host);
  metrics and logs only when on; nothing with `OTEL_SDK_DISABLED`; a
  per-signal endpoint; ids without export; sampler, propagator, readers and
  processors; second `Setup` fails; resource precedence (SERVICE_NAME <
  OTEL_SERVICE_NAME < OTEL_RESOURCE_ATTRIBUTES < options).
- `make tidy-check build vet test lint cover` — all modules pass.

## 7. Outcome

- [x] Implemented and verified locally.
- [ ] CI green; reviewed by Yumin.
