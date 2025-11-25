package cache

import "time"

type RedisConfig struct {
	Host                string        `default:"127.0.0.1"`
	Port                int           `default:"6379"`
	Password            string        `default:""          mask:"fixed"`
	IsFailover          bool          `default:"false"`
	IsElastiCache       bool          `default:"false"`
	IsClusterMode       bool          `default:"false"`
	ClusterAddrs        []string      `default:""`
	ClusterMaxRedirects int           `default:"3"`
	ReadTimeout         time.Duration `default:"3s"`
	PoolSize            int           `default:"50"`
}

type DCacheConfig struct {
	ReadInterval   time.Duration `default:"500ms"`
	EnableStats    bool          `default:"true"`
	EnableTrace    bool          `default:"true"`
	InMemCacheSize int           `default:"52428800"` // base unit in byte: 50 * 1024 * 1024 = 52428800 -> 50MB
}
