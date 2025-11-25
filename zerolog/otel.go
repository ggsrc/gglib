package zerolog

import (
	"context"

	"github.com/agoda-com/opentelemetry-go/otelzerolog"
	"github.com/agoda-com/opentelemetry-logs-go/exporters/otlp/otlplogs"
	"github.com/agoda-com/opentelemetry-logs-go/exporters/otlp/otlplogs/otlplogshttp"
	sdklogs "github.com/agoda-com/opentelemetry-logs-go/sdk/logs"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"

	"github.com/ggsrc/gglib/env"
)

// LoggerOption is a function that configures the logger
type LoggerOption func(*loggerConfig)

type loggerConfig struct {
	otelEnabled bool
	batchSize   int
}

// WithOTLP enables OTLP export for logs
func WithOTLP() LoggerOption {
	return func(c *loggerConfig) {
		c.otelEnabled = true
	}
}

// WithBatchSize sets the OTLP export batch size
func WithBatchSize(size int) LoggerOption {
	return func(c *loggerConfig) {
		c.batchSize = size
	}
}

// setupLogger initializes the zerolog logger with the given options (internal use)
func setupLogger(opts ...LoggerOption) {
	// Default configuration
	config := &loggerConfig{
		otelEnabled: false,
		batchSize:   512,
	}

	// Apply options
	for _, opt := range opts {
		opt(config)
	}

	// Auto-adjust batch size for staging environment
	if config.otelEnabled && env.IsStaging() {
		config.batchSize = 10
	}

	// Set up error stack marshaler
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

	// Create base logger with caller info
	loggerVal := log.With().Caller().Logger()

	// Add OTLP hook if enabled
	if config.otelEnabled {
		ctx := context.Background()
		exporter, _ := otlplogs.NewExporter(ctx, otlplogs.WithClient(otlplogshttp.NewClient()))
		loggerProvider := sdklogs.NewLoggerProvider(
			sdklogs.WithBatcher(
				exporter,
				// add following two options to ensure flush
				//sdklogs.WithBatchTimeout(5*time.Second),
				sdklogs.WithMaxExportBatchSize(config.batchSize),
			),
		)
		hook := otelzerolog.NewHook(loggerProvider)
		loggerVal = loggerVal.Hook(hook)
	}

	// Set as default context logger
	zerolog.DefaultContextLogger = &loggerVal
}
