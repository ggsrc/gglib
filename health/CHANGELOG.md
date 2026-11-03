# Changelog

All notable changes to `github.com/ggsrc/gglib/health` are listed here. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the
module is versioned on its own with tags `health/vX.Y.Z`.

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

- **Liveness no longer follows the dependency checks (G4).** A database or
  Redis outage makes readiness fail (no traffic) and readiness recovers by
  itself when the checks pass again; it no longer restarts every pod at
  once. As a safety net (G13), liveness fails when the
  checks have kept failing for longer than `StuckAfter` (default 3 minutes:
  the dependency is back but this instance did not recover) or when the
  check loop stops running.
- **Every check has a timeout** (`ProbeTimeout`); a check that never returns
  counts as failed and is not started again while it runs. Before, a stuck
  check left the status at "healthy" forever.
- Readiness fails until the first round of checks has finished.
- `Stop` fails readiness only; liveness keeps passing so that a draining pod
  is not restarted.
- Failing probes return 503 instead of 400.
- The probe handlers are registered on a router passed to `New` (before,
  they were only registered on the router `Start` created).
- State is synchronized (data races between the check loop and the
  handlers are gone); failures are logged when a failing streak starts and
  when it ends.
- `GoCheck` returns when its context ends.
- Every exported identifier is documented.

### Added

- `Config.StuckAfter`, `NewWithConfig`, `Server.Close`.
- `Server.Listen`, `Server.Serve` and `Server.Shutdown`, so that a caller
  can report a port conflict before serving and stop the server; `Start` is
  `Listen` then `Serve`. `Shutdown` waits for the round of checks in
  progress, until its context is done.

### Removed

- `Config.LiveCount`: liveness no longer counts failed rounds. No deployment
  set `HEALTHCHECK_LIVECOUNT`.
