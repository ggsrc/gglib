// Package ratelimit admits or rejects events against a per-key rate limit
// stored in Redis.
//
// It is a thin wrapper over redis_rate, which implements the Generic Cell
// Rate Algorithm (GCRA): a sliding window checked and updated in one Lua
// script round trip, so it is atomic across processes, uses O(1) memory per
// key and reports the exact time until the next event would be admitted.
//
// The wrapper exists for three reasons:
//
//   - Callers get a small Decision{Allowed, RetryAfter} instead of the full
//     redis_rate result, so the decision shape stays the same across
//     services.
//   - Services that share this module share one redis_rate version.
//   - GCRA becomes the default. Hand-rolled INCR+EXPIRE limiters are fixed
//     windows, which admit up to twice the limit across a window edge.
//
// The package does not namespace keys, record metrics or log; callers do
// that where they have the context. For leaky-bucket or fixed-window
// semantics, use redis_rate directly.
package ratelimit
