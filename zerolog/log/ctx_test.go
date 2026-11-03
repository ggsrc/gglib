package log_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ggsrc/gglib/zerolog/log"
)

// captureGlobal points the global logger at a buffer and clears
// DefaultContextLogger for the test.
func captureGlobal(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prevGlobal, prevDefault := zlog.Logger, zerolog.DefaultContextLogger
	zlog.Logger = zerolog.New(&buf)
	zerolog.DefaultContextLogger = nil
	t.Cleanup(func() {
		zlog.Logger = prevGlobal
		zerolog.DefaultContextLogger = prevDefault
	})
	return &buf
}

// With no logger in the context and no DefaultContextLogger, Ctx used to
// return a disabled logger and the event was dropped.
func TestCtxFallsBackToGlobalLogger(t *testing.T) {
	buf := captureGlobal(t)
	log.Ctx(context.Background()).Error().Msg("not lost")
	assert.Contains(t, buf.String(), "not lost")
}

func TestCtxPrefersDefaultContextLogger(t *testing.T) {
	global := captureGlobal(t)
	var def bytes.Buffer
	l := zerolog.New(&def)
	zerolog.DefaultContextLogger = &l

	log.Ctx(context.Background()).Info().Msg("to default")
	assert.Contains(t, def.String(), "to default")
	assert.Empty(t, global.String())
}

func TestCtxPrefersContextLogger(t *testing.T) {
	global := captureGlobal(t)
	var own bytes.Buffer
	ctx := zerolog.New(&own).With().Str("request_id", "r1").Logger().WithContext(context.Background())

	log.Ctx(ctx).Warn().Msg("to ctx logger")
	assert.Contains(t, own.String(), `"request_id":"r1"`)
	assert.Empty(t, global.String())
}

type ctxKey struct{}

type ctxHook struct{ seen any }

func (h *ctxHook) Run(e *zerolog.Event, _ zerolog.Level, _ string) {
	if ctx := e.GetCtx(); ctx != nil {
		h.seen = ctx.Value(ctxKey{})
	}
}

// Hooks need the context (for the trace id); Ctx binds it to the logger.
func TestCtxBindsContextForHooks(t *testing.T) {
	captureGlobal(t)
	hook := &ctxHook{}
	zlog.Logger = zlog.Logger.Hook(hook)

	ctx := context.WithValue(context.Background(), ctxKey{}, "span-ctx")
	log.Ctx(ctx).Info().Msg("x")
	assert.Equal(t, "span-ctx", hook.seen)
}

func TestMirroredFunctions(t *testing.T) {
	buf := captureGlobal(t)

	log.Err(errors.New("e1")).Msg("err")
	log.Err(nil).Msg("err-nil")
	log.Trace().Msg("trace")
	log.Debug().Msg("debug")
	log.Info().Msg("info")
	log.Warn().Msg("warn")
	log.Error().Msg("error")
	require.Panics(t, func() { log.Panic().Msg("panic") })
	for _, want := range []string{`"error":"e1"`, "err-nil", "trace", "debug", "info", "warn", `"level":"error"`, `"level":"panic"`} {
		assert.Contains(t, buf.String(), want)
	}

	var out bytes.Buffer
	l := log.Output(&out)
	l.Info().Msg("output")
	assert.Contains(t, out.String(), "output")

	w := log.With().Str("k", "v").Logger()
	buf.Reset()
	w.Info().Msg("with")
	assert.Contains(t, buf.String(), `"k":"v"`)

	assert.Equal(t, zerolog.WarnLevel, log.Level(zerolog.WarnLevel).GetLevel())

	buf.Reset()
	sampled := log.Sample(&zerolog.BasicSampler{N: 2})
	for range 4 {
		sampled.Info().Msg("s")
	}
	assert.Equal(t, 2, bytes.Count(buf.Bytes(), []byte(`"s"`)))

	hook := &ctxHook{}
	hooked := log.Hook(hook)
	hooked.Info().Ctx(context.WithValue(context.Background(), ctxKey{}, "h")).Msg("hooked")
	assert.Equal(t, "h", hook.seen)
}
