package cache

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startRedis(t *testing.T) *RedisConfig {
	t.Helper()
	mr := miniredis.RunT(t)
	port, err := strconv.Atoi(mr.Port())
	require.NoError(t, err)
	return &RedisConfig{Host: mr.Host(), Port: port, ReadTimeout: time.Second, PoolSize: 2, MasterName: "master"}
}

// closedPort returns a port nothing listens on.
func closedPort(t *testing.T) int {
	t.Helper()
	var lc net.ListenConfig
	lis, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := lis.Addr().(*net.TCPAddr).Port
	require.NoError(t, lis.Close())
	return port
}

func TestRedisLifecycle(t *testing.T) {
	r := NewRedisWithConfig(startRedis(t))
	ctx := t.Context()

	assert.Equal(t, "redis", r.Name())
	require.ErrorIs(t, r.OK(ctx), errNotInitialized, "OK before Init")
	require.ErrorIs(t, r.Start(ctx), errNotInitialized, "Start before Init")
	assert.Nil(t, r.Client())

	require.NoError(t, r.Init(ctx))
	require.Error(t, r.Init(ctx), "second Init")
	require.NoError(t, r.Start(ctx))
	require.NoError(t, r.OK(ctx))

	client := r.Client()
	require.NotNil(t, client)
	require.NoError(t, client.Set(ctx, "k", "v", 0).Err())
	got, err := client.Get(ctx, "k").Result()
	require.NoError(t, err)
	assert.Equal(t, "v", got)

	require.NoError(t, r.Stop(ctx))
	require.NoError(t, r.Stop(ctx), "Stop is idempotent")
	assert.Nil(t, r.Client())
	require.ErrorIs(t, r.OK(ctx), errNotInitialized, "OK after Stop")
}

