# interceptor

```sh
go get github.com/ggsrc/gglib/interceptor
```

The gRPC interceptors [grpc](../grpc) installs, one package each. Services
usually get them through `grpc` and do not use them directly.

Depends on [mctx](../mctx) and [zerolog](../zerolog).

| Package | What it does |
|---|---|
| [`interceptor/grpc/context`](#request-context-and-header-forwarding) | Reads the request context on the server; forwards it and other headers on the client. |
| [`interceptor/grpc/errlog`](#error-logging) | Logs failed calls, at a level that depends on the status code. |
| [`interceptor/grpc/recovery`](#panic-recovery) | Turns a panic in a handler into an `Internal` error. |
| [`interceptor/grpc/log`](#request-and-response-bodies-in-spans) | Records request and response bodies in spans (debug only). |
| [`interceptor/grpc/metautils`](#metautils) | Helpers to read and write gRPC metadata. |
| [`interceptor/metadata`](#metadata-keys) | Header names. |

## Request context and header forwarding

`interceptor/grpc/context`. When services call each other, each hop needs to
know who the request is for. The service at the edge (the one that
authenticates the user) encodes the user and the request's common parameters
as JSON — an [`mctx.AppContext`](../mctx) — in the gRPC header `app-ctx-bin`.
These interceptors read it at every hop and pass it on to the next.

### Server: `ContextUnaryServerInterceptor()`, `ContextStreamServerInterceptor()`

- Parses `app-ctx-bin` into an `mctx.AppContext` and stores it in the context;
  handlers read it with `mctx.AppCtxFromContext(ctx)`.
- Stores a logger with `request_id` and `user_id` in the context, so every
  `log.Ctx(ctx)` line carries them.
- A header that does not parse (malformed, or larger than 32 KiB) is logged as
  a warning with its length but not its content. The call goes on without an
  `AppContext`.
- Stores the value of the `metadata` header in the context under
  `metadata.ContextKeyMetadata` (possibly an empty string).

### Client: `ContextUnaryClientInterceptor(mode...)`

Copies the headers of the request currently being served onto the outgoing
call:

- A header the code sets on the outgoing context **replaces** the incoming
  value of the same name; values are not appended.
- Transport headers are never forwarded: names starting with `:` or `grpc-`,
  `content-type`, `user-agent`, `te`, `host`.
- These are always forwarded: `app-ctx-bin`, `x-request-id`, and the tracing
  headers `traceparent`, `tracestate`, `b3`, `x-b3-traceid`, `x-b3-spanid`,
  `x-b3-parentspanid`, `x-b3-sampled`, `x-b3-flags` (for callees and proxies
  that do not use OpenTelemetry).
- Every other header (tokens, cookies, routing hints, …) depends on the
  forwarding mode:

| Mode | Selected by | Other headers |
|---|---|---|
| `ForwardLogOnly` (default) | no setting | Forwarded as before. Each header name is logged once per process at info level (the name only, never the value), naming the headers allowlist mode would drop. |
| `ForwardAllowlist` | `GGLIB_GRPC_METADATA_FORWARDING=allowlist` (`ForwardModeEnv`), or the client option `grpc.WithClientMetadataForwarding` | Dropped. |

Start in the default mode, check the logs for headers your callees rely on,
then switch to the allowlist.

**Security boundary.** `app-ctx-bin` is not signed. These interceptors assume
the services in a cluster trust each other: anyone who can reach a service's
gRPC port can put any user ID in this header. The user, IP and time in an
`AppContext` are whatever the caller wrote; do not trust them unless the
service authenticated the caller itself.

## Error logging

`interceptor/grpc/errlog`: `UnaryServerInterceptor()`,
`StreamServerInterceptor()`, `UnaryClientInterceptor()`. A failed call is logged
— `grpc server error` or `grpc client error` — at a level that separates the
caller's mistakes from the service's own failures:

| Status code | Level |
|---|---|
| `Canceled`, `InvalidArgument`, `NotFound`, `AlreadyExists`, `PermissionDenied`, `FailedPrecondition`, `Aborted`, `OutOfRange`, `Unauthenticated` | warn |
| `Unknown`, `DeadlineExceeded`, `ResourceExhausted`, `Unimplemented`, `Internal`, `Unavailable`, `DataLoss` | error |

Fields: `grpc_method`, `grpc_code`, `error`, and `sub_errors` — the status
details as a JSON array, each with its `@type` — when there are details. An
error that is not a gRPC status counts as `Canceled` or `DeadlineExceeded` for
`context.Canceled` and deadline errors, and as `Unknown` otherwise.

## Panic recovery

`interceptor/grpc/recovery`: `UnaryServerInterceptor()`,
`StreamServerInterceptor()`, `UnaryClientInterceptor()`. A panic is caught,
logged at error level (`grpc server panic` or `grpc client panic`, with the
stack in `panic.stack`),
and returned to the caller as `Internal` with the message `internal error`.
The panic value and stack go only to the log, never to the caller.

## Request and response bodies in spans

`interceptor/grpc/log`: `LogUnaryServerInterceptor()`. Encodes the request and
the response as JSON and records them on the current span as `grpc.request` and
`grpc.response`. Each is cut at `MaxBodySize` (4 KiB) and marked
`…(truncated)`. Bodies are encoded only when the span is recording. The
[grpc](../grpc) server installs it only with `WithServerDebug(true)` (or
`GRPC_DEBUG=true`), because bodies can contain secrets.

## metautils

`interceptor/grpc/metautils`, adapted from go-grpc-middleware v1. `NiceMD`
wraps gRPC metadata:

- `ExtractIncoming(ctx)`, `ExtractOutgoing(ctx)` — the metadata in the
  context, or an empty one;
- `Get` (first value), `Set` (replace), `Add` (append), `Del`; keys are
  case-insensitive;
- `Clone(keys...)` — a deep copy, optionally of some keys only;
- `ToOutgoing(ctx)`, `ToIncoming(ctx)` — put it back into a context.

## Metadata keys

`interceptor/metadata`:

- `CTX_KEY_APP_CTX = "app-ctx-bin"` — the `AppContext` JSON. gRPC
  base64-encodes headers ending in `-bin` automatically.
- `CTX_KEY_METADATA = "metadata"` — a free-form string.
- `ContextKeyMetadata` — the context key the server interceptor stores the
  `metadata` value under.

The names do not follow Go naming conventions; they are kept for
compatibility.

## See also

- [mctx](../mctx) — the `AppContext` type.
- [grpc](../grpc) — where these interceptors sit in the chain.
- [zerolog](../zerolog) — `log.Ctx` and the fields these interceptors add.
