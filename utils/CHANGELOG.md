# Changelog

All notable changes to `github.com/ggsrc/gglib/utils` are listed here. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the
module is versioned on its own with tags `utils/vX.Y.Z`.

## Unreleased

Changes since `main` at `b474e43` (`v0.0.0-20261013145132-b474e434e730`),
the last commit before the first release work.

### Requirements

- Requires Go 1.27 (`go 1.27.0`).
- `go.mod` is complete on its own; sibling gglib modules are required at
  matching versions.

### Deprecated

- The whole package: start spans with `otel.Tracer("<your package>")`
  directly. alfs's cron example is its only user.

### Fixed

- `StartTrace` keeps the span kind the caller passes (it was overwritten
  with internal) and uses the current global tracer provider (it was bound
  when the package loaded).
- `BackgroundCtx` applies its options (they were ignored).

### Changed

- The instrumentation scope is `github.com/ggsrc/gglib/utils` (it was the
  stale `github.com/ggsrc/app`), and spans no longer carry a
  `service.name` attribute: the resource has it.
- No longer depends on `gglib/env`.
- OpenTelemetry v1.47.
