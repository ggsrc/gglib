# Changelog

All notable changes to `github.com/ggsrc/gglib/resource/wpgx` are listed here. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the
module is versioned on its own with tags `resource/wpgx/vX.Y.Z`.

## Unreleased

Changes since `main` at `b474e43` (`v0.0.0-20261013145132-b474e434e730`),
the last commit before the first release work.

### Requirements

- Requires Go 1.27 (`go 1.27.0`).
- `go.mod` is complete on its own; sibling gglib modules are required at
  matching versions.

### Changed

- wpgx v0.5.0 (pgx v5.12, fixes GO-2026-5004). Span attributes follow OTel
  semantic conventions v1.43 (`db.system.name`, `db.query.summary`,
  `wpgx.replica`), and query latency includes reading the rows.
- Every exported identifier is documented.
