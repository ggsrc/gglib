package wpgx

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/stumble/wpgx"
)

func newWPGXPool(ctx context.Context, c *wpgx.Config, configOpts ...ConfigOption) (*wpgx.Pool, error) {
	for _, opt := range configOpts {
		opt(c)
	}
	log.Ctx(ctx).Warn().Msgf("WPGX Config: %+v", &c)
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
