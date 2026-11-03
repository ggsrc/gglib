package zerolog

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"

	"github.com/ggsrc/gglib/env"
)

// Format is the output format of the logger.
type Format int

const (
	// FormatAuto writes JSON, except in the local environment (env.IsLocal),
	// where it writes the human-readable console format. LOG_FORMAT=json or
	// LOG_FORMAT=console overrides it.
	FormatAuto Format = iota
	// FormatJSON writes one JSON object per line; Loki indexes its fields.
	FormatJSON
	// FormatConsole writes human-readable lines.
	FormatConsole
)

// DefaultOTLP is whether Setup sends log records to the OpenTelemetry logger
// provider by default. Off: logs reach Loki through stdout (decision G11).
const DefaultOTLP = false

// Option configures Setup.
type Option func(*config)

type config struct {
	level    *zerolog.Level
	format   Format
	writer   io.Writer
	otlp     bool
	noCaller bool
}

// WithLevel sets the minimum level, overriding LOG_LEVEL.
func WithLevel(level zerolog.Level) Option {
	return func(c *config) { c.level = &level }
}

// WithFormat sets the output format (default FormatAuto).
func WithFormat(f Format) Option {
	return func(c *config) { c.format = f }
}

// WithWriter sets where logs are written (default os.Stderr).
func WithWriter(w io.Writer) Option {
	return func(c *config) { c.writer = w }
}

// WithOTLP also sends each log record to the global OpenTelemetry logger
// provider, which gglib/otel installs when its log export is on. The record
// carries the trace context of the event's context.
func WithOTLP(on bool) Option {
	return func(c *config) { c.otlp = on }
}

// WithoutCaller omits the caller (file:line) field.
func WithoutCaller() Option {
	return func(c *config) { c.noCaller = true }
}

// Setup configures the global logger (github.com/rs/zerolog/log.Logger) and
// zerolog.DefaultContextLogger, which gglib's log.Ctx falls back to:
//
//   - level: WithLevel, else LOG_LEVEL, else LOGGER_LEVEL (deprecated), else
//     info; it is set as the global level, so it is the only knob;
//   - format: see Format; console output is colored only in the local
//     environment;
//   - fields: time (RFC 3339 with milliseconds), level, message, caller,
//     and trace_id and span_id when the event's context holds a span
//     (log.Ctx binds it);
//   - errors logged with Stack() include their stack trace.
//
// Call it once, at the start of main.
func Setup(opts ...Option) {
	cfg := &config{format: FormatAuto, writer: os.Stderr, otlp: DefaultOTLP}
	for _, opt := range opts {
		opt(cfg)
	}

	level := zerolog.InfoLevel
	if cfg.level != nil {
		level = *cfg.level
	} else if l, ok := levelFromEnv(); ok {
		level = l
	}
	zerolog.SetGlobalLevel(level)
	zerolog.TimeFieldFormat = "2006-01-02T15:04:05.000Z07:00"
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack //nolint:reassign // zerolog is configured through package variables

	ctx := zerolog.New(writerFor(cfg)).Hook(traceHook{}).With().Timestamp()
	if !cfg.noCaller {
		ctx = ctx.Caller()
	}
	logger := ctx.Logger()
	if cfg.otlp {
		logger = logger.Hook(newOTLPHook())
	}

	log.Logger = logger
	defaultLogger := logger
	zerolog.DefaultContextLogger = &defaultLogger
}

// levelFromEnv reads LOG_LEVEL, then the deprecated LOGGER_LEVEL.
func levelFromEnv() (zerolog.Level, bool) {
	for _, key := range []string{"LOG_LEVEL", "LOGGER_LEVEL"} {
		v := strings.TrimSpace(os.Getenv(key))
		if v == "" {
			continue
		}
		if l, err := zerolog.ParseLevel(strings.ToLower(v)); err == nil {
			return l, true
		}
	}
	return zerolog.NoLevel, false
}

func writerFor(cfg *config) io.Writer {
	format := cfg.format
	if format == FormatAuto {
		switch strings.ToLower(strings.TrimSpace(os.Getenv("LOG_FORMAT"))) {
		case "json":
			format = FormatJSON
		case "console":
			format = FormatConsole
		default:
			format = FormatJSON
			if env.IsLocal() {
				format = FormatConsole
			}
		}
	}
	if format == FormatConsole {
		return zerolog.ConsoleWriter{
			Out:        cfg.writer,
			TimeFormat: time.DateTime,
			NoColor:    !env.IsLocal(),
		}
	}
	return cfg.writer
}

// InitLogger keeps the behavior gglib/app has relied on: it sets the global
// level to debug or info and makes zerolog.DefaultContextLogger a copy of
// the existing global logger with a caller field, plus the OpenTelemetry
// bridge when WithOTLP(true) is given. Unlike Setup, it leaves the global
// logger's output and format alone.
//
// Deprecated: use Setup.
func InitLogger(debug bool, opts ...Option) {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack //nolint:reassign // zerolog is configured through package variables

	logger := log.With().Caller().Logger()
	if cfg.otlp {
		logger = logger.Hook(newOTLPHook())
	}
	zerolog.DefaultContextLogger = &logger
}
