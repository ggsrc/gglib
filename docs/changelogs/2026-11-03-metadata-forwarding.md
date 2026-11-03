# feat(interceptor): forward identity and tracing headers only (G3), log-only first

Tracking: alva-ai/mono-meta#1115. Stacked on #37. Decision G3 = A; the
whitelist and rollout are decision G17 on the gglib status page (this PR
implements the recommended option A: log-only by default). Census:
`/home/forge/refactor-handoff/2026-11-02/g3-metadata-census.md`.

## 1. Current state

- The client interceptor copied every incoming header into each outgoing
  call and `metadata.Join`ed explicit values with forwarded ones, so a
  callee could receive the service's token and the user's token side by
  side (review I-C1, proven).
- Census of all services: only `app-ctx-bin` is needed across several hops
  (gateway → backend → alfs; gateway → backend → trex → jagent; …);
  `x-request-id` (alfs log correlation) and `traceparent` (trace continuity
  through trex) also rely on forwarding. No Go code sends or reads
  `authorization`, cookies or API keys in gRPC metadata. Every reader takes
  the first value.
- Malformed app contexts were logged at Error with the raw header.
- Streaming servers got no app context.

## 2. Behavior after this change

- Explicit outgoing values replace forwarded ones; transport headers are
  never forwarded; `ForwardKeys` are forwarded.
- Other keys: forwarded and logged by name once per process
  (`ForwardLogOnly`, default) or dropped (`ForwardAllowlist`, via
  `GGLIB_GRPC_METADATA_FORWARDING=allowlist` or
  `WithClientMetadataForwarding`).
- Server: malformed app context → Warn with its length; new stream
  interceptor wired into gglib's server stream chain.
- Client streaming calls still forward nothing (unchanged).

Consumer impact in log-only mode: none for readers; services log one line
per extra key name they forward. Rollout (Alva side): run in STG and PRD,
check the logged key names, then set the variable to `allowlist`. rick's
`grpcdial` forwards everything too; the trex → jagent and backend → trex
hops need the same change on the Alva side (preferably by calling this
interceptor).

## 4. Changes

- `interceptor/grpc/context/context.go` rewritten; tests (97.9%).
- `grpc`: stream chain adds the context interceptor;
  `WithClientMetadataForwarding`; end-to-end test with a real server.
- `.golangci.yml`: `fatcontext` excluded in tests (contexts captured in test
  closures).
- Coverage floors (`<module>/.coverage-floor`): interceptor 75.0 → 95.8, grpc 98.8 → 98.9.

## 5. Verification

- Tests: allowlist mode forwards exactly identity and tracing keys;
  log-only mode forwards and logs names without values, once; explicit
  values replace forwarded ones in both modes; transport keys dropped;
  environment default; stream server context; malformed header logged
  without content; end to end through `grpc.Client` and `grpc.Server`.
- `make tidy-check build vet test lint cover` — all modules pass.

## 6. Human decisions

- G17: confirm the whitelist and the log-only rollout.

## 7. Outcome

- [x] Implemented and verified locally.
- [ ] CI green; reviewed by Yumin.
