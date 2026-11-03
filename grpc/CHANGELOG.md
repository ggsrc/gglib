# Changelog

All notable changes to `github.com/ggsrc/gglib/grpc` are listed here. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the
module is versioned on its own with tags `grpc/vX.Y.Z`.

## Unreleased

Changes since `main` at `b474e43` (`v0.0.0-20261013145132-b474e434e730`),
the last commit before the first release work.

### Requirements

- Requires Go 1.27 (`go 1.27.0`).
- `go.mod` is complete on its own; sibling gglib modules are required at
  matching versions.

### Changed

- gRPC v1.85, otelgrpc v0.72 and go-grpc-middleware v2.3.4 (fixes
  reachable vulnerabilities, including GO-2026-4762, an authorization bypass
  through a `:path` without a leading slash).
- `Server.Start` listens through `net.ListenConfig`.
- Every exported identifier is documented.
