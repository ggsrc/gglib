# chore: upgrade dependencies, fix reachable vulnerabilities, remove unused code

Tracking: alva-ai/mono-meta#1115 (layer L1, gglib). Second PR before `v0.1.0`,
stacked on #28 (release hygiene). Decisions are on the gglib review page
(G1, G2, G8).

## 1. Current state

- `govulncheck` v1.9.0 reported reachable vulnerabilities in 9 of 17
  modules: app 14, otel 12, resource/hatchet 12, grpc 9, zerolog 9,
  interceptor 7, resource/wpgx 3, http 1, resource/cache 1. The sources were
  gRPC v1.78, x/net v0.49, x/text v0.33, OpenTelemetry v1.39 (log SDK v0.15),
  pgx v5.7.6 (through wpgx v0.4.7) and the Hatchet SDK v0.73.12.
- A usage census of 45 repositories (snapshots of `main`, 2026-11-02) found
  modules and packages with no consumer at all, inside or outside gglib:
  `http`, `profiling`, `resource/goroutine`, `interceptor/grpc/ratelimit`,
  `interceptor/http`, and `metric.RecordEvent`. Each carries proven bugs
  (secrets written to spans, a config that never applies, a constructor that
  always panics, an unsynchronised global map).
- `goodns` has one caller, the Sentinel (failover) branch of
  `resource/cache`, which no deployment enables. It resolves the host once,
  ignores `search` domains and the second nameserver, and returns success on
  NXDOMAIN; an empty result then panicked inside go-redis v9.14.
- `resource/hatchet` is about 60 lines of lifecycle glue over the deprecated
  Hatchet v1 SDK, with a 270-module build graph. Its only users are llm-data
  and an alfs example job that never starts; alva-backend and worldline use
  the Hatchet SDK directly.

## 2. Behavior after this change

- Every remaining module (13) passes `govulncheck` with no findings.
- Every direct third-party dependency is at its latest release (see section
  4). The `go` directive stays `go 1.27.0`.
- Removed modules: `goodns`, `http`, `profiling`, `resource/hatchet`.
  Removed packages: `resource/goroutine`, `interceptor/grpc/ratelimit`,
  `interceptor/http`. Removed API: `metric.RecordEvent`, `metric.MetricEvent`.
- `resource/cache` in Sentinel mode passes `host:port` to go-redis instead of
  pre-resolved IPs. The Go dialer tries every address the name resolves to,
  and the name is resolved again on every dial.
- Behavior that consumers see through upgraded dependencies:
  - dcache v0.5.0: `Set` now invalidates the key in other pods' memory caches
    (it never did before); `Invalidate` clears memory even when Redis fails;
    `dcache_latency_milliseconds` records sub-millisecond values, so memory
    hits are no longer 0.
  - wpgx v0.5.0: span attributes follow OTel semconv v1.43
    (`db.system` → `db.system.name`, `db.statement` → `db.query.summary`,
    `db.connection_string` → `wpgx.replica`); query latency includes reading
    rows. Accepted as D12 on the plan page.
  - gRPC v1.85 includes the fixes for GO-2026-4762 (authorization bypass
    through a `:path` without a leading slash), GO-2026-6443 (server panic on
    a missing authority) and GO-2026-6348 (memory exhaustion through
    fragmented DATA frames). Requests that relied on the bypassed behavior no
    longer get through.

## 3. Decisions

- G1 = A (Yumin, 2026-11-03): delete unused code before the first release.
- G2 = B (Yumin, 2026-11-03): delete `resource/hatchet`.
- G8 = B (Yumin, 2026-11-03): tag `v0.1.0` only after every module has full
  tests and its high-severity fixes. Library minimum versions can therefore
  move to the latest releases now; services adopt them when they move to
  `v0.1.0`.
- `app.WithServerName` is kept. The review page listed it as unused, but
  alfs, alva-backend, stream-relay and worldline call it; only its value is
  never read inside `app`. What it should feed is decided with the `app`
  rework.
- The Sentinel branch is kept and fixed rather than deleted. It costs a few
  lines, and Redis high availability (layer L5) may need it. Its other gaps
  (hard-coded master name `master`, `Port` reused as the Sentinel port) are
  left for the `resource/cache` PR.
- Internal requires stay at `b474e43`. The release script rewrites them to
  tagged versions at `v0.1.0`.

## 4. Changes

