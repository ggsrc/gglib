# Changelog

All notable changes to `github.com/ggsrc/gglib/app` are listed here. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the
module is versioned on its own with tags `app/vX.Y.Z`.

## Unreleased

## v0.1.0 - 2026-11-03

First release of the module on its own.

Changes since `main` at `b474e43` (`v0.0.0-20261013145132-b474e434e730`),
the last commit before the first release work.

### Requirements

- Requires Go 1.27 (`go 1.27.0`).
- `go.mod` is complete on its own; sibling gglib modules are required at
  matching versions.

### Changed

- **New lifecycle: `New`, `Init`, `Run`** (replaces `NewApp` and `Start`).
  - `New(ctx, opts...)` sets up logging (`zerolog.Setup`: JSON in clusters,
    G9; OTLP logs, G11) and telemetry (`otel.Setup`, G5), and starts the
    metrics and health servers. From then on the pod is alive and not
    ready.
  - `Init(ctx, rm)` initializes the resources and, while one fails (for
    example the database is down), retries with a back-off from 1 s to
    30 s instead of exiting. Before, a dependency that was down at start-up
    made the process exit, and Kubernetes restarted it with a back-off of up
    to 5 minutes, so it could stay down minutes after the dependency came
    back (G13 follow-up).
  - `Run(ctx, srv, rms...)` starts the resources, adds their checks to the
    probes, serves gRPC when `srv` is not nil (worker processes pass nil),
    marks the pod ready, and blocks until SIGINT/SIGTERM, `ctx` is done, or
    a server fails.
- **The gRPC server's failure stops the process.** Its error used to be
  dropped: a port conflict left a process that served nothing but looked
  healthy.
- **Readiness waits for the start.** The pod becomes ready only after the
  resources and the gRPC server have started; before, it could be ready
  before the gRPC port was bound.
- **Stop order:** readiness fails, RPCs drain, the resources stop (last
  registered first, G12), the metrics and health servers stop, and
  telemetry is flushed last, so the shutdown's own logs and spans are sent.
  Liveness keeps passing during a long drain.
- Telemetry options are fallbacks: `WithServerName` and `WithServerVersion`
  apply only when the deployment sets no `SERVICE_NAME`/`OTEL_SERVICE_NAME`
  or `SERVICE_VERSION`/`GIT_SHA`, so service names in Uptrace do not change.
  If `otel.Setup` was called before `New`, telemetry is left to the caller.
- gRPC v1.85 (fixes reachable vulnerabilities in gRPC and x/net).
- Every exported identifier is documented.

### Added

- `WithServerVersion`, `WithTelemetry` (options for `otel.Setup`),
  `WithLogging` (options for `zerolog.Setup`), `WithShutdownTimeout`.

### Removed

- `NewApp`, `Start`, `WithGRPCServer`, `WithResourceManager`: use `New`,
  `Init` and `Run`.
- `WithOTEL` and the Uptrace dependency: telemetry goes to the endpoint in
  `OTEL_EXPORTER_OTLP_ENDPOINT` (every deployment points it at the
  in-cluster collector). The DSN and batch size are not used.
- `WithEnv`: the environment comes from `ENV` (gglib/env), which every
  deployment sets.
- The exported `Options` struct; options are set through the `With…`
  functions.
