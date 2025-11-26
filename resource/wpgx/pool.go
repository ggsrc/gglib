package wpgx

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/stumble/wpgx"
)

func (w *WPGX) newWPGXPool(ctx context.Context) (*wpgx.Pool, error) {
	c := w.config
	if w.beforeAcquire != nil {
		c.BeforeAcquire = w.beforeAcquire
	}
	log.Ctx(ctx).Warn().Msgf("WPGX Config: %+v", c)
	pool, err := wpgx.NewPool(ctx, c)
	if err != nil {
		return nil, err
	}
	if err = pool.Ping(ctx); err != nil {
		return nil, err
	}
	log.Ctx(ctx).Warn().Msg("primary pool is ready")
	for name, readPool := range pool.ReplicaPools() {
		if err = readPool.Ping(ctx); err != nil {
			return nil, err
		}
		log.Ctx(ctx).Warn().Msgf("Read replica %s is ready", name)
	}
	return pool, nil
}
