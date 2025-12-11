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
	g.cancel()
	g.wg.Wait()
	return nil
}

func (g *GoroutineManager) OK(ctx context.Context) error {
	return nil
}

func NewGoroutineManager() *GoroutineManager {
	return &GoroutineManager{}
}

func (g *GoroutineManager) Run(name string, f func(ctx context.Context) error) {
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
			log.Ctx(g.ctx).Err(err).Msgf("goroutine [%s] error", name)
		}
	}()
}
