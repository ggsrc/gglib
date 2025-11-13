package cache

import "github.com/kelseyhightower/envconfig"

type Option func(*Cache)

func WithAppName(appName string) Option {
	return func(c *Cache) {
		c.appName = appName
	}
}

func WithRedisConfig(redisCfg *RedisConfig) Option {
	return func(c *Cache) {
		c.redisConfig = redisCfg
	}
}

func WithDCacheConfig(dcacheCfg *DCacheConfig) Option {
	return func(c *Cache) {
		c.dCacheConfig = dcacheCfg
	}
}

func WithRedisEnvPrefix(envPrefix string) Option {
	return func(c *Cache) {
		redisCfg := RedisConfig{}
		envconfig.MustProcess(envPrefix, &redisCfg)
		c.redisConfig = &redisCfg
	}
}

func WithDCacheEnvPrefix(envPrefix string) Option {
	return func(c *Cache) {
		dcacheCfg := DCacheConfig{}
		envconfig.MustProcess(envPrefix, &dcacheCfg)
		c.dCacheConfig = &dcacheCfg
	}
}
