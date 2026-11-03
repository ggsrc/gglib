# Changelog

All notable changes to `github.com/ggsrc/gglib/zerolog` are listed here. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the
module is versioned on its own with tags `zerolog/vX.Y.Z`.

## Unreleased

Changes since `main` at `b474e43` (`v0.0.0-20261013145132-b474e434e730`),
the last commit before the first release work.

### Requirements

- Requires Go 1.27 (`go 1.27.0`).
- `go.mod` is complete on its own; sibling gglib modules are required at
  matching versions.

### Added

- **`Setup(opts...)`: one call configures logging (G5).** It sets the global
  logger and `zerolog.DefaultContextLogger`:
  - one level knob: `WithLevel`, else `LOG_LEVEL`, else the deprecated
    `LOGGER_LEVEL`, else info; applied as the global level;
  - JSON in clusters, readable console output locally (`FormatAuto`);
    `LOG_FORMAT=json|console` or `WithFormat` override; color only locally;
  - `trace_id` and `span_id` on every event whose context holds a span;
    caller field; millisecond timestamps; stack traces for errors;
  - `WithOTLP(true)` also sends each record (level, message, trace context)
    to the global OpenTelemetry logger provider, through the official API.
    Off by default (`DefaultOTLP`, decision G11).
  Defaults follow the G9 and G11 recommendations; if decided otherwise,
  only `FormatAuto` and `DefaultOTLP` change.

### Removed

- The agoda OTLP bridge (`WithOTLP()` without arguments, `WithBatchSize`,
  `LoggerOption`). It read zerolog's private buffer by reflection (about 46
  times slower per event) and pulled gRPC and protobuf into the module.
  `WithOTLP(true)` replaces it.

### Deprecated

- `InitLogger`: kept with its previous behavior for gglib/app (sets the
  level and the default context logger, leaves the service's own logger
  alone); use `Setup`.

### Fixed

- **`log.Ctx` no longer drops events silently.** With no logger in the
  context and `zerolog.DefaultContextLogger` unset, it returned a disabled
  logger, so every `log.Ctx(ctx).Error()…` was thrown away; this is what
  happened in alva-gateway (318 call sites) and trex, and in gglib's own
  panic and error logs before `zerolog.InitLogger` ran. It now falls back to
  the global logger.
- `log.Ctx` binds `ctx` to the returned logger, so hooks (for example the
  OTLP bridge) can read the span from the event.

### Changed

- `zerolog/log` tests no longer need Uptrace variables.
- Every exported identifier is documented.
