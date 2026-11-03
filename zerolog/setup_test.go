package zerolog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
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

func useMemProvider(t *testing.T) *memProcessor {
	t.Helper()
	proc := &memProcessor{}
	otel.SetLoggerProvider(sdklog.NewLoggerProvider(sdklog.WithProcessor(proc)))
	return proc
}

func attrs(r sdklog.Record) map[string]attribute.Value {
	out := map[string]attribute.Value{}
	r.WalkAttributes(func(kv attribute.KeyValue) bool {
		out[string(kv.Key)] = kv.Value
		return true
	})
	return out
}

// G11: records reach OpenTelemetry by default, with their fields.
func TestOTLPOnByDefaultWithFields(t *testing.T) {
	restore(t)
	proc := useMemProvider(t)
	var buf bytes.Buffer
	t.Setenv("ENV", "prd")
	Setup(WithWriter(&buf))

	log.Ctx(spanCtx(t)).Warn().
		Str("user_id", "u1").
		Int64("big", 1<<62).
		Uint64("huge", 1<<63).
		Float64("ratio", 0.5).
		Bool("ok", true).
		Interface("nested", map[string]any{"a": []int{1, 2}}).
		Interface("none", nil).
		Err(errors.New("boom")).
		Msg("to otlp")
	require.Len(t, proc.records, 1)
	r := proc.records[0]
	assert.Equal(t, "to otlp", r.Body().AsString())
	assert.Equal(t, otellog.SeverityWarn, r.Severity())
	assert.Equal(t, "warn", r.SeverityText())
	assert.Equal(t, "17d023447a330049126f26a9996249c0", r.TraceID().String())
	assert.Equal(t, "fd1054dfa5132df5", r.SpanID().String())
	assert.WithinDuration(t, time.Now(), r.Timestamp(), time.Minute)
	assert.Equal(t, r.Timestamp().Truncate(time.Millisecond), r.Timestamp(), "timestamp parsed from the time field")
	assert.False(t, r.ObservedTimestamp().IsZero())

	a := attrs(r)
	assert.Equal(t, "u1", a["user_id"].AsString())
	assert.Equal(t, int64(1<<62), a["big"].AsInt64())
	assert.Equal(t, "9223372036854775808", a["huge"].AsString(), "beyond int64: exact, as a string")
	assert.InDelta(t, 0.5, a["ratio"].AsFloat64(), 0)
	assert.True(t, a["ok"].AsBool())
	assert.Equal(t, "boom", a["error"].AsString())
	assert.Contains(t, a["caller"].AsString(), "setup_test.go")
	nested := a["nested"].AsMap()
	require.Len(t, nested, 1)
	assert.Equal(t, attribute.Key("a"), nested[0].Key)
	assert.Len(t, nested[0].Value.AsSlice(), 2)
	assert.Equal(t, attribute.EMPTY, a["none"].Type())
	for _, k := range []string{"time", "level", "message", "trace_id", "span_id"} {
		assert.NotContains(t, a, k, "%s is carried by the record itself", k)
	}
	assert.Contains(t, buf.String(), "to otlp", "stdout still gets the line")
}

func TestOTLPInConsoleFormat(t *testing.T) {
	restore(t)
	proc := useMemProvider(t)
	var buf bytes.Buffer
	Setup(WithWriter(&buf), WithFormat(FormatConsole))
	zlog.Info().Str("k", "v").Msg("hello")
	require.Len(t, proc.records, 1)
	assert.Equal(t, "hello", proc.records[0].Body().AsString())
	assert.Equal(t, "v", attrs(proc.records[0])["k"].AsString())
	assert.False(t, proc.records[0].TraceID().IsValid(), "no span, no trace context")
}

func TestWithOTLPFalse(t *testing.T) {
	restore(t)
	proc := useMemProvider(t)
	var buf bytes.Buffer
	Setup(WithWriter(&buf), WithOTLP(false))
	zlog.Info().Msg("stdout only")
	assert.Empty(t, proc.records)
	assert.Contains(t, buf.String(), "stdout only")
}

func TestOTLPWriterEdgeCases(t *testing.T) {
	restore(t)
	proc := useMemProvider(t)
	w := newOTLPWriter()

	n, err := w.Write([]byte("not json\n"))
	require.NoError(t, err)
	assert.Equal(t, 9, n)
	_, err = w.Write([]byte("null\n"))
	require.NoError(t, err)
	for _, line := range []string{`{"a"`, `{"a":`, `{"a":[1,`, `{"a":[1`, `{"a":{"b"`, `{"a":{"b":1`, `{"a":{"b":}`, `{"a":1,`} {
		_, err = w.Write([]byte(line))
		require.NoError(t, err)
	}
	assert.Empty(t, proc.records, "lines that are not complete JSON objects are dropped")

	// Level from the line; unparsable time and bad trace ids are tolerated.
	_, err = w.Write([]byte(`{"level":"error","time":"yesterday","trace_id":"zz","span_id":"fd1054dfa5132df5","n":1e3}` + "\n"))
	require.NoError(t, err)
	_, err = w.Write([]byte(`{"time":12345,"trace_id":"17d023447a330049126f26a9996249c0","span_id":"bad"}` + "\n"))
	require.NoError(t, err)
	require.Len(t, proc.records, 2)
	r := proc.records[0]
	assert.Equal(t, otellog.SeverityError, r.Severity())
	assert.False(t, r.TraceID().IsValid())
	assert.WithinDuration(t, time.Now(), r.Timestamp(), time.Minute)
	assert.InDelta(t, 1000.0, attrs(r)["n"].AsFloat64(), 0)
	assert.Equal(t, otellog.SeverityUndefined, proc.records[1].Severity())
	assert.False(t, proc.records[1].TraceID().IsValid())
	assert.Equal(t, attribute.EMPTY, proc.records[1].Body().Type(), "no message, no body")
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
