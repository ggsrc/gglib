# grpc

```sh
go get github.com/ggsrc/gglib/grpc
```

Constructors for a gRPC server and gRPC clients with a standard set of
interceptors (code that runs around every call): Prometheus metrics, panic
recovery, request-context propagation, error logging, call logging and
tracing. A service registers its implementation and gets the rest.

Depends on [interceptor](../interceptor). Built on
[grpc-go](https://github.com/grpc/grpc-go),
[go-grpc-middleware v2](https://github.com/grpc-ecosystem/go-grpc-middleware),
[go-grpc-prometheus](https://github.com/grpc-ecosystem/go-grpc-prometheus) and
[otelgrpc](https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc).

## Server

```go
srv := grpc.NewServerWithOptions(grpc.WithServerPort(9090))
pb.RegisterUserServiceServer(srv.Server(), handler)
// app.Run listens, serves and shuts it down.
```

| Name | What it does |
|---|---|
| `NewServerWithOptions(opts...)` | Configured in code; port `DefaultServerPort` (9090). |
| `NewServerWithDefaultEnvPrefix()` | Configured from `GRPC_PORT` (9090), `GRPC_DEBUG` (false), `GRPC_VERBOSE` (false). Panics if a variable cannot be parsed. |
| `(*Server) Server()` | The underlying `*grpc.Server`, to register services on. |
| `(*Server) Listen(ctx)`, `Serve(lis)`, `Start()` | Bind the port, serve, or both. Separate so that a port conflict is reported before serving. |
| `(*Server) Shutdown(ctx)` | Graceful stop that waits for in-flight calls; when `ctx` ends first, forces the stop and returns the context's error. |

### Server options

| Option | Effect |
|---|---|
| `WithServerPort(p)` | Port. |
| `WithServerVerbose(bool)` | Log the start and the end of every call. Off by default: no call logs at all (failed calls are still logged by the error logger). |
| `WithServerDebug(bool)` | Record request and response bodies in the call's span. Bodies can contain passwords and tokens; off by default. |
| `WithServerUnaryInterceptors(...)` | Your interceptors, after the standard ones. |
| `WithServerGRPCServerOptions(...)` | Raw grpc-go server options. Interceptors added with `grpc.ChainUnaryInterceptor` run after the standard ones; one set with `grpc.UnaryInterceptor` runs before them. |

### Server interceptor order (outermost first)

Unary calls:

```text
metrics → panic recovery → request context → error log → call log → body capture (Debug only) → yours → handler
```

Streaming calls:

```text
metrics → panic recovery → request context → error log → handler
```

- Metrics are outermost, so a call stopped by panic recovery is still counted,
  as `Internal`.
- Tracing is not an interceptor but a gRPC stats handler (otelgrpc). The span
  exists before any interceptor runs, so every layer's logs carry `trace_id`.
- Panic recovery runs outside the request context, so the panic log line has
  `trace_id` but not `request_id` or `user_id`.
- Prometheus metrics include `grpc_server_handled_total` and the latency
  histogram `grpc_server_handling_seconds`.
- The call log uses the logger in the request's context, so it carries
  `request_id` and `user_id`.

## Client

```go
client := grpc.NewClientWithOptions()
conn, err := client.Dial(ctx, "user-service:9090")
if err != nil {
	return err
}
users := userpb.NewUserServiceClient(conn)
```

| Name | What it does |
|---|---|
| `NewClientWithOptions(opts...)`, `NewClient(prefix)`, `NewClientWithDefaultEnvPrefix()` | A client factory. The environment-based ones read only `GRPC_VERBOSE`. |
| `(*Client) Dial(ctx, addr, opts...)` | A connection; the actual connection is made on the first call. Defaults: gzip compression, plaintext transport (for in-cluster traffic), the standard interceptors. Your `opts` come after the defaults; an interceptor passed with `grpc.WithUnaryInterceptor` runs before the standard ones. |
| `WithClientVerbose(bool)` | Log the start and the end of every call. |
| `WithClientUnaryInterceptors(...)` | Your interceptors, after the standard ones. |
| `WithClientMetadataForwarding(mode)` | Which incoming headers this client forwards (see [interceptor](../interceptor#request-context-and-header-forwarding)). Without it, `GGLIB_GRPC_METADATA_FORWARDING` decides. |

Client interceptor order for unary calls (outermost first):

```text
panic recovery → error log → request-context forwarding → call log → metrics → yours → network
```

Streaming calls get only the metrics interceptor; header forwarding applies to
unary calls only.

## See also

- [interceptor](../interceptor) — what each interceptor does.
- [app](../app) — serving and shutting down the server.
- [docs/telemetry.md](../docs/telemetry.md) — what the metrics, spans and logs
  look like.
