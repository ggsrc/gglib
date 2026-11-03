# Changelog

All notable changes to `github.com/ggsrc/gglib/app` are listed here. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the
module is versioned on its own with tags `app/vX.Y.Z`.

## Unreleased

Changes since `main` at `b474e43` (`v0.0.0-20261013145132-b474e434e730`),
the last commit before the first release work.

### Requirements

- Requires Go 1.27 (`go 1.27.0`).
- `go.mod` is complete on its own; sibling gglib modules are required at
  matching versions.

### Changed

- gRPC v1.85 and uptrace-go v1.47 (fixes reachable vulnerabilities in
  gRPC, x/net and OpenTelemetry).
- Every exported identifier is documented.
