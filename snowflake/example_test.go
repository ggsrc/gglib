package snowflake_test

import (
	"context"
	"fmt"

	"github.com/ggsrc/gglib/snowflake"
)

func Example() {
	// In a service, pass the Redis client (for example from resource/cache)
	// instead of nil, so every process gets its own node number.
	if err := snowflake.Init(context.Background(), nil, "orders"); err != nil {
		panic(err)
	}
	a, b := snowflake.Generate(), snowflake.Generate()
	fmt.Println(b > a)
	// Output: true
}
