// Package goroutine runs long-lived background goroutines as a
// [resource.Resource], so that they are canceled and awaited when the
// service shuts down.
//
// A [GoroutineManager] hands every goroutine started with
// [GoroutineManager.Run] a context that [GoroutineManager.Stop] cancels, then
// Stop waits for the goroutines to return. Each goroutine recovers its own
// panics (see [github.com/ggsrc/gglib/safego]), so one failing worker does not
// crash the process.
package goroutine

import (
	"context"
	"errors"
	"sync"

	"github.com/ggsrc/gglib/resource"
	"github.com/ggsrc/gglib/safego"
	"github.com/ggsrc/gglib/zerolog/log"
)

var _ resource.Resource = (*GoroutineManager)(nil)

// GoroutineManager starts named goroutines and stops them together. It is a
// [resource.Resource]: register it with a resource.ResourceManager (or the app
// package) so that its goroutines are stopped with the other resources.
//
// The lifecycle is Init, then any number of Run calls, then Stop. All methods
// are safe for concurrent use.
type GoroutineManager struct {
	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	stopped bool
	wg      sync.WaitGroup
}

// NewGoroutineManager creates a new GoroutineManager.
// The manager must be initialized with Init before calling Run or Stop.
// Typical lifecycle: Init → Start → Run (multiple times) → Stop.
func NewGoroutineManager() *GoroutineManager {
	return &GoroutineManager{}
}

// Name returns "goroutine_manager".
func (g *GoroutineManager) Name() string {
	return "goroutine_manager"
}

// Init derives the context that goroutines started with Run receive. That
// context keeps the values of ctx and is canceled when ctx is canceled or
// Stop is called, so pass a context that lives as long as the service, not
// one with an initialization deadline. Init never fails; calling it again has
// no effect.
func (g *GoroutineManager) Init(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.ctx == nil {
		g.ctx, g.cancel = context.WithCancel(ctx)
	}
	return nil
}

// Start does nothing; goroutines start when Run is called.
func (g *GoroutineManager) Start(context.Context) error {
	return nil
}

// Stop cancels the context of every goroutine started with Run and waits
// until they have all returned. If ctx ends first, Stop returns ctx.Err();
// the goroutines keep running until they observe the cancellation.
//
// Stop before Init does nothing and returns nil. After Stop, Run no longer
// starts goroutines. Calling Stop again waits again.
func (g *GoroutineManager) Stop(ctx context.Context) error {
	g.mu.Lock()
	if g.cancel != nil {
		g.stopped = true
		g.cancel()
	}
	g.mu.Unlock()

	done := make(chan struct{})
	go func() {
		g.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// OK always returns nil: a failing goroutine is logged, not reported as
// unhealthy.
func (g *GoroutineManager) OK(context.Context) error {
	return nil
}

// Run starts f in a new goroutine and returns immediately. f receives the
// manager's context and should return when that context is canceled.
//
// If f returns an error, the error is logged with name. If f panics, the
// panic is recovered and logged with its stack; the process keeps running.
//
// Run panics if Init has not been called. After Stop, Run logs an error and
// does not start f.
func (g *GoroutineManager) Run(name string, f func(ctx context.Context) error) {
	g.mu.Lock()
	ctx := g.ctx
	if ctx == nil {
		g.mu.Unlock()
		log.Panic().Msgf("GoroutineManager run called before init")
		return
	}
	if g.stopped {
		g.mu.Unlock()
		log.Ctx(ctx).Error().Str("goroutine", name).
			Msgf("GoroutineManager [%s] not started: the manager is stopped", name)
		return
	}
	// Add under the lock, so that it never races with the Wait in Stop.
	g.wg.Add(1)
	g.mu.Unlock()

	go func() {
		defer g.wg.Done()
		err := safego.Run(ctx, "goroutine "+name, func() error { return f(ctx) })
		// A panic was logged by safego.Run already.
		if err != nil && !errors.Is(err, safego.ErrPanic) {
			log.Ctx(ctx).Err(err).Str("goroutine", name).Msgf("goroutine [%s] error", name)
		}
	}()
}
