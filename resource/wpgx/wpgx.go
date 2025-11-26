package wpgx

import (
	"context"
	"errors"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/stumble/wpgx"
)

type WPGX struct {
	initialized   bool
	pool          *wpgx.Pool
	once          sync.Once
	beforeAcquire func(context.Context, *pgx.Conn) bool
	config        *wpgx.Config
}

func NewWPGXWithDefaultEnvPrefix() *WPGX {
	cfg := wpgx.ConfigFromEnvPrefix("postgres")
	return NewWPGXWithOptions(WithConfig(cfg))
}

func NewWPGXWithOptions(opts ...Options) *WPGX {
	w := &WPGX{}
	for _, opt := range opts {
		opt(w)
	}
	if w.config == nil {
		panic("config cannot be nil")
	}
	return w
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
