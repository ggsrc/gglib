package zerolog

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	otellog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/trace"

	"github.com/ggsrc/gglib/zerolog/log"
)

// restore saves and restores the global logging state a test changes.
func restore(t *testing.T) {
	t.Helper()
	prevLogger, prevDefault, prevLevel := zlog.Logger, zerolog.DefaultContextLogger, zerolog.GlobalLevel()
	prevLP := otel.GetLoggerProvider()
	for _, k := range []string{"ENV", "OTEL_DEPLOYMENT_ENVIRONMENT", "LOG_LEVEL", "LOGGER_LEVEL", "LOG_FORMAT"} {
		t.Setenv(k, "")
	}
	t.Cleanup(func() {
		zlog.Logger, zerolog.DefaultContextLogger = prevLogger, prevDefault
		zerolog.SetGlobalLevel(prevLevel)
		otel.SetLoggerProvider(prevLP)
	})
}

func spanCtx(t *testing.T) context.Context {
	t.Helper()
	tid, err := trace.TraceIDFromHex("17d023447a330049126f26a9996249c0")
	require.NoError(t, err)
	sid, err := trace.SpanIDFromHex("fd1054dfa5132df5")
	require.NoError(t, err)
	return trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: tid, SpanID: sid, TraceFlags: trace.FlagsSampled,
	}))
}

func lastJSON(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[len(lines)-1]), &m), buf.String())
	return m
}

func TestSetupJSONInCluster(t *testing.T) {
	restore(t)
	t.Setenv("ENV", "prd")
	var buf bytes.Buffer
	Setup(WithWriter(&buf))

	log.Ctx(spanCtx(t)).Info().Str("k", "v").Msg("hello")
	m := lastJSON(t, &buf)
	assert.Equal(t, "info", m["level"])
	assert.Equal(t, "hello", m["message"])
	assert.Equal(t, "v", m["k"])
	assert.Equal(t, "17d023447a330049126f26a9996249c0", m["trace_id"])
	assert.Equal(t, "fd1054dfa5132df5", m["span_id"])
	assert.Contains(t, m["caller"], "setup_test.go")
	assert.Regexp(t, `^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{3}`, m["time"])

	buf.Reset()
	zlog.Info().Msg("no span")
	m = lastJSON(t, &buf)
	assert.NotContains(t, m, "trace_id")

	require.NotNil(t, zerolog.DefaultContextLogger)
	buf.Reset()
	zerolog.DefaultContextLogger.Info().Msg("default")
	assert.Contains(t, buf.String(), "default")
}

func TestSetupConsoleLocally(t *testing.T) {
	restore(t)
	var buf bytes.Buffer
	Setup(WithWriter(&buf), WithoutCaller())
	zlog.Info().Msg("readable")
	out := buf.String()
	assert.Contains(t, out, "readable")
	assert.False(t, strings.HasPrefix(out, "{"), out)
	assert.Contains(t, out, "\x1b[", "colored locally")
}

func TestLogFormatOverride(t *testing.T) {
	restore(t)
	var buf bytes.Buffer
	t.Setenv("LOG_FORMAT", "json")
	Setup(WithWriter(&buf))
	zlog.Info().Msg("x")
	assert.True(t, strings.HasPrefix(buf.String(), "{"), "JSON even locally")

	buf.Reset()
	t.Setenv("ENV", "stg")
	t.Setenv("LOG_FORMAT", "console")
	Setup(WithWriter(&buf))
	zlog.Info().Msg("y")
	assert.False(t, strings.HasPrefix(buf.String(), "{"))
	assert.NotContains(t, buf.String(), "\x1b[", "no color outside local")

	buf.Reset()
	Setup(WithWriter(&buf), WithFormat(FormatJSON))
	zlog.Info().Msg("z")
	assert.True(t, strings.HasPrefix(buf.String(), "{"), "an explicit format wins")
}

