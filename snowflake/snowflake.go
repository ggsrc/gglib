package snowflake

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/redis/go-redis/v9"
)

// MaxNodes is the number of distinct node numbers (10 bits).
const MaxNodes = 1024

// ID is a snowflake ID. It is snowflake.ID, re-exported so callers need only
// this import; it has Int64, String and Base58 among its encodings.
type ID = snowflake.ID

// Node generates IDs for one node number. It is safe for concurrent use.
type Node = snowflake.Node

var defaultNode atomic.Pointer[Node]

// Key returns the Redis key whose counter hands out node numbers for appName.
func Key(appName string) string {
	return appName + ":snowflake:nodeID"
}

// NewNode returns a Node with a node number taken from rdb.
//
// The number is INCR Key(appName) modulo MaxNodes. With a nil rdb it is
// derived from the clock instead, which suits tests and local runs only: two
// processes started in the same microsecond modulo 1000 would collide.
func NewNode(ctx context.Context, rdb redis.UniversalClient, appName string) (*Node, error) {
	if appName == "" {
		return nil, errors.New("snowflake: empty app name")
	}
	var number int64
	if rdb == nil {
		number = time.Now().UnixMicro() % 1000
	} else {
		n, err := rdb.Incr(ctx, Key(appName)).Result()
		if err != nil {
			return nil, fmt.Errorf("snowflake: claim node number: %w", err)
		}
		number = n % MaxNodes
	}
	node, err := snowflake.NewNode(number)
	if err != nil {
		return nil, fmt.Errorf("snowflake: new node %d: %w", number, err)
	}
	return node, nil
}

// Init creates a Node with NewNode and makes it the default for Generate. On
// error the previous default, if any, stays in place.
func Init(ctx context.Context, rdb redis.UniversalClient, appName string) error {
	node, err := NewNode(ctx, rdb, appName)
	if err != nil {
		return err
	}
	defaultNode.Store(node)
	return nil
}

// Default returns the default Node, or nil before Init succeeds.
func Default() *Node {
	return defaultNode.Load()
}

// Generate returns a new ID from the default Node. It panics if Init has not
// succeeded, because an ID from an unknown node number could collide.
func Generate() ID {
	node := defaultNode.Load()
	if node == nil {
		panic("snowflake: Generate called before Init")
	}
	return node.Generate()
}
