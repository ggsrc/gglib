// Package cache provides a Redis client and a two-level cache (in-memory plus
// Redis, built on github.com/stumble/dcache) as a resource.Resource.
package cache

import (
	"context"
	"errors"
	"fmt"

	"github.com/kelseyhightower/envconfig"
	"github.com/redis/go-redis/v9"
	"github.com/stumble/dcache"
)

// Cache is a resource that owns a Redis client and a dcache.DCache.
type Cache struct {
	initialized  bool
	appName      string
	redisClient  redis.UniversalClient
	dCache       *dcache.DCache
	redisConfig  *RedisConfig
	dCacheConfig *DCacheConfig
}

// NewCacheWithDefaultEnvPrefix returns a Cache configured from REDIS_* and
// DCACHE_* environment variables. appName namespaces the cache keys.
func NewCacheWithDefaultEnvPrefix(appName string) *Cache {
	return NewCache(appName, "redis", "dcache")
}

// NewCache returns a Cache configured from environment variables with the
// given prefixes. It panics if they cannot be parsed.
func NewCache(appName string, redisEnvPrefix string, dcacheEnvPrefix string) *Cache {
	redisCfg := RedisConfig{}
	envconfig.MustProcess(redisEnvPrefix, &redisCfg)
	dcacheCfg := DCacheConfig{}
	envconfig.MustProcess(dcacheEnvPrefix, &dcacheCfg)
	return NewCacheWithConfig(appName, &redisCfg, &dcacheCfg)
}

// NewCacheWithConfig returns a Cache with the given configuration. It panics
// if either configuration is nil.
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

// NewCacheWithOptions returns a Cache configured by opts. It panics if the
// app name or either configuration is missing.
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

// Name returns "cache".
func (c *Cache) Name() string {
	return "cache"
}

// Init connects to Redis and creates the dcache.
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

// Start returns an error if Init has not been called.
func (c *Cache) Start(ctx context.Context) error {
	if !c.initialized {
		return errors.New("cache not initialized")
	}
	return nil
}

// Stop closes the dcache and then the Redis client.
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

// OK pings Redis through the dcache.
func (c *Cache) OK(ctx context.Context) error {
	if c.dCache != nil {
		err := c.dCache.Ping(ctx)
		if err != nil {
			return fmt.Errorf("dcache health check failed: %w", err)
		}
	}
	return nil
}

// GetRedisClient returns the Redis client, or nil before Init.
func (c *Cache) GetRedisClient() redis.UniversalClient {
	return c.redisClient
}

// GetDCache returns the dcache, or nil before Init.
func (c *Cache) GetDCache() *dcache.DCache {
	return c.dCache
}
