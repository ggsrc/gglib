package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"
)

// Limit is the rate ceiling for one key. It is redis_rate.Limit, re-exported
// so callers need only this import.
type Limit = redis_rate.Limit

// PerSecond returns Limit{Rate: rate, Burst: rate, Period: time.Second}.
func PerSecond(rate int) Limit { return redis_rate.PerSecond(rate) }

// PerMinute returns Limit{Rate: rate, Burst: rate, Period: time.Minute}.
func PerMinute(rate int) Limit { return redis_rate.PerMinute(rate) }

// PerHour returns Limit{Rate: rate, Burst: rate, Period: time.Hour}.
func PerHour(rate int) Limit { return redis_rate.PerHour(rate) }

// Decision is the verdict for one event.
//
// When Allowed is true, RetryAfter is zero. When Allowed is false, RetryAfter
// is the time until one event would be admitted at the steady rate; callers
// can wait and retry, schedule a retry, or return it as a Retry-After header.
type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
}

// Limiter checks events against a Limit using a Redis-backed GCRA bucket per
// key. It holds no state of its own and is safe for concurrent use; share one
// per process and Redis client.
type Limiter struct {
	rl *redis_rate.Limiter
}

// New returns a Limiter that keeps its buckets in rdb.
//
// It panics if rdb is nil: a nil client is a wiring bug, and failing here is
// clearer than a nil dereference inside redis_rate on the first Allow.
func New(rdb redis.UniversalClient) *Limiter {
	if rdb == nil {
		panic("ratelimit.New: nil redis client")
	}
	return &Limiter{rl: redis_rate.NewLimiter(rdb)}
}

// Allow reports whether the next event for key is admitted under lim. The
// check and the bucket update run in one Lua script, so concurrent callers
// cannot admit more than lim allows.
//
// key is opaque to this package; callers namespace their own keys (for
// example "otp:{email}" or "signin:{ip}").
func (l *Limiter) Allow(ctx context.Context, key string, lim Limit) (Decision, error) {
	res, err := l.rl.Allow(ctx, key, lim)
	if err != nil {
		return Decision{}, fmt.Errorf("ratelimit: allow %q: %w", key, err)
	}
	if res.Allowed >= 1 {
		return Decision{Allowed: true}, nil
	}
	return Decision{Allowed: false, RetryAfter: res.RetryAfter}, nil
}

// Reset clears the bucket for key, for example from an admin tool or a test.
// It returns nil when the key does not exist.
func (l *Limiter) Reset(ctx context.Context, key string) error {
	if err := l.rl.Reset(ctx, key); err != nil {
		return fmt.Errorf("ratelimit: reset %q: %w", key, err)
	}
	return nil
}
