# Changelog

All notable changes to `github.com/ggsrc/gglib/resource/wpgx` are listed here. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the
module is versioned on its own with tags `resource/wpgx/vX.Y.Z`.

## Unreleased

## v0.1.0 - 2026-11-03

First release of the module on its own.

Changes since `main` at `b474e43` (`v0.0.0-20261013145132-b474e434e730`),
the last commit before the first release work.

### Requirements

- Requires Go 1.27 (`go 1.27.0`).
- `go.mod` is complete on its own; sibling gglib modules are required at
  matching versions.

### Fixed

- **A failed `Init` can be retried.** Before, the second call returned nil
  with no pool, and `Start` and `OK` reported success.
- **A failed `Init` closes the pool** instead of leaking its connections,
  goroutines and metrics.
- `Init` no longer modifies the caller's `*wpgx.Config`. (With wpgx v0.4.7
  the config log line overwrote replica passwords with `*hidden*`.)
- `Init` gives up after 30 seconds (`DefaultInitTimeout`) when its context
  has no deadline, instead of hanging on an unreachable host.
- `Stop` returns when its context is done instead of waiting for every
  connection to be released; it is safe to call more than once.
- `OK` and `Start` return an error before `Init`; a second `Init` is an
  error.

### Changed

- `OK` pings only the primary. One replica outage no longer marks every pod
  unready; queries routed to that replica fail and the rest keep working.
- Start-up is logged at Info through the global logger (it used to log at
  Warn through the context logger, which dropped the lines when the context
  had none).
- No longer depends on `gglib/zerolog`.
- wpgx v0.5.0 (pgx v5.12, fixes GO-2026-5004). Span attributes follow OTel
  semantic conventions v1.43 (`db.system.name`, `db.query.summary`,
  `wpgx.replica`), and query latency includes reading the rows.
- Every exported identifier is documented.

### Removed

- The type loader (`InitTypeLoader`, `LoadTypes`, `Reset`, `NewLoader`,
  `Loader`, `TypeLoaderParam`). No service used it; it had a data race,
  skipped replicas and retried failed loads on every acquire. Use wpgx
  `Config.AfterConnect` with pgx `Conn.LoadTypes` instead.
