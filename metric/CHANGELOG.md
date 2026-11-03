# Changelog

All notable changes to `github.com/ggsrc/gglib/metric` are listed here. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the
module is versioned on its own with tags `metric/vX.Y.Z`.

## Unreleased

Changes since `main` at `b474e43` (`v0.0.0-20261013145132-b474e434e730`),
the last commit before the first release work.

### Requirements

- Requires Go 1.27 (`go 1.27.0`).
- `go.mod` is complete on its own; sibling gglib modules are required at
  matching versions.

### Removed

- `RecordEvent` and `MetricEvent`. `RecordEvent` wrote a global map
  without a lock and could crash the process; it had no users.

### Fixed

- **`Start` returns the error when the port cannot be bound.** Before, it
  returned nil, the server never listened, and `OK` reported the error only
  on its first call.
- `Stop` before `Start`, or after a failed `Start`, returns nil. Before, it
  panicked, which crashed `app` when a resource failed to start.
- `NewWithOptions` without `WithPort` listens on 4014, like the other
  constructors. Before, it used port 0 (a random port).
- `OK` reports a serving error on every call, not only the first.

### Changed

- A second `Start` returns an error.
- prometheus/client_golang v1.25.
- Every exported identifier is documented.

### Added

- `DefaultPort` and `Server.Addr`.
