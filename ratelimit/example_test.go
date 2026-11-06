package ratelimit_test

import (
	"context"
	"fmt"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/ggsrc/gglib/ratelimit"
)

func Example() {
	mr, err := miniredis.Run() // any Redis works; this one is in-process
	if err != nil {
		panic(err)
	}
	defer mr.Close()
	rdb := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{mr.Addr()}})
	defer rdb.Close()

	limiter := ratelimit.New(rdb)
	ctx := context.Background()
	for i := 1; i <= 3; i++ {
		dec, err := limiter.Allow(ctx, "otp:alice@example.com", ratelimit.PerMinute(2))
		if err != nil {
			panic(err)
		}
		fmt.Printf("attempt %d allowed: %v\n", i, dec.Allowed)
	}
	// Output:
	// attempt 1 allowed: true
	// attempt 2 allowed: true
	// attempt 3 allowed: false
}
