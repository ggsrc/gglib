# resource/cache

```sh
go get github.com/ggsrc/gglib/resource/cache
```

Redis as a [resource](..), in two flavours:

- **`Redis`** — a Redis client, nothing else.
- **`Cache`** — the client plus a two-level cache,
  [dcache](https://github.com/stumble/dcache): an in-process memory cache in
  front of Redis.

Services that only need Redis should use `Redis` and save the memory the
in-process cache reserves (50 MiB by default).

## dcache in one paragraph

A read goes to the process's memory first, then Redis, and only then to the
function you pass (usually a database query); the result is written back to
both levels, and concurrent misses for the same key share one call, so a cold
key does not stampede the database. When any instance sets or invalidates a
key, the other instances are told through Redis pub/sub to drop their
in-memory copy, which keeps the memory caches of several pods consistent.
Common methods: `Get` (read-through), `GetWithTtl`, `Set`, `Invalidate`,
`SetMemCacheMaxTTLSeconds` (cap how long values stay in memory).

## Usage

```go
kv := cache.NewCacheWithDefaultEnvPrefix("my-service") // REDIS_*, DCACHE_*
rm := resource.NewResourceManager([]resource.Resource{kv})
// ... after Init:
dc := kv.GetDCache()          // *dcache.DCache
client := kv.GetRedisClient() // redis.UniversalClient

r := cache.NewRedisWithDefaultEnvPrefix() // client only
// ... after Init: r.Client()
```

## API

| Name | What it does |
|---|---|
| `NewRedisWithDefaultEnvPrefix()`, `NewRedis(prefix)`, `NewRedisWithConfig(cfg)` | A `Redis` resource (name `"redis"`); `Client()` returns the client. |
| `NewCacheWithDefaultEnvPrefix(appName)`, `NewCache(appName, redisPrefix, dcachePrefix)`, `NewCacheWithConfig(appName, redisCfg, dcacheCfg)`, `NewCacheWithOptions(opts...)` | A `Cache` resource (name `"cache"`). `appName` namespaces the cache keys and is required. `GetRedisClient()` and `GetDCache()` return the client and the cache. |
| `WithAppName`, `WithRedisConfig`, `WithRedisEnvPrefix`, `WithDCacheConfig`, `WithDCacheEnvPrefix` | Options for `NewCacheWithOptions`; it panics when a required one is missing. |

The environment-based constructors panic when a variable cannot be parsed, and
a `nil` configuration panics too: both are configuration errors found at
startup.

## Lifecycle

- **Init** logs the configuration (password and certificate masked), creates
  the client, instruments it for OpenTelemetry tracing and metrics, and pings
  Redis with a 10 s limit. Any failure closes the client and returns the error,
  so `Init` can be retried. `Cache` then creates dcache, cleaning up the same
  way on failure.
- **Start** returns an error before a successful `Init`.
- **Stop** closes dcache, then the client (`Cache`); safe to call more than once.
- **OK** pings Redis; before `Init` it returns `cache: not initialized`.

## Connection modes

| Mode | Set | Notes |
|---|---|---|
| Single node | `REDIS_HOST`, `REDIS_PORT` | Default. |
| Cluster | `REDIS_ISCLUSTERMODE=true`, `REDIS_CLUSTERADDRS` (comma-separated) | Follows up to `CLUSTERMAXREDIRECTS` redirects. |
| Sentinel | `REDIS_ISFAILOVER=true`; `HOST` and `PORT` point to a sentinel; `MASTERNAME` names the master | Host names are resolved on every connection, so a sentinel pod that changes IP is still found. |

Cluster and sentinel cannot both be on: `Init` returns an error.

## Configuration

| Variable | Default | Meaning |
|---|---|---|
| `REDIS_HOST`, `REDIS_PORT` | `127.0.0.1`, 6379 | Address (the sentinel's address in sentinel mode). |
| `REDIS_PASSWORD` | empty | Password; masked in logs. |
| `REDIS_ISFAILOVER`, `REDIS_MASTERNAME` | false, `master` | Sentinel mode. |
| `REDIS_ISCLUSTERMODE`, `REDIS_CLUSTERADDRS`, `REDIS_CLUSTERMAXREDIRECTS` | false, empty, 3 | Cluster mode. |
| `REDIS_READTIMEOUT` | 3s | Read timeout. |
| `REDIS_POOLSIZE` | 50 | Connection pool size. |
| `REDIS_DISABLEIDENTITY` | false | Do not send the client identity (some managed Redis services reject `CLIENT SETINFO`). |
| `REDIS_TLSENABLED` | false | TLS, minimum version 1.2. |
| `REDIS_TLSCACERT` | empty | PEM CA certificate to verify the server; empty uses the system roots. Not valid PEM: `Init` fails. |
| `REDIS_TLSSERVERNAME` | empty | Name to verify the certificate against; empty uses the host connected to. |
| `REDIS_TLSINSECURESKIPVERIFY` | false | Skip certificate verification. For tests only. |
| `DCACHE_INMEMCACHESIZE` | 52428800 | In-memory cache size in bytes (50 MiB). |
| `DCACHE_ENABLESTATS`, `DCACHE_ENABLETRACE` | true, true | dcache hit-rate metrics and spans. |

## See also

- [resource](..) — how `Init`, `Start` and `Stop` are driven.
- [otel](../../otel) — the Redis client's OpenTelemetry metrics are exported
  only when OpenTelemetry metrics are turned on.
