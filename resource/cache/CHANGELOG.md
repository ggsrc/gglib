# Changelog

All notable changes to `github.com/ggsrc/gglib/resource/cache` are listed here. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the
module is versioned on its own with tags `resource/cache/vX.Y.Z`.

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

- **`Init` returns errors instead of exiting the process.** An unreachable
  Redis, a bad `TLSCACert` or a failed OpenTelemetry instrumentation used to
  call `log.Fatal`; a DNS failure in Sentinel mode used to panic. Nothing is
  left open when `Init` fails.
- **Removed the ElastiCache mode (`IsElastiCache`), which turned off TLS
  certificate verification.** Use `TLSEnabled` with `TLSServerName` set to
  the real endpoint to verify a certificate behind a CNAME. No deployment set
  the flag.
- `Stop` is safe to call more than once (it used to return
  `redis: client is closed` on the second call, which is the normal path
  when a resource fails to start).
- `OK` and `Start` return an error before `Init`; a second `Init` returns an
  error instead of leaking the first client.
- `Init` uses the caller's context, bounded by a 10-second ping timeout.
- Cluster and Sentinel modes set together are rejected.

### Added

- `Redis`, a resource with only the Redis client (`NewRedis`,
  `NewRedisWithDefaultEnvPrefix`, `NewRedisWithConfig`, `Client`). Services
  that only use the client (alva-gateway, stream-relay) no longer need an
  in-memory cache and a pub/sub subscription.
- `RedisConfig.MasterName` (default `master`, as before) and
  `RedisConfig.TLSServerName`.

### Removed

- `RedisConfig.IsElastiCache` (see above) and `DCacheConfig.ReadInterval`,
  which had no effect: it only applies to dcache's Redis single-flight, which
  is off.

### Changed

- The configuration is logged at Info instead of Warn, with `TLSCACert`
  masked like the password.
- dcache v0.5.0. `Set` now invalidates the key in other pods' memory
  caches; `Invalidate` clears memory even when Redis fails;
  `dcache_latency_milliseconds` records sub-millisecond values.
- go-redis v9.23.
- Sentinel (failover) mode passes `host:port` to go-redis instead of
  resolving the host once through the removed `goodns` module.
- Errors are wrapped with `fmt.Errorf` instead of `github.com/pkg/errors`.
- Every exported identifier is documented.
