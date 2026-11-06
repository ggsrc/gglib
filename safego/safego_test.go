package safego_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ggsrc/gglib/safego"
)

// logContext returns a context whose logger writes JSON lines to the
// returned buffer, and a function that decodes those lines.
func logContext(t *testing.T) (context.Context, func() []map[string]any) {
	t.Helper()
	var (
		mu  sync.Mutex
		buf bytes.Buffer
	)
	w := writerFunc(func(p []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		return buf.Write(p)
	})
	ctx := zerolog.New(w).WithContext(context.Background())
	entries := func() []map[string]any {
		t.Helper()
		mu.Lock()
		defer mu.Unlock()
		var out []map[string]any
		dec := json.NewDecoder(bytes.NewReader(buf.Bytes()))
		for dec.More() {
			var e map[string]any
			require.NoError(t, dec.Decode(&e))
			out = append(out, e)
		}
		return out
	}
	return ctx, entries
}

type writerFunc func(p []byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

func TestRunReturnsResult(t *testing.T) {
	t.Parallel()
	ctx, entries := logContext(t)
	want := errors.New("upstream failed")

	require.NoError(t, safego.Run(ctx, "test.ok", func() error { return nil }))
	require.ErrorIs(t, safego.Run(ctx, "test.err", func() error { return want }), want)
	assert.Empty(t, entries(), "an ordinary error is the caller's to log")
}

func TestRunRecoversAndLogsOnce(t *testing.T) {
	t.Parallel()
	ctx, entries := logContext(t)

	err := safego.Run(ctx, "test.panic", func() error { panic("private detail") })

	require.ErrorIs(t, err, safego.ErrPanic)
	assert.NotContains(t, err.Error(), "private", "the error must not carry the panic value")
	logged := entries()
	require.Len(t, logged, 1)
	assert.Equal(t, "error", logged[0]["level"])
	assert.Equal(t, "safego: panic recovered", logged[0]["message"])
	assert.Equal(t, "test.panic", logged[0]["label"])
	assert.Equal(t, "private detail", logged[0]["panic"])
	assert.Contains(t, logged[0]["stack"], "safego_test.go")
}

func TestRunBindsCallerContextToLog(t *testing.T) {
	t.Parallel()
	type key struct{}
	var seen context.Context
	logger := zerolog.New(writerFunc(func(p []byte) (int, error) { return len(p), nil })).
		Hook(zerolog.HookFunc(func(e *zerolog.Event, _ zerolog.Level, _ string) {
			seen = e.GetCtx()
		}))
	ctx := logger.WithContext(context.WithValue(context.Background(), key{}, "request"))

	_ = safego.Run(ctx, "test.ctx", func() error { panic("boom") })

	require.NotNil(t, seen)
	assert.Equal(t, "request", seen.Value(key{}), "the log event must carry the caller's context")
}

func TestRunRecoversNilFunction(t *testing.T) {
	t.Parallel()
	ctx, _ := logContext(t)
	require.ErrorIs(t, safego.Run(ctx, "test.nil", nil), safego.ErrPanic)
}

func TestRunRecoversPanicWithError(t *testing.T) {
	t.Parallel()
	ctx, entries := logContext(t)
	cause := errors.New("cause")

	err := safego.Run(ctx, "test.panic-error", func() error { panic(cause) })

	require.ErrorIs(t, err, safego.ErrPanic)
	require.NotErrorIs(t, err, cause, "a panic value is never returned to the caller")
	require.Len(t, entries(), 1)
}

func TestGoRecoversPanic(t *testing.T) {
	t.Parallel()
	ctx, entries := logContext(t)
	done := make(chan struct{})

	safego.Go(ctx, "test.go", func() {
		defer close(done)
		panic("boom")
	})

	<-done
	require.Eventually(t, func() bool { return len(entries()) == 1 }, time.Second, time.Millisecond)
	assert.Equal(t, "test.go", entries()[0]["label"])
}

func TestGoRunsFunction(t *testing.T) {
	t.Parallel()
	ctx, entries := logContext(t)
	done := make(chan struct{})

	safego.Go(ctx, "test.go", func() { close(done) })

	<-done
	assert.Empty(t, entries())
}

func TestAsyncDeliversResultOrPanic(t *testing.T) {
	t.Parallel()
	ctx, _ := logContext(t)
	want := errors.New("upstream failed")

	require.NoError(t, <-safego.Async(ctx, "test.async", func() error { return nil }))
	require.ErrorIs(t, <-safego.Async(ctx, "test.async", func() error { return want }), want)
	require.ErrorIs(t, <-safego.Async(ctx, "test.async", func() error { panic("boom") }), safego.ErrPanic)
}

func TestAsyncFinishesWhenCallerStopsWaiting(t *testing.T) {
	t.Parallel()
	ctx, _ := logContext(t)
	finished := make(chan struct{})

	_ = safego.Async(ctx, "test.async", func() error {
		defer close(finished)
		return nil
	})

	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("the goroutine blocked on an unread channel")
	}
}

