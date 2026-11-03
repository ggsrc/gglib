# Changelog

All notable changes to `github.com/ggsrc/gglib/otel` are listed here. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the
module is versioned on its own with tags `otel/vX.Y.Z`.

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

- **Rewritten as the single observability entry point (G5).** One call,
  `otel.Setup(ctx, opts...)`, returns a shutdown function:
  - always installs an SDK tracer provider and the W3C trace-context and
    baggage propagator, so spans have real ids even when nothing is exported;
  - exports over OTLP/HTTP to the endpoint in `OTEL_EXPORTER_OTLP_*`
    (no endpoint, or `OTEL_SDK_DISABLED=true`: nothing is exported);
  - traces and logs (`WithLogs`, G11) on by default; metrics
    (`WithMetrics`, G10) off by default; runtime metrics opt-in;
  - one resource for every service: `service.name`, `service.version`,
    `deployment.environment.name` (plus `deployment.environment` for
    Uptrace), host, and `k8s.pod.name`/`k8s.namespace.name` from
    `POD_NAME`/`POD_NAMESPACE`; `OTEL_RESOURCE_ATTRIBUTES` and options
    override;
  - composable options: sampler (default: SDK, `OTEL_TRACES_SAMPLER`),
    propagator, extra span processors, metric readers and log processors;
  - the shutdown flushes traces, then metrics, then logs, once.
- `otel` is a module again (its `go.mod` had been deleted).
- OpenTelemetry v1.47.

### Added

- `ErrAlreadySetup`, returned by a second `Setup` in the same process.

### Removed

- The previous API (`ConfigureOpenTelemetry`, `Shutdown`, `ForceFlush`,
  `TracerProvider`, `TraceURL`, `ReportError`, `ReportPanic` and the
  `With*` options of the old design). It had no users and several proven
  bugs (an empty endpoint, credentials in logs, overwritten service name).
- No longer depends on `gglib/zerolog`.
