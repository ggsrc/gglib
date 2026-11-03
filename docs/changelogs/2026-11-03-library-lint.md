# chore: library golangci-lint configuration and non-behavioral fixes

Tracking: alva-ai/mono-meta#1115 (layer L1, gglib). Third PR before `v0.1.0`,
stacked on #29.

## 1. Current state

- `.golangci.yml` enabled 14 linters plus the standard set, excluded most of
  staticcheck's style checks, and ran `golines`. It did not ask for doc
  comments, did not forbid `log.Fatal`, `os.Exit` or `fmt.Print` in library
  code, and allowed `github.com/pkg/errors`.
- With the stricter configuration below, the 13 modules had 342 issues: 170
  missing or malformed doc comments, 37 `interface{}`/loop modernisations, 25
  comments without a final period, 24 staticcheck findings, 23 testify
  misuses, and about 20 real problems (process exits inside `Init`, an
  unchecked type assertion, a span that is not ended on every path, `init()`
  functions, `pkg/errors`).

## 2. Behavior after this change

No library behavior changes. Every module passes the new configuration with
0 issues.

- Every exported identifier in env, mctx, metric, resource, health, grpc,
  interceptor, resource/wpgx, resource/cache, app and zerolog has a doc
  comment, and every package has a package comment (new `doc.go` files where
  a package had none).
- `resource` and `resource/cache` use the standard `errors` and
  `fmt.Errorf("...: %w")` instead of `github.com/pkg/errors`. Error texts are
  unchanged.
- `grpc.Server.Start` listens through `net.ListenConfig` (same address, same
  errors).
- Tests use `require` for error assertions, so a failed precondition stops the
  test instead of cascading.
- `make lint-fix` now runs in every module and fails at the end if any module
  still has issues; before, it stopped at the first module with an unfixable
  issue.

## 3. Decisions

- The rules target a public library: documented API, no process exits or
  stdout printing, standard-library errors, correctness, security and test
  linters. Formatting is gofumpt, gci and goimports; `golines` is dropped
  (line length is not a Go convention and it caused churn).
- Disabled on purpose, with the reason in the config: `fieldalignment` and
  `shadow` (noise), gocritic `hugeParam` (switching `zerolog.Logger` or
  `mctx.User` parameters to pointers would break callers), and revive's
  stutter check (`context.ContextUnaryServerInterceptor`,
  `resource.ResourceManager` are public names that services use).
- Names that break Go naming rules but are referenced by services keep their
  names, with a `//nolint` that says why: `metadata.CTX_KEY_METADATA`,
  `metadata.CTX_KEY_APP_CTX` (alva-backend, alva-gateway) and
  `health.HttpRouter` (renamed in the health rework).
- Code whose fix changes behavior, or that G5 replaces, is excluded by path in
  a "Temporary" block. Each rule names the PR that removes it:
  - `otel/`, `utils/`: replaced by the G5 entry point, so their old API is
    not documented here.
  - `app/app.go` contextcheck: telemetry setup moves to G5.
  - `env/` `init()`: becomes explicit initialization in the env PR.
  - `resource/resource.go` `fmt.Printf`: the resource PR.
  - `resource/cache` `log.Fatal`, context and TLS override: the cache PR.

## 4. Changes

- `.golangci.yml`: rewritten (see section 3).
- `Makefile`: `lint-fix` covers every module.
- Doc comments across 11 modules; new `doc.go` in env, grpc, health, mctx,
  metric, resource and zerolog.
- Mechanical fixes from `golangci-lint run --fix`: `any`, `wg.Go`,
  `maps.Copy`, `strings.SplitSeq`, `errors.New`, periods, import order.
- Manual: `pkg/errors` removed; `appendCombine`, `sloppyReassign`,
  `net.ListenConfig`, `hash.Write` result, testify `require`, `t.Helper`,
  `//nolint` explanations.

## 5. Verification

Go 1.27.3, `GOWORK=off`, golangci-lint v2.14.0:

- `make tidy-check build vet test lint` — all 13 modules pass; lint reports
  0 issues in each.
- `make lint-fix` exits 0 and leaves no diff.

## 6. Human decisions

None.

## 7. Outcome

- [x] Configuration applied, all modules clean, checks pass.
- [ ] PR reviewed by Yumin, merged after #29.

## 8. Remaining work

- Each per-module PR removes its "Temporary" exclusion.
- CI runs `make lint` per module (next PR).
