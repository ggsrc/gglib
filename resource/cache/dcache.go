package cache

import (
	"github.com/coocood/freecache"
	"github.com/rs/zerolog/log"
	"github.com/stumble/dcache"
)

func (cache *Cache) newDCacheWithConfig() (*dcache.DCache, error) {
	c := cache.dCacheConfig
	log.Warn().Msgf("DCache Config: %+v", c)
	return dcache.NewDCache(
		cache.appName,
		cache.redisClient,
		freecache.NewCache(c.InMemCacheSize),
		c.ReadInterval,
		c.EnableStats,
		c.EnableTrace)
}
