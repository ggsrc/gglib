package wpgx

import (
	"context"
	"errors"
	"sync"

	"github.com/stumble/wpgx"
)

type WPGX struct {
	initialized bool
	pool        *wpgx.Pool
	configOpts  []ConfigOption
	once        sync.Once
}

func NewWPGX(configOpts ...ConfigOption) *WPGX {
	return &WPGX{
		configOpts: configOpts,
	}
}

func (w *WPGX) Name() string {
	return "wpgx"
}

func (w *WPGX) Init(ctx context.Context) error {
	var err error
	w.once.Do(func() {
		w.pool, err = newWPGXPool(ctx, "postgres", w.configOpts...)
	})
	w.initialized = true
	return err
}

func (w *WPGX) Start(ctx context.Context) error {
	if !w.initialized {
		return errors.New("wpgx not initialized")
	}
	return nil
}

func (w *WPGX) Stop(ctx context.Context) error {
	if w.pool != nil {
		w.pool.Close()
	}
	return nil
}

func (w *WPGX) OK(ctx context.Context) error {
	if w.pool != nil {
		return w.pool.Ping(ctx)
	}
	return nil
}

func (w *WPGX) GetPool() *wpgx.Pool {
	return w.pool
}
