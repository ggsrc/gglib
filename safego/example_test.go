package safego_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/ggsrc/gglib/safego"
)

func ExampleGroup() {
	ctx := context.Background()
	g, groupCtx := safego.WithContext(ctx, "example.fanout")
	results := make([]int, 3)
	for i := range results {
		g.Go(func() error {
			if err := groupCtx.Err(); err != nil {
				return err
			}
			results[i] = i * i
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return
	}
	fmt.Println(results)
	// Output: [0 1 4]
}

func ExampleGroup_GoInto() {
	ctx := context.Background()
	g := safego.NewGroup(ctx, "example.optional")
	var optionalErr error
	g.GoInto(&optionalErr, func() error { return errors.New("optional source unavailable") })
	g.Go(func() error { return nil })
	if err := g.Wait(); err != nil {
		return
	}
	fmt.Println("required work ok; optional:", optionalErr)
	// Output: required work ok; optional: optional source unavailable
}

func ExampleRun() {
	ctx := context.Background()
	for _, item := range []string{"a", "b"} {
		// One item's panic does not stop the loop.
		if err := safego.Run(ctx, "example.batch", func() error {
			if item == "a" {
				panic("bad item")
			}
			return nil
		}); errors.Is(err, safego.ErrPanic) {
			fmt.Println(item, "failed")
			continue
		}
		fmt.Println(item, "done")
	}
	// Output:
	// a failed
	// b done
}
