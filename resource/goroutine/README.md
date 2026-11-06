# resource/goroutine

```sh
go get github.com/ggsrc/gglib/resource/goroutine
```

Long-lived background goroutines — pollers, consumers, periodic jobs — as a
[resource](..). A `GoroutineManager` gives every goroutine it starts a
context that is canceled on shutdown, then waits for the goroutines to
return, so in-flight work can finish before the process exits. Each goroutine
recovers its own panics through [safego](../../safego), so one failing worker
does not crash the process.

Depends on [resource](..), [safego](../../safego) and [zerolog](../../zerolog).

## Usage

```go
import (
	"github.com/ggsrc/gglib/resource"
	"github.com/ggsrc/gglib/resource/goroutine"
)

workers := goroutine.NewGoroutineManager()
rm := resource.NewResourceManager([]resource.Resource{db, workers})
// ... after rm.Init (or app.Init):

workers.Run("outbox-relay", func(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil // shutting down
		case <-ticker.C:
			if err := relayOutbox(ctx); err != nil {
				log.Ctx(ctx).Err(err).Msg("relay failed")
			}
		}
	}
})
```

Register the manager **after** the resources its goroutines use: resources
stop in reverse order, so the goroutines are stopped before the database pool
they write to is closed.

## API

| Name | What it does |
|---|---|
| `NewGoroutineManager()` | A new manager. Call `Init` before `Run`. |
| `Init(ctx)` | Derives the context the goroutines receive: it keeps the values of `ctx` and is canceled when `ctx` is canceled or `Stop` is called. Pass a context that lives as long as the service, not one with an initialization deadline. Never fails; calling it again has no effect. |
| `Start(ctx)` | Does nothing; goroutines start in `Run`. |
| `Run(name, f func(ctx) error)` | Starts `f` in a new goroutine and returns immediately. `f` should return when its context is canceled. |
| `Stop(ctx)` | Cancels the goroutines' context and waits until all of them have returned. Returns `ctx.Err()` if `ctx` ends first; the goroutines keep running until they notice the cancellation. A later `Stop` waits again. |
| `OK(ctx)` | Always `nil`. |
| `Name()` | `"goroutine_manager"`. |

All methods are safe for concurrent use.

## Failure cases

| Case | What happens |
|---|---|
| `f` returns an error | Logged at error level: `goroutine [<name>] error`, with `error` and `goroutine` fields, through the logger of the `Init` context. |
| `f` panics | Recovered and logged once with the panic value and stack (`safego: panic recovered`, label `goroutine <name>`). The other goroutines and the process keep running. |
| `Run` before `Init` | Panics: it is a programming error. |
| `Run` after `Stop` | `f` is not started; an error is logged (`GoroutineManager [<name>] not started: the manager is stopped`). |
| `Stop` before `Init` | Does nothing and returns `nil`; the manager can still be initialized and used. |
| `Stop` deadline passes | `Stop` returns the context error; the resource manager reports it with the other stop errors. |

## Upgrading from the package inside `resource`

Until `resource/v0.1.0`, this package lived in the `github.com/ggsrc/gglib/resource`
module. It is now its own module with the same import path and the same API.

```sh
go get github.com/ggsrc/gglib/resource/goroutine@latest
```

The module requires `resource` v0.1.0 or later, which no longer contains the
package, so `go get` also moves `resource` past the old copy and the import
resolves to exactly one module. If a build reports `ambiguous import`, run
`go get github.com/ggsrc/gglib/resource@latest` as well.

Behaviour changes from the old package are listed in [CHANGELOG.md](CHANGELOG.md).

## See also

- [resource](..) — how `Init`, `Start` and `Stop` are driven.
- [safego](../../safego) — panic-safe goroutines for request-scoped work:
  fire-and-forget, async results and fan-outs.