func TestRedisOKReportsFailure(t *testing.T) {
	mr := miniredis.RunT(t)
	port, err := strconv.Atoi(mr.Port())
	require.NoError(t, err)
	r := NewRedisWithConfig(&RedisConfig{Host: mr.Host(), Port: port, ReadTimeout: time.Second})
	require.NoError(t, r.Init(t.Context()))
	t.Cleanup(func() { _ = r.Stop(context.Background()) })

	mr.SetError("down")
	err = r.OK(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redis health check failed")
}

// Init used to call log.Fatal (exit the process) when Redis was unreachable.
func TestInitReturnsErrorWhenRedisIsDown(t *testing.T) {
	cfg := &RedisConfig{Host: "127.0.0.1", Port: closedPort(t), ReadTimeout: time.Second}

	r := NewRedisWithConfig(cfg)
	err := r.Init(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connect to redis")
	assert.Nil(t, r.Client())

	c := NewCacheWithConfig("app", cfg, &DCacheConfig{InMemCacheSize: 1 << 20})
	require.Error(t, c.Init(t.Context()))
	assert.Nil(t, c.GetRedisClient())
	assert.Nil(t, c.GetDCache())
}

func TestInitReturnsInstrumentationError(t *testing.T) {
	orig := instrument
	instrument = func(redis.UniversalClient) error { return errors.New("boom") }
	t.Cleanup(func() { instrument = orig })

	err := NewRedisWithConfig(startRedis(t)).Init(t.Context())
	require.EqualError(t, err, "boom")
}

func TestInitRejectsConflictingModesAndBadCA(t *testing.T) {
	err := NewRedisWithConfig(&RedisConfig{IsClusterMode: true, IsFailover: true}).Init(t.Context())
	require.ErrorContains(t, err, "cannot both be set")

	err = NewRedisWithConfig(&RedisConfig{TLSEnabled: true, TLSCACert: "not a certificate"}).Init(t.Context())
	require.ErrorContains(t, err, "no valid PEM certificate")
}

func TestNewRedisClientModes(t *testing.T) {
	client, err := newRedisClient(&RedisConfig{Host: "h", Port: 1})
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	standalone, ok := client.(*redis.Client)
	require.True(t, ok)
	assert.Equal(t, "h:1", standalone.Options().Addr)

	client, err = newRedisClient(&RedisConfig{IsClusterMode: true, ClusterAddrs: []string{"a:1", "b:2"}})
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	cluster, ok := client.(*redis.ClusterClient)
	require.True(t, ok)
	assert.Equal(t, []string{"a:1", "b:2"}, cluster.Options().Addrs)

	client, err = newRedisClient(&RedisConfig{IsFailover: true, Host: "sentinel.example", Port: 26379, MasterName: "mymaster"})
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	failover, ok := client.(*redis.Client)
	require.True(t, ok)
	assert.Equal(t, "FailoverClient", failover.Options().Addr, "go-redis marks failover clients this way")
}

// The ElastiCache mode used to replace the TLS settings with
// InsecureSkipVerify: true. It is gone; TLSServerName verifies a certificate
// behind a CNAME instead.
func TestBuildTLSConfig(t *testing.T) {
	cfg, err := buildTLSConfig(&RedisConfig{})
	require.NoError(t, err)
	assert.Nil(t, cfg, "TLS off")

	cfg, err = buildTLSConfig(&RedisConfig{TLSEnabled: true, TLSServerName: "real.endpoint"})
	require.NoError(t, err)
	assert.Equal(t, "real.endpoint", cfg.ServerName)
	assert.False(t, cfg.InsecureSkipVerify)
	assert.Nil(t, cfg.RootCAs, "system roots")
	assert.Equal(t, uint16(0x0303), cfg.MinVersion, "TLS 1.2")

	cfg, err = buildTLSConfig(&RedisConfig{TLSEnabled: true, TLSCACert: testCAPEM(t), TLSInsecureSkipVerify: true})
	require.NoError(t, err)
	assert.NotNil(t, cfg.RootCAs)
	assert.True(t, cfg.InsecureSkipVerify, "explicit opt-in is passed through")

	_, err = buildTLSConfig(&RedisConfig{TLSEnabled: true, TLSCACert: "garbage"})
	require.Error(t, err)
}

func testCAPEM(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func TestCacheLifecycle(t *testing.T) {
	c := NewCacheWithConfig("app", startRedis(t), &DCacheConfig{InMemCacheSize: 1 << 20, EnableStats: true, EnableTrace: true})
	ctx := t.Context()

	assert.Equal(t, "cache", c.Name())
	require.ErrorIs(t, c.OK(ctx), errNotInitialized)
	require.ErrorIs(t, c.Start(ctx), errNotInitialized)

	require.NoError(t, c.Init(ctx))
	require.Error(t, c.Init(ctx), "second Init")
	require.NoError(t, c.Start(ctx))
	require.NoError(t, c.OK(ctx))
	require.NotNil(t, c.GetRedisClient())
	require.NotNil(t, c.GetDCache())

	require.NoError(t, c.Stop(ctx))
	require.NoError(t, c.Stop(ctx), "Stop is idempotent")
	assert.Nil(t, c.GetRedisClient())
	assert.Nil(t, c.GetDCache())
}

func TestCacheOKReportsFailure(t *testing.T) {
	mr := miniredis.RunT(t)
	port, err := strconv.Atoi(mr.Port())
	require.NoError(t, err)
	c := NewCacheWithConfig("app", &RedisConfig{Host: mr.Host(), Port: port, ReadTimeout: time.Second}, &DCacheConfig{InMemCacheSize: 1 << 20})
	require.NoError(t, c.Init(t.Context()))
	t.Cleanup(func() { _ = c.Stop(context.Background()) })

	mr.SetError("down")
	require.ErrorContains(t, c.OK(t.Context()), "dcache health check failed")
}

// Two caches in one process both work; dcache v0.5.0 shares its collectors.
func TestTwoCaches(t *testing.T) {
	cfg := startRedis(t)
	a := NewCacheWithConfig("a", cfg, &DCacheConfig{InMemCacheSize: 1 << 20, EnableStats: true})
	b := NewCacheWithConfig("b", cfg, &DCacheConfig{InMemCacheSize: 1 << 20, EnableStats: true})
	require.NoError(t, a.Init(t.Context()))
	require.NoError(t, b.Init(t.Context()))
	require.NoError(t, b.Stop(t.Context()))
	require.NoError(t, a.OK(t.Context()))
	require.NoError(t, a.Stop(t.Context()))
}

func TestConstructors(t *testing.T) {
	t.Setenv("REDIS_HOST", "redis.example")
	t.Setenv("REDIS_PORT", "6380")
	t.Setenv("DCACHE_INMEMCACHESIZE", "1024")
	t.Setenv("REDIS_ISFAILOVER", "")
	require.NoError(t, os.Unsetenv("REDIS_ISFAILOVER"))

	r := NewRedisWithDefaultEnvPrefix()
	assert.Equal(t, "redis.example", r.cfg.Host)
	assert.Equal(t, 6380, r.cfg.Port)
	assert.Equal(t, "master", r.cfg.MasterName, "default")

	c := NewCacheWithDefaultEnvPrefix("app")
	assert.Equal(t, "app", c.appName)
	assert.Equal(t, "redis.example", c.redisConfig.Host)
	assert.Equal(t, 1024, c.dCacheConfig.InMemCacheSize)
	assert.True(t, c.dCacheConfig.EnableStats, "default")

	c = NewCacheWithOptions(WithAppName("x"), WithRedisEnvPrefix("redis"), WithDCacheEnvPrefix("dcache"))
	assert.Equal(t, "x", c.appName)
	assert.Equal(t, 6380, c.redisConfig.Port)

	rc, dc := &RedisConfig{}, &DCacheConfig{}
	c = NewCacheWithOptions(WithAppName("y"), WithRedisConfig(rc), WithDCacheConfig(dc))
	assert.Same(t, rc, c.redisConfig)
	assert.Same(t, dc, c.dCacheConfig)

	assert.Panics(t, func() { NewRedisWithConfig(nil) })
	assert.Panics(t, func() { NewCacheWithConfig("a", nil, dc) })
	assert.Panics(t, func() { NewCacheWithOptions(WithAppName("a")) }, "missing configs")
	assert.Panics(t, func() { NewCacheWithOptions(WithRedisConfig(rc), WithDCacheConfig(dc)) }, "missing app name")

	t.Setenv("REDIS_PORT", "not-a-number")
	assert.Panics(t, func() { NewRedis("redis") })
}

func TestConfigLogMasksSecrets(t *testing.T) {
	var buf bytes.Buffer
	orig := log.Logger
	log.Logger = zerolog.New(&buf)
	t.Cleanup(func() { log.Logger = orig })

	cfg := startRedis(t)
	cfg.Password = "hunter2"
	cfg.TLSCACert = "-----BEGIN CERTIFICATE-----secret-ish"
	// miniredis has no password set, so AUTH fails; the log line comes first.
	_ = NewRedisWithConfig(cfg).Init(t.Context())

	out := buf.String()
	assert.Contains(t, out, "redis config")
	assert.NotContains(t, out, "hunter2")
	assert.NotContains(t, out, "BEGIN CERTIFICATE")
}
