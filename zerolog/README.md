# zerolog

```sh
go get github.com/ggsrc/gglib/zerolog
```

One call sets up logging for the whole process: level, format, the fields
every line carries, trace correlation, and a copy of every record sent to
OpenTelemetry. The `zerolog/log` package in this module is what code uses to
write logs: `log.Ctx(ctx)` returns the logger for the current request, with its
request ID, user ID and trace ID.

Built on [rs/zerolog](https://github.com/rs/zerolog). Depends on [env](../env).

## Setup

```go
import ggzerolog "github.com/ggsrc/gglib/zerolog"

ggzerolog.Setup() // or with options, see below
```

[app](../app) calls `Setup` in `app.New`; call it yourself only in programs
that do not use `app`, once, at the start of `main`.

| Aspect | Behaviour |
|---|---|
| Level | `WithLevel`, else `LOG_LEVEL`, else `LOGGER_LEVEL` (deprecated), else `info`. Invalid values are ignored. It sets zerolog's global level, so it is the only switch. |
| Format | Automatic by default: JSON, one object per line, except in the `local` environment ([env](../env)), where lines are human-readable. Human-readable output is colored only in `local`. `LOG_FORMAT=json` or `LOG_FORMAT=console` forces one. |
| Output | Standard error, or the writer given to `WithWriter`. |
| Fields on every line | `time` (RFC 3339 with milliseconds), `level`, `message`, `caller` (`file:line`; `WithoutCaller` removes it). |
| Trace fields | When the logger has a context with a valid span, `trace_id` and `span_id`. `log.Ctx(ctx)` binds the context, so logs written through it get them. |
| Error stacks | Errors logged with `.Stack()` carry their stack trace. |
| OpenTelemetry | On by default (`DefaultOTLP = true`); see below. |
| Globals set | `github.com/rs/zerolog/log.Logger` and `zerolog.DefaultContextLogger`. |

### Options

| Option | Effect |
|---|---|
| `WithLevel(zerolog.Level)` | Minimum level, overriding the environment. |
| `WithFormat(FormatAuto \| FormatJSON \| FormatConsole)` | Output format, overriding `LOG_FORMAT`. |
| `WithWriter(io.Writer)` | Destination instead of standard error. |
| `WithOTLP(bool)` | Turns the OpenTelemetry copy on or off. |
| `WithoutCaller()` | Drops the `caller` field. |

## Logs in OpenTelemetry

Each line written to the output is also handed to a writer that parses the
JSON into an OpenTelemetry log record:

| JSON field | Becomes |
|---|---|
| `message` | The record's body. |
| `level` | Severity (trace, debug, info, warn, error, fatal; panic maps to the highest, fatal4). |
| `time` | Timestamp; the observed timestamp is the time of writing. |
| `trace_id`, `span_id` | The record's trace context, which lets a tracing backend show the log under its span. |
| every other field | Attributes. Nested objects become maps, arrays stay arrays. Integers that fit in int64 stay integers, numbers with a decimal point or exponent become floats, larger integers are kept as strings without losing precision. |

- Records leave the process only if [otel](../otel) exports logs and an OTLP
  endpoint is configured. The output on standard error is unaffected.
- Lines that are not a JSON object are skipped (they still reach standard
  error). The writer never returns an error, so it cannot make logging fail.
- It looks up the global OpenTelemetry logger provider at write time, so
  `otel.Setup` may run before or after `zerolog.Setup`.
- Cost: about 5 µs per line.
- Records carry `trace_id` and `span_id` but no sampling flag.

## zerolog/log

```go
import "github.com/ggsrc/gglib/zerolog/log"

func (s *Service) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
	log.Ctx(ctx).Info().Str("id", req.Id).Msg("loading user")
	// ...
}
```

The package mirrors `github.com/rs/zerolog/log` (`log.Info()`, `log.Error()`,
`log.With()`, … on the global logger) and adds `Ctx`:

| Name | What it does |
|---|---|
| `Ctx(ctx) *zerolog.Logger` | The logger for this request: the one stored in `ctx` (the gRPC server stores one with `request_id` and `user_id`, see [interceptor](../interceptor)), else the default context logger, else the global logger. It never returns a disabled logger. The returned logger is bound to `ctx`, so `trace_id` and `span_id` are added. |
| `Fatal()` | Logs, then calls `os.Exit(1)`. |
| `Panic()` | Logs, then panics. |

## See also

- [otel](../otel) — whether log records are exported, and where to.
- [docs/telemetry.md](../docs/telemetry.md) — an example log line and how the
  fields are used.
