package goroutine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ggsrc/gglib/resource"
)

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) entries(t *testing.T) []map[string]any {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []map[string]any
	dec := json.NewDecoder(bytes.NewReader(b.buf.Bytes()))
	for dec.More() {
		var e map[string]any
		require.NoError(t, dec.Decode(&e))
		out = append(out, e)
	}
	return out
}

// started returns an initialized and started manager whose goroutines log to
// the returned buffer.
func started(t *testing.T) (*GoroutineManager, *syncBuffer) {
	t.Helper()
	logs := &syncBuffer{}
	ctx := zerolog.New(logs).WithContext(context.Background())
	gm := NewGoroutineManager()
	require.NoError(t, gm.Init(ctx))
	require.NoError(t, gm.Start(ctx))
	return gm, logs
}

func TestGoroutineManager_GracefulShutdown_WithWork(t *testing.T) {
	t.Parallel()
	gm, _ := started(t)

	var workCompleted atomic.Int32
	numGoroutines := 3

	for range numGoroutines {
		gm.Run("worker", func(ctx context.Context) error {
			// Simulate work that needs to complete before shutdown
			for {
				select {
				case <-ctx.Done():
					// Perform cleanup work
					time.Sleep(100 * time.Millisecond)
					workCompleted.Add(1)
					return nil
				case <-time.After(5 * time.Millisecond):
					// Simulate periodic work
				}
			}
		})
	}

	// Stop should wait for cleanup to complete
	require.NoError(t, gm.Stop(context.Background()))
	assert.Equal(t, int32(numGoroutines), workCompleted.Load())
}

func TestGoroutineManager_StopTimeout(t *testing.T) {
	t.Parallel()
	gm, _ := started(t)
	release := make(chan struct{})
	gm.Run("slow", func(context.Context) error {
		<-release
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, gm.Stop(ctx), context.DeadlineExceeded)

	close(release)
	require.NoError(t, gm.Stop(context.Background()), "a second Stop waits for the rest")
}

func TestGoroutineManager_RunBeforeInitPanics(t *testing.T) {
	t.Parallel()
	gm := NewGoroutineManager()
	require.Panics(t, func() {
		gm.Run("early", func(context.Context) error { return nil })
	})
}

func TestGoroutineManager_RunAfterStopDoesNotStart(t *testing.T) {
	t.Parallel()
	gm, logs := started(t)
	require.NoError(t, gm.Stop(context.Background()))

	var ran atomic.Bool
	gm.Run("late", func(context.Context) error {
		ran.Store(true)
		return nil
	})
	require.NoError(t, gm.Stop(context.Background()))

	assert.False(t, ran.Load())
	logged := logs.entries(t)
	require.Len(t, logged, 1)
	assert.Equal(t, "error", logged[0]["level"])
	assert.Equal(t, "late", logged[0]["goroutine"])
}

func TestGoroutineManager_PanicIsRecoveredAndLogged(t *testing.T) {
	t.Parallel()
	gm, logs := started(t)

	gm.Run("crashy", func(context.Context) error { panic("boom") })
	var survived atomic.Bool
	gm.Run("healthy", func(ctx context.Context) error {
		<-ctx.Done()
		survived.Store(true)
		return nil
	})

	require.Eventually(t, func() bool { return len(logs.entries(t)) == 1 }, time.Second, time.Millisecond)
	require.NoError(t, gm.Stop(context.Background()))
	assert.True(t, survived.Load(), "a panic in one goroutine must not affect the others")

	logged := logs.entries(t)
	require.Len(t, logged, 1, "the panic is logged once")
	assert.Equal(t, "error", logged[0]["level"])
	assert.Equal(t, "goroutine crashy", logged[0]["label"])
	assert.Equal(t, "boom", logged[0]["panic"])
	assert.NotEmpty(t, logged[0]["stack"])
}

func TestGoroutineManager_ErrorIsLogged(t *testing.T) {
	t.Parallel()
	gm, logs := started(t)

	gm.Run("failing", func(context.Context) error { return errors.New("upstream failed") })
	gm.Run("fine", func(context.Context) error { return nil })
	require.NoError(t, gm.Stop(context.Background()))

	logged := logs.entries(t)
	require.Len(t, logged, 1)
	assert.Equal(t, "error", logged[0]["level"])
	assert.Equal(t, "upstream failed", logged[0]["error"])
	assert.Equal(t, "failing", logged[0]["goroutine"])
	assert.Equal(t, "goroutine [failing] error", logged[0]["message"])
}

func TestGoroutineManager_SecondInitKeepsFirstContext(t *testing.T) {
	t.Parallel()
	gm, _ := started(t)
	canceled := make(chan struct{})
	gm.Run("first", func(ctx context.Context) error {
		<-ctx.Done()
		close(canceled)
		return nil
	})

	require.NoError(t, gm.Init(context.Background()))
	require.NoError(t, gm.Stop(context.Background()))

	select {
	case <-canceled:
	default:
		t.Fatal("Stop did not cancel a goroutine started before the second Init")
	}
}

func TestGoroutineManager_InitContextCancelsGoroutines(t *testing.T) {
	t.Parallel()
	type key struct{}
	parent, cancel := context.WithCancel(context.WithValue(context.Background(), key{}, "value"))
	gm := NewGoroutineManager()
	require.NoError(t, gm.Init(parent))

	got := make(chan any, 1)
	gm.Run("child", func(ctx context.Context) error {
		<-ctx.Done()
		got <- ctx.Value(key{})
		return nil
	})
	cancel()

	assert.Equal(t, "value", <-got)
	require.NoError(t, gm.Stop(context.Background()))
}

func TestGoroutineManager_StopBeforeInit(t *testing.T) {
	t.Parallel()
	gm := NewGoroutineManager()
	require.NoError(t, gm.Stop(context.Background()))

	require.NoError(t, gm.Init(context.Background()))
	ran := make(chan struct{})
	gm.Run("after", func(context.Context) error {
		close(ran)
		return nil
	})
	<-ran
	require.NoError(t, gm.Stop(context.Background()))
}

func TestGoroutineManager_ConcurrentRunAndStop(t *testing.T) {
	t.Parallel()
	gm, _ := started(t)

	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			gm.Run("racer", func(ctx context.Context) error {
				<-ctx.Done()
				return nil
			})
		})
	}
	wg.Go(func() { assert.NoError(t, gm.Stop(context.Background())) })
	wg.Wait()
	require.NoError(t, gm.Stop(context.Background()))
}

func TestGoroutineManager_IsAResource(t *testing.T) {
	t.Parallel()
	gm := NewGoroutineManager()
	assert.Equal(t, "goroutine_manager", gm.Name())
	require.NoError(t, gm.OK(context.Background()))

	rm := resource.NewResourceManager([]resource.Resource{gm})
	ctx := context.Background()
	require.NoError(t, rm.Init(ctx))
	require.NoError(t, rm.Start(ctx))
	stopped := make(chan struct{})
	gm.Run("worker", func(ctx context.Context) error {
		<-ctx.Done()
		close(stopped)
		return nil
	})
	require.NoError(t, rm.Stop(ctx))
	<-stopped
}
