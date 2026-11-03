// Package wpgx provides a Postgres connection pool, built on
// github.com/stumble/wpgx, as a resource.Resource.
package wpgx

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/stumble/wpgx"
)

// Options configures a WPGX.
type Options func(w *WPGX)

// WithConfig sets the pool configuration. It is required. Init does not
// modify it.
func WithConfig(cfg *wpgx.Config) Options {
	return func(w *WPGX) {
		w.config = cfg
	}
}

// WithBeforeAcquire sets the BeforeAcquire hook of the primary pool.
func WithBeforeAcquire(f func(context.Context, *pgx.Conn) bool) Options {
	return func(w *WPGX) {
		w.beforeAcquire = f
	}
}
