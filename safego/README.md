# safego

```sh
go get github.com/ggsrc/gglib/safego
```

Start goroutines that recover their own panics. In Go, a panic in a goroutine
that does not recover it ends the whole process, whatever the code that
started the goroutine does; one nil map in a background task takes every
in-flight request down with it. safego puts a recovery boundary around each
goroutine, logs the panic once, and turns it into an ordinary error of the work
that owned it.

Depends on [zerolog](../zerolog) for logging.

## Which function to use

| Shape of the work | Use | A panic becomes |
|---|---|---|
| Fire and forget: background work nobody waits for | `Go(ctx, label, fn)` | a log line |
| One call whose result you want later | `Async(ctx, label, fn) <-chan error` | `ErrPanic` on the channel |
| A fan-out you wait for | `NewGroup` / `WithContext`, then `Go`, `TryGo`, `GoInto`, `Wait` | the error of the goroutine that panicked |
| One isolated unit in the current goroutine: a loop iteration, a batch item, a cleanup step | `Run(ctx, label, fn) error` | `ErrPanic`, returned |

`label` names the call site in logs, for example `"billing.sendInvoice"`.

```go
import "github.com/ggsrc/gglib/safego"

// Background work.
safego.Go(ctx, "audit.write", func() { writeAudit(ctx, event) })

// A fan-out where any failure cancels the rest.
g, gctx := safego.WithContext(ctx, "profile.load")
g.Go(func() error { return loadUser(gctx, id) })
g.Go(func() error { return loadSettings(gctx, id) })
if err := g.Wait(); err != nil {
	return err
}

// One optional branch whose failure is handled, not fatal.
var recsErr error
g2 := safego.NewGroup(ctx, "home.page")
g2.Go(func() error { return loadFeed(ctx) })
g2.GoInto(&recsErr, func() error { return loadRecommendations(ctx) })
if err := g2.Wait(); err != nil {
	return err
}
if recsErr != nil {
	// show the page without recommendations
}

// A long-lived loop that survives a bad item.
for msg := range messages {
	if err := safego.Run(ctx, "queue.handle", func() error { return handle(ctx, msg) }); err != nil {
		nack(msg)
	}
}
```

## API

| Name | What it does |
|---|---|
| `ErrPanic` | The error that replaces a recovered panic. Match it with `errors.Is`. |
| `Run(ctx, label, fn func() error) error` | Calls `fn` and returns its error, or `ErrPanic` if it panicked. A nil `fn` counts as a panic. |
| `Go(ctx, label, fn func())` | Runs `fn` in a new goroutine. A panic is logged; nothing else observes it. |
| `Async(ctx, label, fn func() error) <-chan error` | Runs `fn` in a new goroutine and sends its error, or `ErrPanic`, on a channel with room for one value, so the goroutine finishes even if nobody reads it. |
| `NewGroup(ctx, label) *Group` | A group that waits for all its goroutines and cancels none of them. |
| `WithContext(ctx, label) (*Group, context.Context)` | A group plus a derived context, canceled when a goroutine started with `Go` fails or panics, or when `Wait` returns. |
| `(*Group).Go(fn func() error)` | Runs `fn` in the group. Its error or `ErrPanic` fails the group: `Wait` returns the first one. Blocks while the group is at its limit. |
| `(*Group).TryGo(fn func() error) bool` | `Go` without blocking: starts `fn` only below the limit and reports whether it did. |
| `(*Group).GoInto(errp *error, fn func() error)` | Runs `fn` in the group but stores its error or `ErrPanic` in `*errp` instead of failing the group. Read `*errp` after `Wait`. A nil `errp` discards the error. |
| `(*Group).SetLimit(n int)` | At most `n` goroutines at once; negative means no limit. |
| `(*Group).Wait() error` | Waits for every goroutine, then returns the first error from `Go`. |

`Group` wraps [`golang.org/x/sync/errgroup`](https://pkg.go.dev/golang.org/x/sync/errgroup)
and has the same semantics apart from panics. Create it with `NewGroup` or
`WithContext`; the zero value is not usable.

## What a panic looks like

Each panic is logged once, at error level, through the logger of the
caller's context (`log.Ctx(ctx)` from [zerolog/log](../zerolog#zerologlog)),
so request ID, user ID and trace ID are on the line:

```json
{"level":"error","panic":"assignment to entry in nil map","stack":"goroutine 42 [running]:\n...","label":"audit.write","message":"safego: panic recovered"}
```

The caller receives `ErrPanic`, which carries neither the panic value nor the
label. That keeps internal details such as a panic message out of API
responses; the log line has them.

## Rules of thumb

- **Cancellation is yours.** safego never cancels or detaches the context you
  pass. For work that must outlive a request, pass
  `context.WithoutCancel(ctx)`; for work that must stop with it, pass `ctx`
  and watch `ctx.Done()`.
- **`Go` is for work nobody waits for.** If the process must not exit before
  the work finishes, use a `Group`, or a
  [`resource/goroutine`](../resource/goroutine) manager that is stopped on
  shutdown.
- **A recovered panic is still a bug.** Alert on the `safego: panic
  recovered` log line; recovery keeps the process up, it does not make the
  code correct.
- Inside a goroutine you start yourself, wrap the body with `Run` rather
  than writing your own `recover`.

## See also

- [resource/goroutine](../resource/goroutine) — long-lived goroutines that are
  canceled and awaited on shutdown; recovers panics with safego.
- [interceptor](../interceptor) — panic recovery for gRPC handlers.