- Deleted: `goodns/`, `http/`, `profiling/`, `resource/hatchet/`,
  `resource/goroutine/`, `interceptor/grpc/ratelimit/`, `interceptor/http/`.
- `metric/metric.go`: removed `RecordEvent`, `MetricEvent` and the global
  `counters` map.
- `resource/cache/redis.go`: Sentinel addresses are `host:port`; the
  `goodns` import and requirement are gone.
- `otel/logging.go`: `otel.SetLoggerProvider` replaces the deprecated
  `log/global.SetLoggerProvider` (the old function now only forwards to it).
- `go get -u -t ./...` and `go mod tidy` in every module. Direct
  dependencies:

  | Dependency | From | To | Modules |
  |---|---|---|---|
  | google.golang.org/grpc | v1.78.0 | v1.85.0 | app, grpc, interceptor |
  | go.opentelemetry.io/otel, trace, sdk, sdk/metric, OTLP exporters | v1.39.0 | v1.47.0 | otel, interceptor, utils, zerolog |
  | go.opentelemetry.io/otel/sdk/log | v0.15.0 | v1.47.0 | otel |
  | otlplog/otlploghttp | v0.15.0 | v0.23.0 | otel |
  | otelgrpc, contrib runtime | v0.64.0 | v0.72.0 | grpc, otel |
  | github.com/stumble/wpgx (pgx v5.12.0) | v0.4.7 | v0.5.0 | resource/wpgx |
  | github.com/stumble/dcache | v0.4.0 | v0.5.0 | resource/cache |
  | github.com/redis/go-redis/v9, redisotel | v9.14.0 | v9.23.0 | resource/cache |
  | github.com/uptrace/uptrace-go | v1.39.0 | v1.47.0 | app |
  | github.com/prometheus/client_golang | v1.23.2 | v1.25.0 | metric |
  | github.com/rs/zerolog | v1.34.0 | v1.35.1 | 7 modules |
  | github.com/bytedance/sonic | v1.15.0 | v1.15.5 | interceptor, mctx |
  | github.com/getsentry/sentry-go | v0.41.0 | v0.50.0 | interceptor |
  | grpc-ecosystem/go-grpc-middleware/v2 | v2.3.3 | v2.3.4 | grpc |
  | coocood/freecache | v1.2.4 | v1.2.7 | resource/cache |
  | golang.org/x/sync | v0.19.0 | v0.24.0 | resource |
  | stretchr/testify | v1.11.1 | v1.12.1 | interceptor, zerolog |
  | google.golang.org/protobuf | v1.36.11 | v1.36.12 | interceptor |

  Indirect: x/net v0.62.0, x/text v0.43.0. `interceptor` no longer needs
  `envconfig` or `x/time` (they were used only by the deleted packages).
- `README.md`: module table and release order.

## 5. Verification

Toolchain Go 1.27.3 (`GOTOOLCHAIN=local`), `GOWORK=off`, golangci-lint v2.14.0,
govulncheck v1.9.0.

- `make tidy-check build vet` — all 13 modules pass.
- `make test` (`-race -count=1`) — all 13 modules pass.
- `make lint` — 0 issues in all 13 modules (after `make lint-fix` reordered
  one import in `otel/logging.go`).
- `govulncheck ./...` in every module — "No vulnerabilities found" in all 13.
- Consumers of removed code, `command grep` over the 45-repository snapshot
  (2026-11-02): no imports of `gglib/http`, `gglib/profiling`,
  `resource/goroutine`, `interceptor/grpc/ratelimit`, `interceptor/http`,
  `RecordEvent` or `MetricEvent`. `goodns` appears only as an indirect
  requirement in six services' go.mod. `resource/hatchet` is imported by
  `alfs/cmd/cronjob/cronserver/cron.go` and `llm-data/pkg/hatchet/client.go`.
- The Sentinel change is checked by build and vet only. Tests for
  `resource/cache` come with that module's PR.

## 6. Human decisions

None open. The deletions and version policy follow G1, G2 and G8.

## 7. Outcome

- [x] Changes made and verified locally.
- [ ] PR opened, reviewed by Yumin, merged after #28.

## 8. Remaining work

- Consumers that move to gglib `v0.1.0` must replace `resource/hatchet`
  (alfs example job, llm-data) by calling the Hatchet SDK directly. Recorded
  for the Alva side in alva-ai/mono-meta#1116.
- Next PRs: the library golangci-lint configuration and its non-behavioral
  fixes; CI with a per-module matrix and coverage ratchet; then per-module
  tests and fixes.
