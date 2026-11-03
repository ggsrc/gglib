package zerolog

import (
	"io"
	"testing"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/log/noop"
)

// BenchmarkOTLPWriter measures the cost Setup adds per log line when OTLP is
// on (decoding the line and building the record), with a provider that
// drops records.
func BenchmarkOTLPWriter(b *testing.B) {
	prev := otel.GetLoggerProvider()
	b.Cleanup(func() { otel.SetLoggerProvider(prev) })
	otel.SetLoggerProvider(noop.NewLoggerProvider())
	for _, bc := range []struct {
		name string
		out  io.Writer
	}{
		{"stdout-only", io.Discard},
		{"with-otlp", zerolog.MultiLevelWriter(io.Discard, newOTLPWriter())},
	} {
		b.Run(bc.name, func(b *testing.B) {
			logger := zerolog.New(bc.out).With().Timestamp().Logger()
			for b.Loop() {
				logger.Info().Str("trace_id", "17d023447a330049126f26a9996249c0").Str("span_id", "fd1054dfa5132df5").
					Str("grpc_method", "/svc.v1.Svc/Get").Str("user_id", "12345").Int("code", 0).
					Dur("elapsed", 1500).Msg("finished call")
			}
		})
	}
}
