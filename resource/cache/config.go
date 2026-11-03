package cache

import "time"

// RedisConfig configures the Redis client. The client runs in one of three
// modes: standalone (Host and Port), cluster (IsClusterMode, ClusterAddrs) or
// Sentinel (IsFailover; Host and Port address a Sentinel, which reports the
// primary named MasterName). Cluster and Sentinel cannot both be set.
type RedisConfig struct {
	Host                string        `default:"127.0.0.1"`
	Port                int           `default:"6379"`
	Password            string        `default:""          mask:"fixed"`
	IsFailover          bool          `default:"false"`
	MasterName          string        `default:"master"`
	IsClusterMode       bool          `default:"false"`
	ClusterAddrs        []string      `default:""`
	ClusterMaxRedirects int           `default:"3"`
	ReadTimeout         time.Duration `default:"3s"`
	PoolSize            int           `default:"50"`
	DisableIdentity     bool          `default:"false"`

	// TLSEnabled turns on TLS. The server certificate is verified against
	// TLSCACert (PEM) or the system roots, for the name in TLSServerName or,
	// when that is empty, the host the client dials.
	TLSEnabled            bool   `default:"false"`
	TLSServerName         string `default:""`
	TLSInsecureSkipVerify bool   `default:"false"`
	TLSCACert             string `default:""          mask:"fixed"`
}

// DCacheConfig configures the dcache layer: an in-memory cache of
// InMemCacheSize bytes in front of Redis, kept consistent across instances
// through Redis pub/sub.
type DCacheConfig struct {
	EnableStats    bool `default:"true"`
	EnableTrace    bool `default:"true"`
	InMemCacheSize int  `default:"52428800"` // 50 MiB
}
