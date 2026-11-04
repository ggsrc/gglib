# health

```sh
go get github.com/ggsrc/gglib/health
```

Answers the two Kubernetes probes. Checks (database ping, Redis ping, …) run in
the background on an interval; **readiness** follows their results, while
**liveness** fails only when they have been failing for too long or when the
checks themselves are stuck. A dependency outage makes the pod unready, so it
stops receiving traffic, but does not restart it — restarting does not bring a
database back. [app](../app) runs this server and adds the resources' checks.

## Endpoints

| Path | 200 when | Otherwise |
|---|---|---|
| `GET /health/ready` | At least one check round has finished, no more than `ReadyCount` consecutive rounds have failed (default 0: the last round passed), the service marked itself ready, and shutdown has not started. | 503 |
| `GET /health/alive` | The service has not marked itself not alive, and the checks are not stuck: they have not been failing for longer than `StuckAfter` (default 3 min), and a round finished within the last 3 intervals plus 1 timeout (20 s by default). | 503 |

The body is empty; only the status code matters. Before the first round
finishes, readiness returns 503.

## Configuration (prefix `HEALTHCHECK_`)

| Variable | Default | Meaning |
|---|---|---|
| `HEALTHCHECK_PORT` | 8080 | Port. |
| `HEALTHCHECK_PROBEINTERVAL` | 5s | Time between check rounds. |
| `HEALTHCHECK_PROBETIMEOUT` | 5s | Time limit for one round. |
| `HEALTHCHECK_READYCOUNT` | 0 | Consecutive failed rounds still reported as ready. |
| `HEALTHCHECK_STUCKAFTER` | 3m | How long checks may keep failing before liveness fails. |
| `HEALTHCHECK_READY` | true | Initial value of the ready flag (`app` sets it to false at once). |
| `HEALTHCHECK_ALIVE` | true | Initial value of the alive flag. |

`StuckAfter` is the safety net for a process that cannot recover by itself
after its dependencies came back. Keep it well above short blips: database and
Redis clients reconnect on their own.

## API

| Name | What it does |
|---|---|
| `InitHealthCheck(checks...)` | Configured from `HEALTHCHECK_*`, with its own router. This is what `app` uses. |
| `NewWithDefaultEnvPrefix(router, checks...)`, `New(prefix, router, checks...)`, `NewWithConfig(conf, router, checks...)` | Other constructors. Pass an existing router to mount the probes on another HTTP server, or `nil` for a new one. The check loop starts at once. The environment-based ones panic if a variable cannot be parsed. |
| `HealthCheckable` | Interface with one method, `OK(ctx) error`. Resources and the metrics server implement it. |
| `Checkable` | A check function, `func(ctx) error`. |
| `(*Server) AddHooks(checks...)` | Adds checks later. `app.Run` uses it for the resources' checks. |
| `(*Server) Ready(bool)`, `Alive(bool)` | Set the service's own ready and alive flags. |
| `(*Server) Stop()` | Makes readiness fail from now on, to drain before shutdown. Liveness is not affected. |
| `(*Server) Listen(ctx)`, `Serve(lis)`, `Start()` | Bind the port, serve (blocking), or both. Separate so that a port conflict is reported before serving starts. |
| `(*Server) Shutdown(ctx)` | Stops the check loop, waits for the running round and in-flight probe requests (until `ctx` ends), then closes the HTTP server. |
| `(*Server) Close()` | Stops only the check loop. |
| `(*Server) Router()` | The router the probes are on. |
| `GoCheck(ctx, checks...)` | Runs checks in parallel and returns the first error. |

## How a round runs

- The first round runs at once, then one every interval.
- All checks run in parallel under the round's timeout; any error fails the
  round.
- A check whose previous call has not returned yet is not started again; it
  counts as failed ("health check from an earlier round has not returned"). A
  hung check therefore does not pile up goroutines.
- Logging: one `error` line when checks start failing (`health check failed`),
  `debug` lines while they keep failing, one `info` line when they pass again
  (`health checks pass again`, with the number of failed rounds). A long
  outage does not flood the logs.

## See also

- [docs/lifecycle.md](../docs/lifecycle.md) — how the probes answer during
  startup, outages and shutdown, and how long until Kubernetes restarts a pod.
- [resource](../resource) — where most checks come from.
