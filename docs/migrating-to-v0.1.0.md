# Migrating to v0.1.0

v0.1.0 is the first tagged release. Before it, consumers depended on untagged
commits (pseudo-versions such as `v0.0.0-20260421072045-14479fe14411`). This
page lists what behaves differently and what was removed; each module's
`CHANGELOG.md` has the details.

All modules require Go 1.27.

## Behaviour changes

| Module | Before | v0.1.0 |
|---|---|---|
| [app](../app) | `NewApp` + `Start`. Resources were initialized before the probes started, and a failure exited the process (restart loop with growing back-off). A failure to bind the gRPC port was ignored; the pod reported ready before gRPC was bound. Buffered spans were not flushed on exit. | `New` / `Init` / `Run`. Initialization retries while the pod stays alive and unready; a port error exits; the pod is ready only after everything started; ordered shutdown flushes telemetry; telemetry is set up by [otel](../otel). |
| [resource](../resource) | Stopped in registration order; started all resources in parallel; a retried `Init` initialized resources that had already succeeded again. | Stops in **reverse** order; starts one by one; `Init` resumes at the resource that failed. If you registered users before their dependencies to get the right shutdown order, reverse your registration now. |
| [health](../health) | Liveness followed the dependency checks, so an outage restarted pods; a hung check never timed out; the state was not synchronized; ready before any check ran. | Liveness fails only when checks are stuck or failing for longer than `StuckAfter`; checks time out; ready only after a passing round. |
| [env](../env) | Only exactly `prd` was production. | `prd`, `prod`, `production` in any case; unknown names count as production. |
| [grpc](../grpc), [interceptor](../interceptor) | A panic in a streaming handler crashed the process. Request bodies were recorded in spans whenever `ENV` was not `prd`. All incoming headers were forwarded to callees, so tokens were appended to the caller's own. Status details were logged incorrectly. | Streams have panic recovery. Bodies are recorded only with `WithServerDebug(true)` / `GRPC_DEBUG=true`. Header forwarding has an allowlist (log-only by default), and headers set by the caller replace incoming ones. Status details are logged as `sub_errors`. |
| [zerolog](../zerolog) | Without setup, `log.Ctx` returned a disabled logger and logs were silently dropped. Standard-error logs had no `trace_id`. OpenTelemetry log records lost most fields. | `log.Ctx` always writes. Lines carry `trace_id` and `span_id`. Records carry every field. |
| [mctx](../mctx) | Large integers in `Extras` lost precision on every hop; no size limit; several unused fields. | `json.Number`; 32 KiB limit; unused fields removed. |
| [resource/cache](../resource/cache) | An unreachable Redis exited the process. TLS certificate verification was off in managed-cache mode. Every user got a 50 MiB in-memory cache. | Errors are returned and retried by `app.Init`. TLS verifies certificates. `Redis` is a client-only resource. |
| [resource/wpgx](../resource/wpgx) | wpgx v0.4.7, which had an SQL injection vulnerability; a failed `Init` followed by a retry was taken as success. | wpgx v0.5.0; a failed `Init` cleans up and can be retried. |
| [otel](../otel) | Exported to an empty host when no endpoint was set; the service name was overwritten with `UnknownService`. | Exports only to configured endpoints; deployment configuration decides the service name. |

## Removed

| Removed | Why |
|---|---|
| `goodns` | Used only by Redis sentinel mode. It reported success when a name did not resolve and panicked without nameservers. Go's resolver is used instead. |
| `http`, `interceptor/http` | No users. They recorded whole requests, including URLs with passwords, in spans and trusted `X-Forwarded-For` blindly. |
| `profiling` | No users. Its configuration merge always failed, so its configuration never took effect. |
| `resource/hatchet` | Built on a deprecated SDK with 12 reachable vulnerabilities. Use the Hatchet SDK directly. |
| `resource/goroutine` | Left out of `resource` v0.1.0. It is back as its own module, [resource/goroutine](../resource/goroutine), with the same import path and API: `go get github.com/ggsrc/gglib/resource/goroutine@latest`. |
| `interceptor/grpc/ratelimit` | No users; every constructor panicked. |
| `interceptor/grpc/errors` | Its error types were not produced anywhere; `errlog` now logs standard gRPC status details. |
| `metric.RecordEvent` | Wrote to an unsynchronized map concurrently and could crash the process. |
| `app.NewApp`, `Start`, `WithOTEL`, `WithEnv`, `WithGRPCServer`, `WithResourceManager` | Replaced by `New` / `Init` / `Run` and their options. |
| `zerolog.InitLogger` | Replaced by `Setup`. |

## Dependency updates that are visible

- dcache v0.5.0: `Set` now invalidates the in-memory copies in other instances
  (before, only `Invalidate` did).
- wpgx v0.5.0: span attributes follow current OpenTelemetry semantic
  conventions, for example `db.system` → `db.system.name`.
- grpc-go 1.85, with security fixes.
