package ratelimit

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// newTestLimiter spins up an in-process miniredis and wires a Limiter
// against it. Returns a teardown the caller defers.
func newTestLimiter(t *testing.T) (*Limiter, *miniredis.Miniredis, func()) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{mr.Addr()}})
	return New(rdb), mr, func() { _ = rdb.Close() }
}

// Burst — a fresh bucket admits exactly `Rate` calls back-to-back, then
// rejects the next with a non-zero RetryAfter.
func TestAllow_BurstCapacityRespected(t *testing.T) {
	l, _, cleanup := newTestLimiter(t)
	defer cleanup()
	ctx := context.Background()

	const capacity = 5
	lim := PerSecond(capacity)
	for i := range capacity {
		dec, err := l.Allow(ctx, "k", lim)
		if err != nil {
			t.Fatalf("allow %d: %v", i, err)
		}
		if !dec.Allowed {
			t.Fatalf("allow %d/%d must succeed in burst", i+1, capacity)
		}
	}

	dec, err := l.Allow(ctx, "k", lim)
	if err != nil {
		t.Fatalf("allow capacity+1: %v", err)
	}
	if dec.Allowed {
		t.Fatalf("allow #%d must be rejected", capacity+1)
	}
	if dec.RetryAfter <= 0 {
		t.Fatalf("retry-after must be positive on rejection, got %v", dec.RetryAfter)
	}
}

// Concurrency — N>capacity goroutines hammering the same bucket must
// see exactly `capacity` admissions. This is the regression test for
// the TOCTOU race that the home-rolled INCR+EXPIRE limiters were
// vulnerable to. GCRA's atomic Lua admission rule prevents it.
func TestAllow_ConcurrentBurstExactlyCapacity(t *testing.T) {
	l, _, cleanup := newTestLimiter(t)
	defer cleanup()
	ctx := context.Background()

	const (
		capacity = 10
		workers  = 50
	)
	lim := PerMinute(capacity)

	var allowed int64
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			dec, err := l.Allow(ctx, "concur", lim)
			if err == nil && dec.Allowed {
				atomic.AddInt64(&allowed, 1)
			}
		}()
	}
	wg.Wait()

	if allowed != capacity {
		t.Fatalf("GCRA must admit exactly capacity=%d under concurrency, got %d", capacity, allowed)
	}
}

// Sliding window — after the bucket fills, advancing time by one
// emission interval frees exactly one slot. Verifies smooth pacing
// (not all-at-once reset like fixed-window has).
//
// Implementation note: redis_rate's Lua reads `redis.call('TIME')`,
// which miniredis answers from its own clock (m.SetTime). We pin the
// clock at a known instant, fill the bucket, then jog the clock forward
// by exactly one emission interval — equivalent to a real sleep but
// deterministic and instant.
func TestAllow_SlidingWindowFreesOneSlotPerInterval(t *testing.T) {
	l, mr, cleanup := newTestLimiter(t)
	defer cleanup()
	ctx := context.Background()

	// period * rate chosen so emission_interval = 1s exactly — the
	// redis_rate Lua does float math on (period/rate) and accumulates
	// drift if the result isn't representable, which makes the burst
	// boundary admit one fewer call than expected. Whole-second
	// emission sidesteps that without weakening the assertions.
	const capacity = 4
	period := 4 * time.Second
	emissionInterval := period / capacity // = 1s exactly
	lim := Limit{Rate: capacity, Burst: capacity, Period: period}

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mr.SetTime(t0)

	for i := range capacity {
		dec, err := l.Allow(ctx, "slide", lim)
		if err != nil || !dec.Allowed {
			t.Fatalf("fill #%d: allowed=%v err=%v", i, dec.Allowed, err)
		}
	}
	dec, err := l.Allow(ctx, "slide", lim)
	if err != nil {
		t.Fatalf("post-fill: %v", err)
	}
	if dec.Allowed {
		t.Fatalf("bucket must be full")
	}

	// Jog the clock forward exactly one emission interval (+ 1ms slack
	// so we land just past the allow boundary, not on it).
	mr.SetTime(t0.Add(emissionInterval + time.Millisecond))

	dec, err = l.Allow(ctx, "slide", lim)
	if err != nil {
		t.Fatalf("post-advance: %v", err)
	}
	if !dec.Allowed {
		t.Fatalf("exactly one slot must free after one emission interval")
	}

	// Immediate next acquire should reject — only one slot freed.
	dec, err = l.Allow(ctx, "slide", lim)
	if err != nil {
		t.Fatalf("post-second: %v", err)
	}
	if dec.Allowed {
		t.Fatalf("no second slot should free without waiting another interval")
	}
}

