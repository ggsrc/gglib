# utils

> **Deprecated.** Start spans with OpenTelemetry directly:
> `otel.Tracer("your/package").Start(ctx, "name")`. The service name is
> already set on every span by [otel](../otel).

```sh
go get github.com/ggsrc/gglib/utils
```

Two small helpers that start spans with the global tracer provider
(`TracerName = "github.com/ggsrc/gglib/utils"`):

| Name | What it does |
|---|---|
| `StartTrace(ctx, name, opts...)` | Starts a span on `ctx`; the default kind is internal. |
| `BackgroundCtx(name, opts...)` | Starts a span on a new, empty context. |
