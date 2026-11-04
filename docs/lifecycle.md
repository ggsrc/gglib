# Lifecycle of a service

How the gglib modules work together over the life of one process: what
happens at startup, how the Kubernetes probes answer at each stage, what
happens when a dependency fails, and the order of shutdown. The code is the
example from the [README](../README.md):

```go
a, _ := app.New(ctx, app.WithServerName("my-service"))         // 1
rm := resource.NewResourceManager([]resource.Resource{db, kv}) // 2
_ = a.Init(ctx, rm)                                            // 2
srv := grpc.NewServerWithDefaultEnvPrefix()                    // 3
pb.RegisterMyServiceServer(srv.Server(), handler)              // 3
_ = a.Run(ctx, srv, rm)                                        // 4
```

## Probes at each stage

Kubernetes uses two probes. When **liveness** (`/health/alive`) fails, it
restarts the container. When **readiness** (`/health/ready`) fails, it stops
sending requests to the pod but does not restart it. gglib's rule: a failing
dependency makes the pod unready, never restarts it.

| Stage | What happens | Liveness | Readiness |
|---|---|---|---|
| `app.New` | Logging and OpenTelemetry are set up; the metrics server starts on :4014 and the health server on :8080, marked not ready. | pass | fail |
| `app.Init` | Connects to the database and Redis. On failure it logs and retries after 1 s, 2 s, 4 s, … up to 30 s, until it succeeds. | pass | fail |
| Your code | Builds handlers from the connected dependencies and registers the gRPC services. | pass | fail |
| `app.Run` starting | Starts the resources, adds their checks, binds the gRPC port, serves, then marks the pod ready. | pass | fail → pass |
| Running | Every 5 s all checks run (database ping, Redis ping, metrics server). | pass; fails only after checks have failed for more than 3 min | follows the checks |
| SIGTERM | Ordered shutdown (below). | pass until the end | fail at once |

## When a dependency fails

**Redis is down for 20 seconds.** The next round of checks (within 5 s) fails
and the pod becomes unready, so it receives no new requests. When Redis is
back, the next round passes and the pod is ready again. No restart; the Redis
client drops broken connections and reconnects by itself.

**The database is down for 10 minutes.** For the first 3 minutes the pod is
only unready. After 3 minutes of failing checks liveness fails, and Kubernetes
restarts the container after its failure threshold (with a 10 s period and a
threshold of 3, about 30 s later). The new process answers the probes at once
(alive, not ready) and keeps retrying in `Init`; when the database is back it
becomes ready within seconds.

**A wrong password is deployed.** `Init` keeps retrying and logs
`app: resources not initialized; retrying` on every attempt. The new pods stay
unready, so the rollout stops and the old pods keep serving. There is no crash
loop with growing back-off.

**The gRPC port is taken.** `Run` fails to bind it, shuts down and returns the
error; the process exits.

**A check hangs.** Each round waits at most 5 s; a timeout is a failure. A
check that has not returned from the previous round is not started again and
counts as failed. If the check loop itself stops for more than 20 s (3
intervals plus 1 timeout), liveness fails and Kubernetes restarts the pod.

**Only a read replica is down.** Health checks ping the primary only, so the
pod stays ready. Queries sent to that replica fail; the others succeed.

Why 3 minutes: a restart is for a process that cannot recover by itself after
its dependencies came back. Short blips should not restart anything, since the
Postgres and Redis clients reconnect on their own. Tune it with
`HEALTHCHECK_STUCKAFTER` (see [health](../health)).

## Shutdown order

On SIGTERM or SIGINT (or when `Run`'s context ends, the gRPC server stops, or
the health server fails), `app` stops in this order. Steps 1–4 share one
shutdown timeout, 30 s by default (`app.WithShutdownTimeout`).

1. **Readiness fails.** Kubernetes stops sending new requests to the pod.
2. **gRPC stops gracefully.** No new connections or calls; in-flight calls
   finish. When the timeout runs out, the rest are cut.
3. **Resources stop in reverse registration order.** Workers that use the
   database stop before the database pool closes. Dependencies close only
   after all calls have finished, so no handler uses a closed connection.
4. **The metrics and health servers stop.**
5. **Buffered logs and spans are flushed**, with a separate 5 s timeout, so the
   flush runs even if the steps above used the whole timeout, and the
   shutdown's own log lines are exported.

Liveness passes during the whole shutdown, so a long drain is not mistaken for
a hung process.

### Deployment notes

- Kubernetes kills the process `terminationGracePeriodSeconds` (default 30 s)
  after SIGTERM. Shutdown can take up to the shutdown timeout plus 5 s, so set
  the grace period above that; otherwise the last log lines and spans may be
  lost.
- A service that waits for long-lived streams to end needs a longer shutdown
  timeout, and a grace period to match.
- Point the liveness probe at `/health/alive` and the readiness probe at
  `/health/ready` on the health port (8080 by default).

## See also

- [app](../app) — the API and its options.
- [health](../health) — the probe rules and their configuration.
- [resource](../resource) — init, start and stop semantics.
