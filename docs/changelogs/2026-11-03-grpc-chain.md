# fix(grpc, interceptor): stream panic recovery, chain order, bodies only in debug

Tracking: alva-ai/mono-meta#1115 (gglib, per-module PR). Stacked on #31.
Decision G6 = A (request bodies off by default). The request-context
interceptor (G3) is a separate PR.

## 1. Current state

Review findings (rpc.md), proven by tests:

- G-1: the server stream chain had only Prometheus; a panic in a streaming
  handler crashed the process. alfs and alva-backend expose streaming RPCs.
- G-3, I-R2: unary panics returned a plain error (`Unknown` on the wire) and
  were missing from `grpc_server_handled_total`.
- G-6: `grpc.StreamInterceptor` in server options panicked; a dial option
  `grpc.WithUnaryInterceptor` replaced the client chain silently.
- G-7: hand-written chain functions raced under concurrent handler calls.
- G-5, G-13: call logs ignored the request context; unknown levels panicked.
- I-L1: bodies (passwords, tokens) went into spans whenever `ENV != "prd"`.
- I-E1, I-E3: cancellations logged at Error; details decoded into a legacy
  type.
- G-12, I-R3, I-X1: Alva-specific helpers, a Sentry handler and a legacy
  error package with no users.

## 2. Behavior after this change

- Server unary chain, outermost first: metrics → recovery → request context
  → error log → call log → [bodies, only with Debug] → user interceptors.
  Stream chain: metrics → recovery → error log → user interceptors.
- Panics become `codes.Internal` "internal error" and are counted.
- Interceptors from options compose (`Chain*` APIs); a single
  `grpc.UnaryInterceptor`/`StreamInterceptor` or dial `WithUnaryInterceptor`
  runs before the standard chain.
- Bodies in spans only with `WithServerDebug(true)`, cut at 4 KiB.
- errlog: context errors map to their codes; `grpc_code` field;
  `sub_errors` = protojson details.
- Removed: unused helpers and options listed in the module changelogs.

Consumer impact:

- alfs, alva-backend and worldline pass `WithServerDebug(<config>)`; with
  their config unset, STG no longer records bodies in spans. Set
  `<PREFIX>_GRPC_DEBUG=true` (their own config key) on a service to turn it
  back on.
- alva-backend's own stream recovery stays valid (it now runs inside
  gglib's).
- Dashboards: `grpc_server_handled_total` gains the previously missing
  panicked calls under `grpc_code="Internal"`.
- Log field `sub_errors` changes shape (protojson objects) and is absent
  when there are no details; new field `grpc_code`.

## 3. Decisions

- G6 = A.
- Metrics outermost so that recovered panics are counted; recovery before
  the request context so that a panic there is caught too.
- Removals follow G1 (no users in the 45-repository snapshot).
- Kept for now: forced gzip and plaintext on the client (in-cluster
  defaults; changing them is a wire change), go-grpc-prometheus (its metric
  names are load-bearing in 10 dashboards and 3 alerts).

## 4. Changes

- `grpc/{server,client,options,util}.go`; `grpc/grpc_test.go` (real server
  and client over TCP with the gRPC health service).
- `interceptor/grpc/{recovery,errlog,log}`; `interceptor/grpc/errors`
  deleted; new tests.
- Coverage floors (`<module>/.coverage-floor`): grpc 0 → 98.8, interceptor 43.8 → 75.0 (the
  request-context package is covered by the G3 PR).

## 5. Verification

- grpc tests: panic → Internal and counted; stream panic recovered and the
  server keeps serving; option interceptors compose; dial interceptor does
  not replace the chain; bodies only with Debug (ENV=stg); verbose call logs
  through the context logger; logger levels.
- `make tidy-check build vet test lint cover` — all 13 modules pass.

## 7. Outcome

- [x] Implemented and verified locally.
- [ ] CI green; reviewed by Yumin.
