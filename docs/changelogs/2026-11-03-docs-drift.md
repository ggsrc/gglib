# docs: correct the README module table and the otel package comment

Tracking: alva-ai/mono-meta#1115. Found while writing the gglib reference
pages from the v0.1.0 code.

## 1. Current state

- README module table:
  - `env` lists a host name and build information; the module has no host
    name function, and build information only serves as the fallback
    service version.
  - `mctx` lists a location; `LocationInfo` was removed before v0.1.0.
  - `grpc` lists `env` as a dependency; it depends only on `interceptor`.
  - `resource/wpgx` lists `zerolog`; it depends on no gglib module.
  - `utils` lists `env` and does not say it is deprecated.
  - `interceptor` lists "logging"; the module records request and response
    bodies in spans, it does not log calls (the `grpc` module does).
- The `otel` package comment says logs are exported only when turned on;
  `DefaultLogs` is `true` since decision G11.

## 2. Behavior after this change

No code change. The README table and the `otel` package comment match the
code.

## 3. Changes

- `README.md`: module table rows for env, grpc, interceptor, mctx,
  resource/wpgx, utils.
- `otel/otel.go`: package comment.

## 4. Verification

- Dependencies checked against each module's `go.mod` and imports.
- `gofmt -l otel` reports nothing; CI runs the full checks.

## 5. Outcome

- [x] CI green (run 41729749574, `ci-ok` passed); merged.
