package wpgx

import (
	"context"
	"errors"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/stumble/wpgx"
)

// WPGX is a resource that owns a wpgx.Pool.
type WPGX struct {
	initialized   bool
	pool          *wpgx.Pool
	once          sync.Once
	beforeAcquire func(context.Context, *pgx.Conn) bool
	config        *wpgx.Config
}

// NewWPGXWithDefaultEnvPrefix returns a WPGX configured from POSTGRES_*
// environment variables.
func NewWPGXWithDefaultEnvPrefix() *WPGX {
	cfg := wpgx.ConfigFromEnvPrefix("postgres")
	return NewWPGXWithOptions(WithConfig(cfg))
}

// NewWPGXWithOptions returns a WPGX configured by opts. It panics if no
// configuration is given.
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

// Name returns "wpgx".
func (w *WPGX) Name() string {
	return "wpgx"
}

// Init creates the pool and pings the primary and every replica.
func (w *WPGX) Init(ctx context.Context) error {
	var err error
	w.once.Do(func() {
		w.pool, err = w.newWPGXPool(ctx)
	})
	w.initialized = true
	return err
}

// Start returns an error if Init has not been called.
func (w *WPGX) Start(ctx context.Context) error {
	if !w.initialized {
		return errors.New("wpgx not initialized")
	}
	return nil
}

// Stop closes the pool.
func (w *WPGX) Stop(ctx context.Context) error {
	if w.pool != nil {
		w.pool.Close()
	}
	return nil
}

// OK pings the primary.
func (w *WPGX) OK(ctx context.Context) error {
	if w.pool != nil {
		return w.pool.Ping(ctx)
	}
	return nil
}

// GetPool returns the pool, or nil before Init.
func (w *WPGX) GetPool() *wpgx.Pool {
	return w.pool
}
