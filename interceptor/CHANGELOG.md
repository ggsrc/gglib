# Changelog

All notable changes to `github.com/ggsrc/gglib/interceptor` are listed here. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the
module is versioned on its own with tags `interceptor/vX.Y.Z`.

## Unreleased

Changes since `main` at `b474e43` (`v0.0.0-20261013145132-b474e434e730`),
the last commit before the first release work.

### Requirements

- Requires Go 1.27 (`go 1.27.0`).
- `go.mod` is complete on its own; sibling gglib modules are required at
  matching versions.

### Fixed

- `recovery` returns `codes.Internal` with the message "internal error"
  (it returned a plain error, sent as `Unknown`), captures the stack once
  and logs the method. New `recovery.StreamServerInterceptor`.
- `errlog` maps `context.Canceled` and `DeadlineExceeded` to their codes, so
  a client that goes away is logged at Warn, not Error. It logs
  `grpc_code`, and `sub_errors` holds every status detail as protojson with
  its `@type` (it used a legacy type that dropped `ErrorInfo` metadata and
  turned other details into `{}`). New `errlog.StreamServerInterceptor`.
- `log` (request and response bodies in spans) encodes protos with protojson
  and cuts each body at 4 KiB (`MaxBodySize`).

### Removed

- `interceptor/grpc/ratelimit` (every constructor panicked; no users).
- `interceptor/http` (trusted `X-Forwarded-For` blindly; no users).
- `recovery.SentryPanicHandler` and the `PanicHandler` parameter (no users;
  removes sentry-go from the module graph).
- `interceptor/grpc/errors` (legacy error type; no producers).

### Changed

- gRPC v1.85, OpenTelemetry v1.47. sonic, copier and sentry-go are no
  longer dependencies.
- Every exported identifier is documented.
