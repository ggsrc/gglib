package wpgx

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/stumble/wpgx"
)

type Options func(w *WPGX)

func WithConfig(cfg *wpgx.Config) Options {
	return func(w *WPGX) {
		w.config = cfg
	}
}

func WithBeforeAcquire(f func(context.Context, *pgx.Conn) bool) Options {
	return func(w *WPGX) {
		w.beforeAcquire = f
	}
}
