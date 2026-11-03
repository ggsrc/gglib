# Changelog

All notable changes to `github.com/ggsrc/gglib/otel` are listed here. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the
module is versioned on its own with tags `otel/vX.Y.Z`.

## Unreleased

Changes since `main` at `b474e43` (`v0.0.0-20261013145132-b474e434e730`),
the last commit before the first release work.

### Requirements

- Requires Go 1.27 (`go 1.27.0`).
- `go.mod` is complete on its own; sibling gglib modules are required at
  matching versions.

### Changed

- `otel` is a module again (its `go.mod` had been deleted).
- OpenTelemetry v1.47; the log SDK is stable (v1.47).
- Uses `otel.SetLoggerProvider` instead of the deprecated `log/global`.
