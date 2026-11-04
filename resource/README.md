# resource

```sh
go get github.com/ggsrc/gglib/resource
```

A small lifecycle for dependencies with a start and an end — connection
pools, caches, background workers: initialize them together, start them
together, check their health, and stop them in the right order.
[resource/wpgx](wpgx) and [resource/cache](cache) are resources;
[app](../app) drives a `ResourceManager`.

## The Resource interface

```go
type Resource interface {
	Name() string                    // appears in errors, e.g. "wpgx"
	Init(ctx context.Context) error  // connect, prepare; must be safe to retry after a failure
	Start(ctx context.Context) error // begin work; must not block
	Stop(ctx context.Context) error  // release; must be safe even if never started
	OK(ctx context.Context) error    // health check
}
```

## ResourceManager

```go
rm := resource.NewResourceManager([]resource.Resource{db, cache, worker})
```

A `ResourceManager` has the same `Init`, `Start`, `Stop` and `OK`, plus
`AddResource(r) error` (safe for concurrent use; returns `nil`).

| Method | Order | Behaviour |
|---|---|---|
| `Init` | registration order | Initializes one by one and stops at the first error: `resource <name>: init: <cause>`. **Resumable:** the next call starts at the resource that failed; resources that succeeded are not initialized again. |
| `Start` | registration order | Starts one by one. If one fails, all resources are stopped, and the start error is returned together with any stop errors. |
| `Stop` | **reverse** registration order | Stops every resource even if one fails, and returns all errors joined. Only the first call has an effect. |
| `OK` | all | Checks every resource and returns all errors joined. |

**Register what others depend on first.** A worker that writes to the
database is registered after the database, so at shutdown it stops before the
pool closes and never writes to a closed pool.

## Writing your own resource

```go
type Worker struct{ cancel context.CancelFunc; done chan struct{} }

func (w *Worker) Name() string                 { return "worker" }
func (w *Worker) Init(ctx context.Context) error { return nil }
func (w *Worker) Start(ctx context.Context) error {
	ctx, w.cancel = context.WithCancel(context.WithoutCancel(ctx))
	w.done = make(chan struct{})
	go func() { defer close(w.done); w.loop(ctx) }()
	return nil
}
func (w *Worker) Stop(ctx context.Context) error {
	if w.cancel == nil {
		return nil
	}
	w.cancel()
	select {
	case <-w.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (w *Worker) OK(ctx context.Context) error { return nil }
```

## See also

- [app](../app) — `Init` retries the manager's `Init` with backoff; `Run`
  starts it and adds `OK` to the health checks.
- [resource/wpgx](wpgx), [resource/cache](cache) — the resources gglib ships.
