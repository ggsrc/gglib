package cache

import (
	"github.com/coocood/freecache"
	"github.com/kelseyhightower/envconfig"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/stumble/dcache"
)

func newDCache(appName, dcacheEnvPrefix string, redisConn redis.UniversalClient) (*dcache.DCache, error) {
	c := DCacheConfig{}
	envconfig.MustProcess(dcacheEnvPrefix, &c)
	log.Warn().Msgf("DCache Config: %+v", c)
	return dcache.NewDCache(
		appName,
		redisConn,
		freecache.NewCache(c.InMemCacheSize),
		c.ReadInterval,
		c.EnableStats,
		c.EnableTrace)
}
