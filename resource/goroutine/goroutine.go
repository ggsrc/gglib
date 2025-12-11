package goroutine

import (
	"context"
	"sync"

	"github.com/ggsrc/gglib/zerolog/log"
)

type GoroutineManager struct {
	ctx    context.Context
	cancel func()
	wg     sync.WaitGroup
}

func (g *GoroutineManager) Name() string {
	return "goroutine_manager"
}

func (g *GoroutineManager) Init(ctx context.Context) error {
	g.ctx, g.cancel = context.WithCancel(ctx)
	return nil
}

func (g *GoroutineManager) Start(ctx context.Context) error {
	return nil
}

func (g *GoroutineManager) Stop(ctx context.Context) error {
	if g.cancel != nil {
		g.cancel()
	}

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

func (g *GoroutineManager) OK(ctx context.Context) error {
	return nil
}

func NewGoroutineManager() *GoroutineManager {
	return &GoroutineManager{}
}

func (g *GoroutineManager) Run(name string, f func(ctx context.Context) error) {
	if g.ctx == nil {
		log.Panic().Msgf("GoroutineManager run called before init")
	}
	g.wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Ctx(g.ctx).Error().Interface("panic", r).Msgf("GoroutineManager [%s] panicked", name)
			}
			g.wg.Done()
		}()

		err := f(g.ctx)
		if err != nil {
			log.Err(err).Msgf("goroutine [%s] error", name)
		}
	}()
}
