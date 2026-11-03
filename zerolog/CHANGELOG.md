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
