# pprof

```sh
go get github.com/ggsrc/gglib/pprof
```

An HTTP server for on-demand profiling of a running service. It serves only
goroutine and heap profiles and a CPU profile of bounded length, is off unless
enabled, can require a token, and runs at most one CPU profile at a time. Use it
with `go tool pprof` when a pod misbehaves; it is not continuous profiling.

## Usage

```go
srv := pprof.NewWithDefaultEnvPrefix() // DEBUG_PPROF_*, off by default
if err := srv.Start(ctx); err != nil {
	return err // the address is taken
}
defer srv.Stop(context.Background())
```

```sh
kubectl port-forward pod/<pod> 6060
go tool pprof -http=: "http://localhost:6060/debug/pprof/profile?seconds=10"
curl -o heap.pb.gz http://localhost:6060/debug/pprof/heap
curl "http://localhost:6060/debug/pprof/goroutine?debug=2"   # readable stacks
```

Add `-H "Authorization: Bearer $TOKEN"` (or `X-Debug-Token: $TOKEN`) when a
token is set.

## Endpoints

| Path | Returns |
|---|---|
| `/debug/health` | `ok` |
| `/debug/pprof/goroutine` | Goroutine profile. Binary by default; `?debug=1` or `?debug=2` for text. |
| `/debug/pprof/heap` | Heap profile, same formats. |
| `/debug/pprof/profile` | CPU profile for `?seconds=N` (default `CPU_PROFILE_SECONDS`). Longer than `CPU_PROFILE_MAX_SECONDS` is a 400; a second concurrent request is a 429. The profile ends early if the client disconnects. |

Only `GET` is allowed (405 otherwise). With a token set, a request without it
gets 401. Nothing else from `net/http/pprof` is exposed: no index, `cmdline`,
`trace` or symbol lookup.

## Configuration

| Variable | Default | Meaning |
|---|---|---|
| `DEBUG_PPROF_ENABLED` | `false` | Serve at all. |
| `DEBUG_PPROF_ADDR` | `:6060` | Listen address. |
| `DEBUG_PPROF_TOKEN` | empty | Required token; empty means no token. |
| `DEBUG_PPROF_CPU_PROFILE_SECONDS` | `10` | CPU profile length without `?seconds=`. Capped at the maximum. |
| `DEBUG_PPROF_CPU_PROFILE_MAX_SECONDS` | `10` | Longest CPU profile allowed. |

A length of 0 or less means the default, 10.

## API

| Name | What it does |
|---|---|
| `NewWithDefaultEnvPrefix()` | Configured from `DEBUG_PPROF_*`. |
| `New(prefix)` | Configured from `<prefix>_*`. Panics if a variable cannot be parsed. |
| `NewWithOptions(opts...)` | Configured in code and **enabled**; `WithEnabled`, `WithAddr`, `WithToken`, `WithCPUProfileSeconds(default, max)`. |
| `(*Server) Start(ctx) error` | Binds the address synchronously (failure: `pprof: listen: …`), then serves in the background. Does nothing when disabled. A second call returns an error. |
| `(*Server) Stop(ctx) error` | Graceful shutdown; nothing to do if never started. |
| `(*Server) Addr() net.Addr` | The address actually listened on; nil before `Start` or when disabled. |
| `(*Server) OK(ctx) error` | The error the server stopped with, if any. |
| `(*Server) Enabled() bool` | Whether `Start` serves. |
| `(*Server) Handler() http.Handler` | The routes with their checks, to test them or mount them elsewhere. |

## Things to know

- Profiles reveal code paths and memory contents. Keep the port inside the
  cluster and set a token where the network is shared.
- A CPU profile costs a few percent of CPU while it runs; the maximum length
  bounds that.
- The server does not log. Log `srv.Enabled()` and `srv.Addr()` at startup if
  you want them.

## See also

- [metric](../metric) — the metrics server, with the same `Start`/`Stop`/`OK`
  methods.
