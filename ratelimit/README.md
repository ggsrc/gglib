# ratelimit

```sh
go get github.com/ggsrc/gglib/ratelimit
```

A Redis-backed rate limiter: "at most N events per period" for each key,
shared by every process that uses the same Redis. It wraps
[redis_rate](https://github.com/go-redis/redis_rate), which implements the
Generic Cell Rate Algorithm (GCRA). The check and the update run in one Lua
script, so concurrent callers cannot admit more than the limit, and a rejection
reports exactly how long to wait.

## Usage

```go
limiter := ratelimit.New(kv.GetRedisClient()) // any redis.UniversalClient

dec, err := limiter.Allow(ctx, "otp:"+email, ratelimit.PerMinute(5))
if err != nil {
	return err // Redis is unavailable; this is not a rejection
}
if !dec.Allowed {
	return tooManyRequests(dec.RetryAfter)
}
```

`kv` is a [resource/cache](../resource/cache) resource; any
`redis.UniversalClient` works.

## API

| Name | What it does |
|---|---|
| `New(rdb)` | A `Limiter` that keeps its buckets in `rdb`. Panics if `rdb` is nil. |
| `(*Limiter) Allow(ctx, key, lim) (Decision, error)` | Admits or rejects the next event for `key`. On a Redis error it returns the error (`ratelimit: allow "<key>": …`) and a zero `Decision`. |
| `(*Limiter) Reset(ctx, key) error` | Clears the bucket for `key`. A missing key is not an error. |
| `Decision` | `Allowed`, and `RetryAfter`: zero when allowed, otherwise the time until one event would be admitted. |
| `Limit`, `PerSecond(n)`, `PerMinute(n)`, `PerHour(n)` | The limit, re-exported from redis_rate. The helpers set `Rate` and `Burst` to `n`. Build a `Limit` directly for a different burst. |

## Things to know

- Keys are yours to namespace (`"otp:{email}"`, `"signin:{ip}"`). The limiter
  stores each bucket in Redis as `rate:<key>` (redis_rate's prefix).
- The window slides: after the limit is reached, one event is admitted again
  every `Period / Rate`, not all at once at the end of a window.
- The limiter does not log or record metrics. Add them at the call site, where
  you have the labels.
- For leaky-bucket or fixed-window behaviour, use redis_rate directly.

## See also

- [resource/cache](../resource/cache) — the Redis client as a resource.
