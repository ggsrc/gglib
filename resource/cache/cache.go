// Package cache provides Redis as a resource.Resource: Redis, a plain client,
// and Cache, a client plus a two-level cache (in-memory and Redis, built on
// github.com/stumble/dcache).
package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/kelseyhightower/envconfig"
	"github.com/redis/go-redis/v9"
	"github.com/stumble/dcache"
)

// Cache is a resource that owns a Redis client and a dcache.DCache built on
// it. Services that only need the client should use Redis.
type Cache struct {
	appName      string
	redisConfig  *RedisConfig
	dCacheConfig *DCacheConfig

	mu          sync.Mutex
	redisClient redis.UniversalClient
	dCache      *dcache.DCache
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
		panic("cache: RedisConfig and DCacheConfig cannot be nil")
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
		panic("cache: use WithRedisConfig or WithRedisEnvPrefix, and WithDCacheConfig or WithDCacheEnvPrefix")
	}
	if c.appName == "" {
		panic("cache: app name cannot be empty, use WithAppName")
	}
	return c
}

// Name returns "cache".
func (c *Cache) Name() string {
	return "cache"
}

// Init connects to Redis and creates the dcache. It returns an error, and
// leaves nothing open, if either step fails.
func (c *Cache) Init(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.redisClient != nil {
		return errors.New("cache: already initialized")
	}
	client, err := connectRedis(ctx, c.redisConfig)
	if err != nil {
		return err
	}
	d, err := newDCache(c.appName, client, c.dCacheConfig)
	if err != nil {
		_ = client.Close()
		return err
	}
	c.redisClient, c.dCache = client, d
	return nil
}

// Start returns an error if Init has not succeeded.
func (c *Cache) Start(context.Context) error {
	if c.GetRedisClient() == nil {
		return errNotInitialized
	}
	return nil
}

// Stop closes the dcache and then the Redis client. It is safe to call more
// than once.
func (c *Cache) Stop(context.Context) error {
	c.mu.Lock()
	d, client := c.dCache, c.redisClient
	c.dCache, c.redisClient = nil, nil
	c.mu.Unlock()
	if d != nil {
		d.Close()
	}
	if client != nil {
		return client.Close()
	}
	return nil
}

// OK pings Redis through the dcache. It returns an error before Init.
func (c *Cache) OK(ctx context.Context) error {
	d := c.GetDCache()
	if d == nil {
		return errNotInitialized
	}
	if err := d.Ping(ctx); err != nil {
		return fmt.Errorf("dcache health check failed: %w", err)
	}
	return nil
}

// GetRedisClient returns the Redis client, or nil before Init and after Stop.
func (c *Cache) GetRedisClient() redis.UniversalClient {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.redisClient
}

// GetDCache returns the dcache, or nil before Init and after Stop.
func (c *Cache) GetDCache() *dcache.DCache {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.dCache
}