func TestLevel(t *testing.T) {
	restore(t)
	var buf bytes.Buffer
	Setup(WithWriter(&buf))
	assert.Equal(t, zerolog.InfoLevel, zerolog.GlobalLevel(), "default")

	t.Setenv("LOGGER_LEVEL", "debug")
	Setup(WithWriter(&buf))
	assert.Equal(t, zerolog.DebugLevel, zerolog.GlobalLevel(), "deprecated LOGGER_LEVEL still read")

	t.Setenv("LOG_LEVEL", "WARN")
	Setup(WithWriter(&buf))
	assert.Equal(t, zerolog.WarnLevel, zerolog.GlobalLevel(), "LOG_LEVEL wins")
	zlog.Info().Msg("dropped")
	assert.Empty(t, buf.String())

	t.Setenv("LOG_LEVEL", "nonsense")
	Setup(WithWriter(&buf))
	assert.Equal(t, zerolog.DebugLevel, zerolog.GlobalLevel(), "unparsable LOG_LEVEL falls through to LOGGER_LEVEL")

	Setup(WithWriter(&buf), WithLevel(zerolog.ErrorLevel))
	assert.Equal(t, zerolog.ErrorLevel, zerolog.GlobalLevel(), "WithLevel wins")
}

type memProcessor struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (p *memProcessor) OnEmit(_ context.Context, r *sdklog.Record) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.records = append(p.records, r.Clone())
	return nil
}
func (p *memProcessor) Enabled(context.Context, sdklog.EnabledParameters) bool { return true }
func (p *memProcessor) Shutdown(context.Context) error                         { return nil }
func (p *memProcessor) ForceFlush(context.Context) error                       { return nil }

func TestOTLPBridge(t *testing.T) {
	restore(t)
	proc := &memProcessor{}
	otel.SetLoggerProvider(sdklog.NewLoggerProvider(sdklog.WithProcessor(proc)))
	var buf bytes.Buffer
	t.Setenv("ENV", "prd")
	Setup(WithWriter(&buf), WithOTLP(true))

	log.Ctx(spanCtx(t)).Warn().Msg("to otlp")
	require.Len(t, proc.records, 1)
	r := proc.records[0]
	assert.Equal(t, "to otlp", r.Body().AsString())
	assert.Equal(t, otellog.SeverityWarn, r.Severity())
	assert.Equal(t, "warn", r.SeverityText())
	assert.Equal(t, "17d023447a330049126f26a9996249c0", r.TraceID().String())
	assert.Contains(t, buf.String(), "to otlp", "stdout still gets the line")
}

func TestOTLPOffByDefault(t *testing.T) {
	restore(t)
	proc := &memProcessor{}
	otel.SetLoggerProvider(sdklog.NewLoggerProvider(sdklog.WithProcessor(proc)))
	var buf bytes.Buffer
	Setup(WithWriter(&buf))
	zlog.Info().Msg("stdout only")
	assert.Empty(t, proc.records)
}

func TestSeverity(t *testing.T) {
	cases := map[zerolog.Level]otellog.Severity{
		zerolog.TraceLevel: otellog.SeverityTrace,
		zerolog.DebugLevel: otellog.SeverityDebug,
		zerolog.InfoLevel:  otellog.SeverityInfo,
		zerolog.WarnLevel:  otellog.SeverityWarn,
		zerolog.ErrorLevel: otellog.SeverityError,
		zerolog.FatalLevel: otellog.SeverityFatal,
		zerolog.PanicLevel: otellog.SeverityFatal4,
		zerolog.NoLevel:    otellog.SeverityUndefined,
	}
	for in, want := range cases {
		assert.Equal(t, want, severity(in), in.String())
	}
}

// InitLogger is what gglib/app calls today: it keeps the service's own
// global logger output and only sets the level and the default context
// logger.
func TestInitLoggerKeepsServiceLogger(t *testing.T) {
	restore(t)
	var buf bytes.Buffer
	zlog.Logger = zerolog.New(&buf).With().Str("svc", "mine").Logger()
	proc := &memProcessor{}
	otel.SetLoggerProvider(sdklog.NewLoggerProvider(sdklog.WithProcessor(proc)))

	InitLogger(true, WithOTLP(true))
	assert.Equal(t, zerolog.DebugLevel, zerolog.GlobalLevel())
	zerolog.DefaultContextLogger.Debug().Msg("via default")
	m := lastJSON(t, &buf)
	assert.Equal(t, "mine", m["svc"], "the service's logger is the base")
	assert.Contains(t, m["caller"], "setup_test.go")
	assert.Len(t, proc.records, 1)

	InitLogger(false)
	assert.Equal(t, zerolog.InfoLevel, zerolog.GlobalLevel())
}
