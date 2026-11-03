package zerolog

import (
	"github.com/rs/zerolog"
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
	e.Str(traceIDField, sc.TraceID().String()).Str(spanIDField, sc.SpanID().String())
}

// Field names of the span that an event was logged in.
const (
	traceIDField = "trace_id"
	spanIDField  = "span_id"
)

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