// Per-key isolation — events on key A don't consume key B's bucket.
// Critical for OTP / Signin use cases where each email / IP gets its
// own quota.
func TestAllow_KeysAreIsolated(t *testing.T) {
	l, _, cleanup := newTestLimiter(t)
	defer cleanup()
	ctx := context.Background()

	lim := PerMinute(2)

	// Burn alice's full quota.
	for i := range 2 {
		dec, _ := l.Allow(ctx, "alice", lim)
		if !dec.Allowed {
			t.Fatalf("alice burst #%d should succeed", i+1)
		}
	}
	if dec, _ := l.Allow(ctx, "alice", lim); dec.Allowed {
		t.Fatalf("alice's 3rd call must reject")
	}

	// bob's bucket is independent.
	dec, _ := l.Allow(ctx, "bob", lim)
	if !dec.Allowed {
		t.Fatalf("bob's first call must succeed despite alice's bucket being full")
	}
}

// Reset — clears a key so the next Allow starts fresh. Mirrors what
// admin tools / tests use to undo a rate-limit hit.
func TestReset_ClearsBucket(t *testing.T) {
	l, _, cleanup := newTestLimiter(t)
	defer cleanup()
	ctx := context.Background()

	lim := PerMinute(1)
	if dec, _ := l.Allow(ctx, "r", lim); !dec.Allowed {
		t.Fatalf("first call must succeed")
	}
	if dec, _ := l.Allow(ctx, "r", lim); dec.Allowed {
		t.Fatalf("second call must reject")
	}

	if err := l.Reset(ctx, "r"); err != nil {
		t.Fatalf("reset: %v", err)
	}

	dec, err := l.Allow(ctx, "r", lim)
	if err != nil {
		t.Fatalf("post-reset: %v", err)
	}
	if !dec.Allowed {
		t.Fatalf("call after reset must succeed (bucket cleared)")
	}
}

// Reset on a missing key is a no-op (no error). Documents the contract
// for tools that "ensure clean state" without first checking existence.
func TestReset_MissingKeyIsNoOp(t *testing.T) {
	l, _, cleanup := newTestLimiter(t)
	defer cleanup()

	if err := l.Reset(context.Background(), "never-touched"); err != nil {
		t.Fatalf("reset on missing key must not error, got: %v", err)
	}
}

// New panics on a nil client. Wiring bugs surface at construction
// instead of as a confusing nil-deref on first Allow.
func TestNew_PanicsOnNilClient(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("New(nil) must panic, no panic recovered")
		}
	}()
	_ = New(nil)
}

// The Limit helpers fill Rate and Burst with the same value.
func TestLimitHelpers(t *testing.T) {
	cases := []struct {
		got  Limit
		want Limit
	}{
		{PerSecond(3), Limit{Rate: 3, Burst: 3, Period: time.Second}},
		{PerMinute(4), Limit{Rate: 4, Burst: 4, Period: time.Minute}},
		{PerHour(5), Limit{Rate: 5, Burst: 5, Period: time.Hour}},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Fatalf("got %+v, want %+v", c.got, c.want)
		}
	}
}

// A Redis failure comes back as an error naming the key, never as a
// rejection, so callers can tell "over the limit" from "limiter down".
func TestRedisErrorsAreReturned(t *testing.T) {
	l, mr, cleanup := newTestLimiter(t)
	defer cleanup()
	mr.Close()
	ctx := context.Background()

	dec, err := l.Allow(ctx, "k", PerSecond(1))
	if err == nil || !strings.Contains(err.Error(), `ratelimit: allow "k"`) {
		t.Fatalf("allow: want a wrapped error, got %v", err)
	}
	if dec != (Decision{}) {
		t.Fatalf("allow: want a zero Decision on error, got %+v", dec)
	}
	if err := l.Reset(ctx, "k"); err == nil || !strings.Contains(err.Error(), `ratelimit: reset "k"`) {
		t.Fatalf("reset: want a wrapped error, got %v", err)
	}
}
