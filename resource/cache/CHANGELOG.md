# Changelog

All notable changes to `github.com/ggsrc/gglib/resource/cache` are listed here. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the
module is versioned on its own with tags `resource/cache/vX.Y.Z`.

## Unreleased

Changes since `main` at `b474e43` (`v0.0.0-20261013145132-b474e434e730`),
the last commit before the first release work.

### Requirements

- Requires Go 1.27 (`go 1.27.0`).
- `go.mod` is complete on its own; sibling gglib modules are required at
  matching versions.

### Changed

- dcache v0.5.0. `Set` now invalidates the key in other pods' memory
  caches; `Invalidate` clears memory even when Redis fails;
  `dcache_latency_milliseconds` records sub-millisecond values.
- go-redis v9.23.
- Sentinel (failover) mode passes `host:port` to go-redis instead of
  resolving the host once through the removed `goodns` module.
- Errors are wrapped with `fmt.Errorf` instead of `github.com/pkg/errors`.
- Every exported identifier is documented.
