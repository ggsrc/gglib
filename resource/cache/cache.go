package cache

import (
	"context"

	"github.com/kelseyhightower/envconfig"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"github.com/stumble/dcache"
)

type Cache struct {
	initialized  bool
	appName      string
	redisClient  redis.UniversalClient
	dCache       *dcache.DCache
	redisConfig  *RedisConfig
	dCacheConfig *DCacheConfig
}

func NewCacheWithDefaultEnvPrefix(appName string) *Cache {
	return NewCache(appName, "redis", "dcache")
}

func NewCache(appName string, redisEnvPrefix string, dcacheEnvPrefix string) *Cache {
	redisCfg := RedisConfig{}
	envconfig.MustProcess(redisEnvPrefix, &redisCfg)
	dcacheCfg := DCacheConfig{}
	envconfig.MustProcess(dcacheEnvPrefix, &dcacheCfg)
	return NewCacheWithConfig(appName, &redisCfg, &dcacheCfg)
}

func NewCacheWithConfig(appName string, redisCfg *RedisConfig, dcacheCfg *DCacheConfig) *Cache {
	if redisCfg == nil || dcacheCfg == nil {
		panic("cfg cannot be nil")
	}
	return &Cache{
		appName:      appName,
		redisConfig:  redisCfg,
		dCacheConfig: dcacheCfg,
	}
}

func NewCacheWithOptions(opts ...Option) *Cache {
	c := &Cache{}
	for _, opt := range opts {
		opt(c)
	}
	if c.redisConfig == nil || c.dCacheConfig == nil {
		panic(
			"redisConfig and dcacheConfig cannot be nil, use WithRedisConfig/WithRedisEnvPrefix and WithDCacheConfig/WithDCacheEnvPrefix",
		)
	}
	if c.appName == "" {
		panic("appName cannot be empty, use WithAppName")
	}
	return c
}

func (c *Cache) Name() string {
	return "cache"
}

func (c *Cache) Init(ctx context.Context) error {
	c.redisClient = c.newRedisClientWithConfig()
	dCache, err := c.newDCacheWithConfig()
	if err != nil {
		return err
	}
	c.dCache = dCache
	c.initialized = true
	return nil
}

func (c *Cache) Start(ctx context.Context) error {
	if !c.initialized {
		return errors.New("cache not initialized")
	}
	return nil
}

func (c *Cache) Stop(ctx context.Context) error {
	if c.dCache != nil {
		c.dCache.Close()
	}
	if c.redisClient != nil {
		err := c.redisClient.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Cache) OK(ctx context.Context) error {
	if c.dCache != nil {
		err := c.dCache.Ping(ctx)
		if err != nil {
			return errors.Wrap(err, "dcache health check failed")
		}
	}
	return nil
}

func (c *Cache) GetRedisClient() redis.UniversalClient {
	return c.redisClient
}

func (c *Cache) GetDCache() *dcache.DCache {
	return c.dCache
}
