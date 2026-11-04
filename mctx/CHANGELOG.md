# Changelog

All notable changes to `github.com/ggsrc/gglib/mctx` are listed here. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the
module is versioned on its own with tags `mctx/vX.Y.Z`.

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

- **Numbers in `Extras` keep their exact value**: `StringToAppCtx` decodes
  them as `json.Number` instead of `float64`. Before, IDs above 2^53 changed
  value after one hop (1234567890123456789 became 1234567890123456768).
  Readers that type-switch on `float64` must handle `json.Number`.
- `StringToAppCtx` rejects input larger than `MaxAppCtxSize` (32 KiB) with
  `ErrAppCtxTooLarge`. Before, any size was parsed (a 10.7 MB header took
  about a second and over 80 MB of memory).
- sonic v1.15.5, which runs natively on Go 1.27 (v1.14 did not build on Go
  1.26 or later).
- `AppContext` documents that its fields come from the caller and are not
  verified.
- Every exported identifier is documented.

### Added

- `MaxAppCtxSize`, `ErrAppCtxTooLarge`.

### Removed

- `User.Address`, `User.SignAt`, `User.Signature`, `User.AccessToken`,
  `User.JwtString` and `LocationInfo`, left over from an earlier login scheme. No
  service uses them, and an access token does not belong in a header that is
  forwarded to every downstream service. Senders that still include the
  fields are accepted; the fields are dropped.
