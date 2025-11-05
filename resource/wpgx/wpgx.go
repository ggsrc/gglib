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
	config      *wpgx.Config
}

func NewWPGX(configOpts ...ConfigOption) *WPGX {
	cfg := wpgx.ConfigFromEnvPrefix("postgres")
	return NewWPGXWithConfig(cfg, configOpts...)
}

func NewWPGXWithConfig(cfg *wpgx.Config, configOpts ...ConfigOption) *WPGX {
	if cfg == nil {
		panic("cfg cannot be nil")
	}
	return &WPGX{
		configOpts: configOpts,
		config:     cfg,
	}
}

func (w *WPGX) Name() string {
	return "wpgx"
}

func (w *WPGX) Init(ctx context.Context) error {
	var err error
	w.once.Do(func() {
		w.pool, err = w.newWPGXPool(ctx)
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
