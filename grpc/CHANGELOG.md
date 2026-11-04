# Changelog

All notable changes to `github.com/ggsrc/gglib/grpc` are listed here. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the
module is versioned on its own with tags `grpc/vX.Y.Z`.

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

- **A panic in a streaming handler no longer crashes the process.** The
  stream chain had only Prometheus; it is now metrics → recovery → error log.
- A panic in a unary call returns `codes.Internal` (it was `Unknown`) and is
  counted in `grpc_server_handled_total` (it was missing). Metrics are now
  the outermost unary interceptor.
- `WithServerGRPCServerOptions(grpc.StreamInterceptor(…))` or
  `grpc.UnaryInterceptor(…)` no longer panics ("already set"); the standard
  chain is built with `grpc.ChainUnaryInterceptor`/`ChainStreamInterceptor`.
- A `grpc.WithUnaryInterceptor` dial option no longer replaces the whole
  client chain; it runs before it.
- The chain is no longer the hand-written, non-re-entrant copy of
  go-grpc-middleware v1 (data race under concurrent handler calls).
- Call logs use the logger in the request context (with `request_id` and
  `user_id`) and fall back to the global logger; an unknown level logs at
  Error instead of panicking.

### Changed

- **Request and response bodies are recorded in spans only with
  `WithServerDebug(true)`.** Before, they were recorded whenever `ENV`
  was not exactly `prd`, including staging, with passwords and tokens.
  Bodies are cut at 4 KiB.
- The client no longer falls back to a disabled logger when
  `zerolog.DefaultContextLogger` is unset; it uses the global logger, like
  the server.
- No longer depends on `gglib/env`.
- gRPC v1.85, otelgrpc v0.72 and go-grpc-middleware v2.3.4 (fixes
  reachable vulnerabilities, including GO-2026-4762, an authorization bypass
  through a `:path` without a leading slash).
- `Server.Start` listens through `net.ListenConfig`.

### Added

- `DefaultServerPort`.
- `Server.Listen`, so that a caller can report a port conflict before
  serving; `Start` is `Listen` then `Serve`.
- `WithClientMetadataForwarding`, and the request-context interceptor in
  the server stream chain (see the interceptor changelog).

### Removed

- `WithClientPanicHandler`, `WithServerPanicHandler` (no users).
- `InterceptorLogger` is no longer exported.
- `GetCredIDsFromFormula`, `GetDeviceType`, `DeviceType` and its constants,
  `GetIPFromRequest` (trusted `X-Forwarded-For`), `GetNumericHash`,
  `GetMetadata`, `CReg`, `NReg`: application-specific helpers with no users.
- Every exported identifier is documented.
