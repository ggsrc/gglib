package cache

import "github.com/kelseyhightower/envconfig"

// Option configures a Cache.
type Option func(*Cache)

// WithAppName sets the app name that namespaces the cache keys.
func WithAppName(appName string) Option {
	return func(c *Cache) {
		c.appName = appName
	}
}

// WithRedisConfig sets the Redis configuration.
func WithRedisConfig(redisCfg *RedisConfig) Option {
	return func(c *Cache) {
		c.redisConfig = redisCfg
	}
}

// WithDCacheConfig sets the dcache configuration.
func WithDCacheConfig(dcacheCfg *DCacheConfig) Option {
	return func(c *Cache) {
		c.dCacheConfig = dcacheCfg
	}
}

// WithRedisEnvPrefix reads the Redis configuration from environment variables
// with the given prefix. It panics if they cannot be parsed.
func WithRedisEnvPrefix(envPrefix string) Option {
	return func(c *Cache) {
		redisCfg := RedisConfig{}
		envconfig.MustProcess(envPrefix, &redisCfg)
		c.redisConfig = &redisCfg
	}
}

// WithDCacheEnvPrefix reads the dcache configuration from environment
// variables with the given prefix. It panics if they cannot be parsed.
func WithDCacheEnvPrefix(envPrefix string) Option {
	return func(c *Cache) {
		dcacheCfg := DCacheConfig{}
		envconfig.MustProcess(envPrefix, &dcacheCfg)
		c.dCacheConfig = &dcacheCfg
	}
}
