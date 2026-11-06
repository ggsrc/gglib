package snowflake

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newRedis(t *testing.T) (*miniredis.Miniredis, redis.UniversalClient) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{mr.Addr()}, MaxRetries: -1})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb
}

func resetDefault(t *testing.T) {
	t.Helper()
	defaultNode.Store(nil)
	t.Cleanup(func() { defaultNode.Store(nil) })
}

func TestKey(t *testing.T) {
	require.Equal(t, "orders:snowflake:nodeID", Key("orders"))
}

// Each process start takes the next node number, and the IDs it generates
// carry that number.
func TestNewNodeTakesSuccessiveNodeNumbers(t *testing.T) {
	_, rdb := newRedis(t)
	ctx := context.Background()

	for want := int64(1); want <= 3; want++ {
		node, err := NewNode(ctx, rdb, "orders")
		require.NoError(t, err)
		require.Equal(t, want, node.Generate().Node())
	}
}

// Node numbers wrap at MaxNodes instead of failing.
func TestNewNodeWrapsAtMaxNodes(t *testing.T) {
	mr, rdb := newRedis(t)
	require.NoError(t, mr.Set(Key("orders"), "1023"))

	node, err := NewNode(context.Background(), rdb, "orders")
	require.NoError(t, err)
	require.Equal(t, int64(0), node.Generate().Node())
}

// Different apps have separate counters.
func TestNewNodeCountersArePerApp(t *testing.T) {
	_, rdb := newRedis(t)
	ctx := context.Background()

	a, err := NewNode(ctx, rdb, "a")
	require.NoError(t, err)
	b, err := NewNode(ctx, rdb, "b")
	require.NoError(t, err)
	require.Equal(t, int64(1), a.Generate().Node())
	require.Equal(t, int64(1), b.Generate().Node())
}

func TestNewNodeWithoutRedisUsesTheClock(t *testing.T) {
	node, err := NewNode(context.Background(), nil, "orders")
	require.NoError(t, err)
	n := node.Generate().Node()
	require.GreaterOrEqual(t, n, int64(0))
	require.Less(t, n, int64(1000))
}

func TestNewNodeErrors(t *testing.T) {
	_, err := NewNode(context.Background(), nil, "")
	require.EqualError(t, err, "snowflake: empty app name")

	mr, rdb := newRedis(t)
	mr.Close()
	_, err = NewNode(context.Background(), rdb, "orders")
	require.ErrorContains(t, err, "snowflake: claim node number: ")

	// A counter that is not an integer makes INCR fail too.
	mr2, rdb2 := newRedis(t)
	require.NoError(t, mr2.Set(Key("orders"), "not-a-number"))
	_, err = NewNode(context.Background(), rdb2, "orders")
	require.ErrorContains(t, err, "snowflake: claim node number: ")
}

// A negative counter yields a negative node number, which the snowflake
// library rejects; NewNode returns that error instead of a broken node.
func TestNewNodeRejectsInvalidNodeNumber(t *testing.T) {
	mr, rdb := newRedis(t)
	require.NoError(t, mr.Set(Key("orders"), "-5"))
	_, err := NewNode(context.Background(), rdb, "orders")
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), "snowflake: new node -4: "), err.Error())
}

func TestGenerateBeforeInitPanics(t *testing.T) {
	resetDefault(t)
	require.Nil(t, Default())
	require.PanicsWithValue(t, "snowflake: Generate called before Init", func() { Generate() })
}

func TestInitSetsTheDefault(t *testing.T) {
	resetDefault(t)
	_, rdb := newRedis(t)
	ctx := context.Background()

	require.NoError(t, Init(ctx, rdb, "orders"))
	require.NotNil(t, Default())
	require.Equal(t, int64(1), Generate().Node())

	// A failed Init keeps the previous default.
	before := Default()
	require.Error(t, Init(ctx, rdb, ""))
	require.Same(t, before, Default())
}

// IDs from one node are unique and increasing, also under concurrency.
func TestGenerateIsUniqueUnderConcurrency(t *testing.T) {
	resetDefault(t)
	require.NoError(t, Init(context.Background(), nil, "orders"))

	const workers, perWorker = 8, 2000
	ids := make(chan ID, workers*perWorker)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			prev := ID(0)
			for range perWorker {
				id := Generate()
				if id <= prev {
					t.Errorf("IDs from one goroutine must increase: %d after %d", id, prev)
				}
				prev = id
				ids <- id
			}
		})
	}
	wg.Wait()
	close(ids)

	seen := make(map[ID]struct{}, workers*perWorker)
	for id := range ids {
		_, dup := seen[id]
		require.False(t, dup, "duplicate ID %d", id)
		seen[id] = struct{}{}
	}
}
