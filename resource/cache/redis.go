package cache

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	mask "github.com/showa-93/go-mask"
)

// pingTimeout bounds the connectivity check in Init.
const pingTimeout = 10 * time.Second

// Redis is a resource that owns a Redis client, for services that need Redis
// but not the in-memory dcache layer.
type Redis struct {
	cfg *RedisConfig

	mu     sync.Mutex
	client redis.UniversalClient
}

// NewRedisWithDefaultEnvPrefix returns a Redis configured from REDIS_*
// environment variables.
func NewRedisWithDefaultEnvPrefix() *Redis {
	return NewRedis("redis")
}

// NewRedis returns a Redis configured from environment variables with the
// given prefix. It panics if they cannot be parsed.
func NewRedis(envPrefix string) *Redis {
	cfg := RedisConfig{}
	envconfig.MustProcess(envPrefix, &cfg)
	return NewRedisWithConfig(&cfg)
}

// NewRedisWithConfig returns a Redis with the given configuration. It panics
// if cfg is nil.
func NewRedisWithConfig(cfg *RedisConfig) *Redis {
	if cfg == nil {
		panic("cache: RedisConfig cannot be nil")
	}
	return &Redis{cfg: cfg}
}

// Name returns "redis".
func (r *Redis) Name() string { return "redis" }

// Init creates the client, instruments it with OpenTelemetry and pings Redis.
// It returns an error, and leaves no client open, if any step fails.
func (r *Redis) Init(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.client != nil {
		return errors.New("cache: redis already initialized")
	}
	client, err := connectRedis(ctx, r.cfg)
	if err != nil {
		return err
	}
	r.client = client
	return nil
}

// Start returns an error if Init has not succeeded.
func (r *Redis) Start(context.Context) error {
	if r.Client() == nil {
		return errNotInitialized
	}
	return nil
}

// Stop closes the client. It is safe to call more than once.
func (r *Redis) Stop(context.Context) error {
	r.mu.Lock()
	client := r.client
	r.client = nil
	r.mu.Unlock()
	if client == nil {
		return nil
	}
	return client.Close()
}

// OK pings Redis. It returns an error before Init.
func (r *Redis) OK(ctx context.Context) error {
	client := r.Client()
	if client == nil {
		return errNotInitialized
	}
	if err := client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis health check failed: %w", err)
	}
	return nil
}

// Client returns the Redis client, or nil before Init and after Stop.
func (r *Redis) Client() redis.UniversalClient {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.client
}

var errNotInitialized = errors.New("cache: not initialized")

// connectRedis builds an instrumented client from cfg and pings it.
func connectRedis(ctx context.Context, cfg *RedisConfig) (redis.UniversalClient, error) {
	masker := mask.NewMasker()
	masker.RegisterMaskStringFunc(mask.MaskTypeFixed, masker.MaskFixedString)
	if masked, err := masker.Mask(cfg); err == nil {
		log.Info().Msgf("redis config: %+v", masked)
	}

	client, err := newRedisClient(cfg)
	if err != nil {
		return nil, err
	}
	if err := instrument(client); err != nil {
		_ = client.Close()
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("cache: connect to redis: %w", err)
	}
	return client, nil
}

// instrument adds OpenTelemetry tracing and metrics; it is a variable so that
// tests can make it fail.
var instrument = func(client redis.UniversalClient) error {
	if err := redisotel.InstrumentTracing(client); err != nil {
		return fmt.Errorf("cache: instrument redis tracing: %w", err)
	}
	if err := redisotel.InstrumentMetrics(client); err != nil {
		return fmt.Errorf("cache: instrument redis metrics: %w", err)
	}
	return nil
}

// newRedisClient builds a client for the mode cfg selects. It does not
// connect.
func newRedisClient(cfg *RedisConfig) (redis.UniversalClient, error) {
	if cfg.IsClusterMode && cfg.IsFailover {
		return nil, errors.New("cache: IsClusterMode and IsFailover cannot both be set")
	}
	tlsConfig, err := buildTLSConfig(cfg)
	if err != nil {
		return nil, err
	}

	switch {
	case cfg.IsClusterMode:
		return redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:           cfg.ClusterAddrs,
			MaxRedirects:    cfg.ClusterMaxRedirects,
			ReadTimeout:     cfg.ReadTimeout,
			PoolSize:        cfg.PoolSize,
			Password:        cfg.Password,
			DisableIdentity: cfg.DisableIdentity,
			TLSConfig:       tlsConfig,
		}), nil
	case cfg.IsFailover:
		// The host name goes to go-redis as is: the dialer tries every address
		// it resolves to and resolves it again on each dial, so Sentinel pods
		// that change IP are still found.
		return redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:      cfg.MasterName,
			SentinelAddrs:   []string{net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))},
			Password:        cfg.Password,
			PoolSize:        cfg.PoolSize,
			ReadTimeout:     cfg.ReadTimeout,
			DisableIdentity: cfg.DisableIdentity,
			TLSConfig:       tlsConfig,
		}), nil
	default:
		return redis.NewClient(&redis.Options{
			Addr:            net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
			ReadTimeout:     cfg.ReadTimeout,
			PoolSize:        cfg.PoolSize,
			Password:        cfg.Password,
			DisableIdentity: cfg.DisableIdentity,
			TLSConfig:       tlsConfig,
		}), nil
	}
}

// buildTLSConfig returns nil when TLS is off.
func buildTLSConfig(cfg *RedisConfig) (*tls.Config, error) {
	if !cfg.TLSEnabled {
		return nil, nil //nolint:nilnil // nil means plain TCP to go-redis
	}
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: cfg.TLSServerName,
		// Opt-in for test setups; production configs verify certificates.
		InsecureSkipVerify: cfg.TLSInsecureSkipVerify, //nolint:gosec // explicit opt-in, off by default
	}
	if cfg.TLSCACert != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(cfg.TLSCACert)) {
			return nil, errors.New("cache: TLSCACert contains no valid PEM certificate")
		}
		tlsConfig.RootCAs = pool
	}
	return tlsConfig, nil
}
