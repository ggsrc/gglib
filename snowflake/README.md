# snowflake

```sh
go get github.com/ggsrc/gglib/snowflake
```

Unique, roughly time-ordered 64-bit IDs from
[bwmarrin/snowflake](https://github.com/bwmarrin/snowflake). Each process takes
its node number from a Redis counter, so processes of the same app never share
one while fewer than 1024 of them run at once.

## Usage

```go
// At startup, after Redis is up (for example kv from resource/cache):
if err := snowflake.Init(ctx, kv.GetRedisClient(), "orders"); err != nil {
	return err
}

// Anywhere afterwards:
id := snowflake.Generate()
row.ID = id.Int64()
```

To hold a node yourself instead of using the default, call
`snowflake.NewNode(ctx, rdb, appName)` and use `node.Generate()`.

## API

| Name | What it does |
|---|---|
| `Init(ctx, rdb, appName) error` | Creates a node with `NewNode` and makes it the default. On error the previous default stays. |
| `Generate() ID` | A new ID from the default node. Panics if `Init` has not succeeded. |
| `Default() *Node` | The default node, or nil before `Init`. |
| `NewNode(ctx, rdb, appName) (*Node, error)` | A node whose number is `INCR Key(appName)` modulo `MaxNodes`. Errors: empty `appName`, or Redis failing (`snowflake: claim node number: …`). With a nil `rdb` the number comes from the clock. |
| `Key(appName)` | The counter key, `<appName>:snowflake:nodeID`. |
| `ID`, `Node`, `MaxNodes` | `snowflake.ID` and `snowflake.Node` re-exported; 1024 node numbers. |

## Things to know

- **Every start takes a new number.** The counter only grows, so a rolling
  update moves each new process to the next number. Numbers repeat only after
  1024 starts, by which time the process that used the number before has
  usually exited.
- **A Redis reset can cause duplicates.** If Redis loses the counter while old
  processes still run, new processes start again from 1 and may share a number
  with a running process until it restarts. Use a persistent Redis.
- **A nil `rdb` is for tests and local runs.** The clock-based number can
  repeat across processes.
- The library's defaults apply: the Twitter epoch (2010-11-04), 10 node bits
  and 12 sequence bits, which allows 4096 IDs per millisecond per node.
  bwmarrin/snowflake keeps these settings in package variables; changing them
  changes every node in the process.
- Nothing is logged. Log the node number at startup with
  `snowflake.Generate().Node()` if you want it.

## See also

- [resource/cache](../resource/cache) — the Redis client as a resource.
