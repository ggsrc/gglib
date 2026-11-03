# feat(app): New, Init, Run — probes from the start, init retries, ordered shutdown

Tracking: alva-ai/mono-meta#1115 (gglib, last PR before `v0.1.0`). Wires
in G5 (`otel.Setup`, `zerolog.Setup`), G9/G11 (log format and OTLP logs),
G12 (stop order) and closes the gap found with G13.

## 1. Current state

- `NewApp(opts...)` took a gRPC server and a resource manager; `Start(ctx)`
  set up Uptrace and `zerolog.InitLogger`, started the gRPC and health
  servers in goroutines, started the resources and the metrics server, and
  waited for a signal or a health-server error.
- The gRPC server's error (`grpcErrCh`) was never read: a port conflict left
  a process that served nothing and passed its probes.
- Readiness could pass before the gRPC port was bound.
- Services initialize their resources before `NewApp`. If the database is
  down at start-up, `Init` fails, the process exits and Kubernetes restarts
  it with a growing back-off (up to 5 minutes), so a pod can stay down
  minutes after the database is back. With G13 (`StuckAfter` 3 minutes) a
  long outage restarts pods, which then hit this back-off.
- alva-backend's worker-only mode re-implements the metrics server, the
  health server and signal handling, because `NewApp` requires gRPC.
- Telemetry was not flushed on exit.

## 2. Behavior after this change

```go
a, err := app.New(ctx, app.WithServerName("alfs"), app.WithDebug(c.Debug))
if err != nil { log.Fatal()... }                 // port conflict, bad telemetry config
rm := resource.NewResourceManager([]resource.Resource{db, cache})
if err := a.Init(ctx, rm); err != nil { ... }    // retries until ctx is done
srv := buildServer(db, cache)                    // registers the gRPC services
err = a.Run(ctx, srv, runtimeRM)                 // srv may be nil (worker)
```

- B1 `New` sets up logging and telemetry and serves `/metrics` and the
  probes at once: alive, not ready.
- B2 `Init` retries a failing resource with a back-off from 1 s to 30 s and
  does not initialize the resources that already succeeded
  (`ResourceManager.Init` now resumes). The pod stays alive and not ready:
  no traffic, no restart, however long the outage. It returns only when its
  context is done.
- B3 `Run` starts the resources, adds their checks to the probes, binds
  and serves gRPC (when given), then marks the pod ready.
- B4 `Run` returns, after stopping everything, on SIGINT/SIGTERM, when its
  context is done, or when the gRPC or health server fails.
- B5 `Stop`: readiness fails; RPCs drain (force-stopped at the shutdown
  timeout); resources stop, last registered first; metrics and health
  servers stop; telemetry is flushed last with its own 5 s deadline.
  Liveness passes until the end.
- B6 Telemetry service name and version come from the deployment
  (`OTEL_SERVICE_NAME`, `GIT_SHA`, …); `WithServerName` and
  `WithServerVersion` are only fallbacks. Every deployment sets
  `OTEL_SERVICE_NAME` (for example `alfs-server`, `alva-backend-worker`),
  which differs from the names services pass to `WithServerName`; an
  override would have renamed every service in Uptrace.

## 3. Decisions

- D1 Replace `NewApp`/`Start` instead of keeping both: first release, one
  way to start a service. Every service touches `main` for v0.1.0 anyway
  (G12 registration order, `WithOTEL`).
- D2 `Init` retries for as long as its context allows; it cannot tell a
  dependency that is down from a wrong password. A misconfigured rollout
  therefore stalls not ready (visible in the rollout and in the error log
  of every attempt) instead of crash-looping.
- D3 `Init` does not add the resources' checks to the probes and `Run` does
  not initialize: services initialize one manager and run another that also
  holds workers (alfs, worldline), as they do today.
- D4 A second `otel.Setup` in the process (`otel.ErrAlreadySetup`) is not an
  error for `New`: the caller owns telemetry then.
- D5 `zerolog.InitLogger` and its hook are removed: only gglib/app called
  them.

## 4. Changes

- `app/app.go`, `app/options.go` rewritten; `app/app_test.go` (new);
  `app/shutdown_test.go` adapted (waits for the RPC to end, so that no
  handler logs into the next test).
- `resource`: `Init` resumes after a failure.
- `health`: `Listen`, `Serve`, `Shutdown` (waits for the check loop).
- `grpc`: `Server.Listen`.
- `otel`: `ErrAlreadySetup`.
- `zerolog`: `InitLogger` and the OTLP hook removed.
- `.golangci.yml`: the last temporary exclusion (app contextcheck) is gone.
- Coverage floors: app 16.7 → 96.0, health 96.9 → 98.0, zerolog 97.2 →
  97.5.

## 5. Verification

- `make tidy-check build vet test lint cover vulncheck`: all 13 modules
  pass; lint 0 issues; no vulnerabilities. app 96.8%, health 100%.
- app tests: probes alive/not ready after `New`; `Init` retries and resumes,
  gives up with the context; `Run` serves, readiness follows the resources,
  stops in reverse order; worker mode without gRPC; SIGTERM; resource start
  failure; gRPC port conflict; gRPC server stopping; health server failure;
  port conflicts in `New`; telemetry set up elsewhere; telemetry flushed
  last with its own deadline; telemetry fallbacks; options; drain before
  closing dependencies, and force-stop at the deadline.
- 100 runs with `-race`, and 3 × 15 in parallel with `-cpu=1,2`: no failure,
  no race.

## 6. Migration (Alva, mono-meta#1116)

| Service | Change |
|---|---|
| alfs | `ggapp.New` first; `a.Init(ctx, initResourceManager)` instead of `initResourceManager.Init` + `log.Fatal`; `a.Run(ctx, grpcServer, resourceManager)`; drop `withOptionalOTEL`; register dependencies before the maintenance controller (G12) |
| worldline | same; the reordering wrapper can go (G12) |
| stream-relay | same; register the cache before its users (G12); `WithShutdownTimeout(66 * time.Minute)` instead of setting the variable |
| alva-backend | API mode as above; worker-only mode becomes `a.Run(ctx, nil, rm)` and drops its own metrics server, health server and signal handling |

All four: remove `WithOTEL` and `WithEnv`; `WithServerVersion` only if the
image has no `GIT_SHA`.

## 7. Outcome

- [x] Implemented and verified locally.
- [ ] CI green; merged.
