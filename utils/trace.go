// Package utils has small tracing helpers.
//
// Deprecated: start spans with go.opentelemetry.io/otel directly
// (otel.Tracer("<your package>").Start). The resource that gglib/otel sets
// up already carries service.name.
package utils

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// TracerName is the instrumentation scope of spans started here.
const TracerName = "github.com/ggsrc/gglib/utils"

// StartTrace starts a span with the global tracer provider. The span kind is
// internal unless opts set another one.
func StartTrace(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	opts = append([]trace.SpanStartOption{trace.WithSpanKind(trace.SpanKindInternal)}, opts...)
	return otel.Tracer(TracerName).Start(ctx, spanName, opts...) //nolint:spancheck // the caller ends the span
}

// BackgroundCtx starts a span named name on a new background context.
func BackgroundCtx(name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return StartTrace(context.Background(), name, opts...)
}
