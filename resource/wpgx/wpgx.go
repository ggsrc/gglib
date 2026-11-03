package wpgx

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"
	"github.com/stumble/wpgx"
)

// DefaultInitTimeout bounds Init when its context has no deadline, so that an
// unreachable database cannot hang start-up.
const DefaultInitTimeout = 30 * time.Second

var errNotInitialized = errors.New("wpgx: not initialized")

// WPGX is a resource that owns a wpgx.Pool.
type WPGX struct {
	config        *wpgx.Config
	beforeAcquire func(context.Context, *pgx.Conn) bool

	mu   sync.Mutex
	pool *wpgx.Pool
}

// NewWPGXWithDefaultEnvPrefix returns a WPGX configured from POSTGRES_*
// environment variables. It exits the process if they are invalid (see
// wpgx.ConfigFromEnvPrefix).
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
		panic("wpgx: config cannot be nil, use WithConfig")
	}
	return w
}

// Name returns "wpgx".
func (w *WPGX) Name() string {
	return "wpgx"
}

// Init creates the pool and pings the primary and every replica. When ctx has
// no deadline, Init gives up after DefaultInitTimeout. On failure it returns
// the error, closes the pool, and can be called again.
func (w *WPGX) Init(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.pool != nil {
		return errors.New("wpgx: already initialized")
	}
	ctx, cancel := withInitTimeout(ctx)
	defer cancel()

	// Work on a copy: the caller's config is left as it was.
	cfg := *w.config
	if w.beforeAcquire != nil {
		cfg.BeforeAcquire = w.beforeAcquire
	}
	log.Info().Str("config", cfg.String()).Msg("wpgx: creating pool")

	pool, err := wpgx.NewPool(ctx, &cfg)
	if err != nil {
		return fmt.Errorf("wpgx: create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return fmt.Errorf("wpgx: ping: %w", err)
	}
	log.Info().Int("replicas", len(pool.ReplicaPools())).Msg("wpgx: pool is ready")
	w.pool = pool
	return nil
}

func withInitTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, DefaultInitTimeout)
}

// Start returns an error if Init has not succeeded.
func (w *WPGX) Start(context.Context) error {
	if w.GetPool() == nil {
		return errNotInitialized
	}
	return nil
}

// Stop closes the pool, waiting for acquired connections to be released until
// ctx is done. It is safe to call more than once.
func (w *WPGX) Stop(ctx context.Context) error {
	w.mu.Lock()
	pool := w.pool
	w.pool = nil
	w.mu.Unlock()
	if pool == nil {
		return nil
	}
	done := make(chan struct{})
	go func() {
		pool.Close()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wpgx: close pool: %w", ctx.Err())
	}
}

// OK pings the primary. A replica outage does not make the service unhealthy:
// queries routed to that replica fail, the rest keep working.
func (w *WPGX) OK(ctx context.Context) error {
	pool := w.GetPool()
	if pool == nil {
		return errNotInitialized
	}
	return pool.PingPrimary(ctx)
}

// GetPool returns the pool, or nil before Init and after Stop.
func (w *WPGX) GetPool() *wpgx.Pool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.pool
}
