package cache

import (
	"github.com/coocood/freecache"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/stumble/dcache"
)

func newDCacheWithConfig(appName string, c *DCacheConfig, redisConn redis.UniversalClient) (*dcache.DCache, error) {
	log.Warn().Msgf("DCache Config: %+v", c)
	return dcache.NewDCache(
		appName,
		redisConn,
		freecache.NewCache(c.InMemCacheSize),
		c.ReadInterval,
		c.EnableStats,
		c.EnableTrace)
}
