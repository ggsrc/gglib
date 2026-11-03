package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func recordSpans(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec)))
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	return rec
}

// The span kind passed by the caller used to be overwritten with internal,
// and the tracer was bound when the package loaded.
func TestStartTrace(t *testing.T) {
	rec := recordSpans(t)

	_, span := StartTrace(t.Context(), "a")
	span.End()
	_, span = StartTrace(t.Context(), "b", trace.WithSpanKind(trace.SpanKindClient), trace.WithAttributes(attribute.String("k", "v")))
	span.End()

	ended := rec.Ended()
	require.Len(t, ended, 2)
	assert.Equal(t, trace.SpanKindInternal, ended[0].SpanKind())
	assert.Equal(t, trace.SpanKindClient, ended[1].SpanKind(), "the caller's kind wins")
	assert.Equal(t, TracerName, ended[0].InstrumentationScope().Name)
	assert.Contains(t, ended[1].Attributes(), attribute.String("k", "v"))
}

// BackgroundCtx used to ignore its options.
func TestBackgroundCtx(t *testing.T) {
	rec := recordSpans(t)
	ctx, span := BackgroundCtx("job", trace.WithAttributes(attribute.String("job", "nightly")))
	assert.True(t, trace.SpanFromContext(ctx).SpanContext().IsValid())
	span.End()

	ended := rec.Ended()
	require.Len(t, ended, 1)
	assert.Equal(t, "job", ended[0].Name())
	assert.Contains(t, ended[0].Attributes(), attribute.String("job", "nightly"))
}
