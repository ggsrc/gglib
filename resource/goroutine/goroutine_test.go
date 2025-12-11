package goroutine

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestGoroutineManager_GracefulShutdown_WithWork(t *testing.T) {
	gm := NewGoroutineManager()
	ctx := context.Background()

	if err := gm.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if err := gm.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	var workCompleted atomic.Int32
	numGoroutines := 3

	for range numGoroutines {
		gm.Run("worker", func(ctx context.Context) error {
			// Simulate work that needs to complete before shutdown
			for {
				select {
				case <-ctx.Done():
					// Perform cleanup work
					time.Sleep(100 * time.Millisecond)
					workCompleted.Add(1)
					return nil
				case <-time.After(5 * time.Millisecond):
					// Simulate periodic work
				}
			}
		})
	}

	// Stop should wait for cleanup to complete
	if err := gm.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify all goroutines completed their cleanup
	if count := workCompleted.Load(); count != int32(numGoroutines) {
		t.Errorf("Expected %d goroutines to complete work, but got %d", numGoroutines, count)
	}
}
