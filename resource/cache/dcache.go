package cache

import (
	"fmt"

	"github.com/coocood/freecache"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/stumble/dcache"
)

// newDCache builds the dcache layer on top of client. Redis single-flight
// stays off (dcache's default), so no read interval is needed.
func newDCache(appName string, client redis.UniversalClient, cfg *DCacheConfig) (*dcache.DCache, error) {
	log.Info().Msgf("dcache config: %+v", *cfg)
	d, err := dcache.NewDCache(
		appName,
		client,
		freecache.NewCache(cfg.InMemCacheSize),
		0,
		cfg.EnableStats,
		cfg.EnableTrace)
	if err != nil {
		return nil, fmt.Errorf("cache: create dcache: %w", err)
	}
	return d, nil
}
