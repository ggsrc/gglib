package log_test

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"

	otelzerolog "github.com/ggsrc/gglib/zerolog"
	"github.com/ggsrc/gglib/zerolog/log"
)

func TestLog(t *testing.T) {
	otelzerolog.Setup(otelzerolog.WithLevel(zerolog.DebugLevel))
	traceID, err := trace.TraceIDFromHex("17d023447a330049126f26a9996249c0")
	require.NoError(t, err)
	spaceID, err := trace.SpanIDFromHex("fd1054dfa5132df5")
	require.NoError(t, err)
	ctx := trace.ContextWithSpanContext(t.Context(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spaceID,
		TraceFlags: trace.FlagsSampled,
		TraceState: trace.TraceState{},
		Remote:     false,
	}))

	logger := log.With().Ctx(ctx).Logger()
	logger.Info().Msg("hello world")
}
