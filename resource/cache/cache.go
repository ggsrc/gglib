package cache

import (
	"context"

	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"github.com/stumble/dcache"
)

type Cache struct {
	appName     string
	redisClient redis.UniversalClient
	dCache      *dcache.DCache
}

func NewCache(appName string) *Cache {
	return &Cache{
		appName: appName,
	}
}

func (c *Cache) Name() string {
	return "cache"
}

func (c *Cache) Start(ctx context.Context) error {
	c.redisClient = newRedisClient("redis")
	dCache, err := newDCache(c.appName, "dcache", c.redisClient)
	if err != nil {
		return err
	}
	c.dCache = dCache
	return nil
}

func (c *Cache) Stop(ctx context.Context) error {
	if c.redisClient != nil {
		err := c.redisClient.Close()
		if err != nil {
			return err
		}
	}
	if c.dCache != nil {
		c.dCache.Close()
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
