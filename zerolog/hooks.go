package zerolog

import (
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/trace"
)

// traceHook adds trace_id and span_id from the span in the event's context.
type traceHook struct{}

func (traceHook) Run(e *zerolog.Event, _ zerolog.Level, _ string) {
	ctx := e.GetCtx()
	if ctx == nil {
		return
	}
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return
	}
	e.Str("trace_id", sc.TraceID().String()).Str("span_id", sc.SpanID().String())
}

// otlpHook sends each event's level and message to the global OpenTelemetry
// logger provider, with the trace context of the event's context. Fields
// stay in the stdout line; zerolog hooks cannot read them.
type otlpHook struct {
	logger otellog.Logger
}

func newOTLPHook() otlpHook {
	return otlpHook{logger: otel.GetLoggerProvider().Logger("github.com/ggsrc/gglib/zerolog")}
}

func (h otlpHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	if level == zerolog.Disabled {
		return
	}
	var r otellog.Record
	r.SetTimestamp(time.Now())
	r.SetSeverity(severity(level))
	r.SetSeverityText(level.String())
	r.SetBody(attribute.StringValue(msg))
	h.logger.Emit(e.GetCtx(), r)
}

func severity(l zerolog.Level) otellog.Severity {
	switch l {
	case zerolog.TraceLevel:
		return otellog.SeverityTrace
	case zerolog.DebugLevel:
		return otellog.SeverityDebug
	case zerolog.InfoLevel:
		return otellog.SeverityInfo
	case zerolog.WarnLevel:
		return otellog.SeverityWarn
	case zerolog.ErrorLevel:
		return otellog.SeverityError
	case zerolog.FatalLevel:
		return otellog.SeverityFatal
	case zerolog.PanicLevel:
		return otellog.SeverityFatal4
	default:
		return otellog.SeverityUndefined
	}
}