func TestGroupWaitsForAllAndReturnsPanic(t *testing.T) {
	t.Parallel()
	ctx, entries := logContext(t)
	g := safego.NewGroup(ctx, "test.group")
	finished := make(chan struct{})

	g.Go(func() error { panic("boom") })
	g.Go(func() error {
		time.Sleep(20 * time.Millisecond)
		close(finished)
		return nil
	})

	require.ErrorIs(t, g.Wait(), safego.ErrPanic)
	select {
	case <-finished:
	default:
		t.Fatal("Wait returned before every goroutine finished")
	}
	require.Len(t, entries(), 1)
	assert.Equal(t, "test.group", entries()[0]["label"])
}

func TestGroupReturnsOrdinaryError(t *testing.T) {
	t.Parallel()
	ctx, _ := logContext(t)
	want := errors.New("upstream failed")
	g := safego.NewGroup(ctx, "test.group")

	g.Go(func() error { return want })
	g.Go(func() error { return nil })

	err := g.Wait()
	require.ErrorIs(t, err, want)
	require.NotErrorIs(t, err, safego.ErrPanic)
}

func TestWithContextCancelsSiblingsOnPanic(t *testing.T) {
	t.Parallel()
	ctx, _ := logContext(t)
	g, groupCtx := safego.WithContext(ctx, "test.group")

	g.Go(func() error { panic("boom") })
	g.Go(func() error {
		select {
		case <-groupCtx.Done():
			return groupCtx.Err()
		case <-time.After(time.Second):
			return errors.New("sibling was not canceled")
		}
	})

	require.ErrorIs(t, g.Wait(), safego.ErrPanic)
	require.ErrorIs(t, groupCtx.Err(), context.Canceled)
}

func TestGoIntoStoresBranchErrorWithoutFailingGroup(t *testing.T) {
	t.Parallel()
	ctx, _ := logContext(t)
	g, groupCtx := safego.WithContext(ctx, "test.group")
	want := errors.New("optional source failed")
	var panicErr, ordinaryErr, okErr error

	g.GoInto(&panicErr, func() error { panic("boom") })
	g.GoInto(&ordinaryErr, func() error { return want })
	g.GoInto(&okErr, func() error { return nil })
	g.GoInto(nil, func() error { panic("discarded") })
	g.Go(func() error {
		time.Sleep(20 * time.Millisecond)
		return groupCtx.Err()
	})

	require.NoError(t, g.Wait(), "GoInto must neither fail nor cancel the group")
	require.ErrorIs(t, panicErr, safego.ErrPanic)
	require.ErrorIs(t, ordinaryErr, want)
	require.NoError(t, okErr)
}

func TestGroupSetLimitAndTryGo(t *testing.T) {
	t.Parallel()
	ctx, _ := logContext(t)
	g := safego.NewGroup(ctx, "test.group")
	g.SetLimit(1)
	release := make(chan struct{})

	require.True(t, g.TryGo(func() error {
		<-release
		return nil
	}))
	assert.False(t, g.TryGo(func() error { return nil }), "the limit is reached")
	close(release)
	require.NoError(t, g.Wait())

	require.True(t, g.TryGo(func() error { panic("boom") }))
	require.ErrorIs(t, g.Wait(), safego.ErrPanic)
}

func TestGroupLimitBlocksGo(t *testing.T) {
	t.Parallel()
	ctx, _ := logContext(t)
	g := safego.NewGroup(ctx, "test.group")
	g.SetLimit(2)
	var (
		mu              sync.Mutex
		active, maxSeen int
	)

	for range 10 {
		g.Go(func() error {
			mu.Lock()
			active++
			maxSeen = max(maxSeen, active)
			mu.Unlock()
			time.Sleep(2 * time.Millisecond)
			mu.Lock()
			active--
			mu.Unlock()
			return nil
		})
	}

	require.NoError(t, g.Wait())
	assert.LessOrEqual(t, maxSeen, 2)
}
