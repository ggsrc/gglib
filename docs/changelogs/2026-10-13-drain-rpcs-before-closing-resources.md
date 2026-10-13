# fix(app): drain RPCs before closing shared resources

## 1. Background

STG Backend sha-2ed90ed received SIGTERM while an API-run script was active.
The script completed, but credits settlement failed with closed pool and the
API returned500. App.Stop currently runs gRPC shutdown and resource closure
concurrently, closing Postgres while an in-flight handler still needs it.

## 2. Behavior

Stop readiness first; drain gRPC before stopping its resource dependencies.
Keep deadline-triggered force-stop behavior. Stop metrics after resources and
return shutdown errors without skipping remaining cleanup. Existing App API
and resource interfaces stay unchanged; no new background-worker API.

## 3. Decisions and scope

Fix the shared App lifecycle, then update Backend's app module pin only after
this provider PR merges. User authorized implementation/publication/merge.
No PRD promotion. Backend worker-only lifecycle is outside this shared App.
Its existing cleanup remains intact; do not claim all background jobs gain a
new admission protocol. Generic resource producers still own their Stop contract.

## 4. Implementation

Sequence health Stop, gRPC Shutdown, resource Stop, metrics Stop. Collect errors
with errors.Join. Use real in-flight gRPC regressions with a dependency that
rejects writes after closure, plus a forced-deadline test. Avoid data-store mocks
that merely assert call order.

## 5. Validation

Canonical repository formatters from .golangci.yml via golangci-lint fmt.
Canonical full multi-module lint via make lint. Module go1.24.7; use installed
Go1.24.7 unless dependency Go requirements require higher. Run app tests/race
with standalone modules and a disposable workspace for the real local gRPC
module. No published replace/workspace artifacts.

## 6. Checklist

- [x] Reproduce premature closure with in-flight gRPC test.
- [x] Sequence dependency shutdown; prove normal and deadline behavior.
- [x] Full formatting/lint and meaningful tests; audit exact diff.
- [ ] Publish provider PR, resolve checks, merge as authorized.
- [ ] Update Backend pin from merged commit, verify, publish and merge consumer.
- [ ] Observe STG image and live behavior through Sentinel; keep any gap explicit.

## 7. Evidence

RED: original app fails both real-gRPC regressions (premature resource closure;
forced-shutdown error discarded). GREEN: GOWORK=off GOTOOLCHAIN=go1.24.7
GOCACHE=/private/tmp/gglib-shutdown-cache go test -race ./... passes in app.
Canonical format: golangci-lint v2.8.0 fmt on changed files using repository
configuration. Full canonical make lint passes all16 modules in a disposable,
unpublished workspace. Standalone app lint passes as well.

Full16-module tests pass with UPTRACE_DISABLED=1: the existing zerolog TestLog
requires Uptrace configuration otherwise and panics without a DSN. No live
telemetry credentials needed. First standalone full lint cannot load grpc's
pre-existing incomplete go.sum; disposable local-module workspace resolves it.
Full lint also exposed existing import/line formatting in grpc, resource and
interceptor files. Included only formatter/blank-line changes to make that
full scope clean; no behavior changes in those modules. No go.work or module
pin/replace changes in this provider PR.


## 8. Review

Main-agent review: API unchanged; readiness stopped before drain; dependencies
remain available until grpc.Shutdown returns; deadline force-stop preserved;
errors returned while subsequent cleanup still runs. Metrics remain until
drain and resource cleanup finish. Two real-gRPC regressions cover the observed
failure and deadline exit. Baseline format changes are semantically inert.
No PRD state change, telemetry secret or unrelated user checkout changes.
