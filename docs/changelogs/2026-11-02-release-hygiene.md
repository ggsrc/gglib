# chore: release hygiene before the first per-module release

Tracking: alva-ai/mono-meta#1115 (layer L1, gglib). First of four PRs before
`v0.1.0`: hygiene (this) → dependency and vulnerability updates → CI → release
script.

## 1. Current state

- 16 modules, no tags, no CI, no LICENSE, empty README; `main` unprotected.
- With `GOWORK=off`, 8 of 16 modules failed `go mod tidy -diff` and 3
  (`grpc`, `metric`, `resource/hatchet`) failed `go build -mod=readonly`
  because of missing go.sum lines. This was hidden by local, uncommitted
  `go.work` files; only `go.work.sum` was committed.
- `grpc` imports `interceptor/grpc/errlog`, but its go.mod required an
  `interceptor` commit (`d92191b`) that predates errlog (`50cf7c9`, #22).
- `resource/cache` required `goodns` at `d7f8c73`, a commit that exists only
  on branch `tobin/dev`, not on `main`.
- Internal requires pointed at five different January commits, so a consumer
  that upgraded only `app` received older sibling code.
- `mctx` required `bytedance/sonic v1.14.2`, which does not compile on Go 1.26
  or later; `app`, `grpc` and `interceptor` inherited it.
- `otel/` had Go files but no go.mod (deleted in 15e1b11, #5), so it belonged
  to a synthesized root module and could not be imported as a module.
- `zerolog/log` `TestLog` panicked unless `UPTRACE_DSN` or `UPTRACE_DISABLED`
  was set: it called `uptrace.ConfigureOpentelemetry()` with no DSN.

## 2. Behavior after this change

- Every module (17, including `otel`) declares `go 1.27.0` and passes
  `go mod tidy -diff`, `go build -mod=readonly`, `go vet`, `go test -race` and
  `golangci-lint` on its own with `GOWORK=off`.
- Every internal require points at the current `main` commit `b474e43`
  (`v0.0.0-20261013145132-b474e434e730`). Upgrading any one module now brings
  sibling code that matches it.
- `github.com/ggsrc/gglib/otel` is a module again (deps: env, zerolog; otel
  v1.39.0 family, matching `zerolog`).
- No library code changes. The only Go change is the test fix in
  `zerolog/log/log_test.go`.

## 3. Decisions

Approved on the plan page (mono-meta#1115, 2026-11-02):

- D2: Apache-2.0. The LICENSE is the standard text, the same file the other
  ggsrc repositories use.
- Go baseline 1.27 (`go 1.27.0`). D3 originally chose 1.26; Yumin raised it to
  1.27 on 2026-11-02. Consumer impact: production images currently build with
  Go 1.26.10, so each service moves its toolchain (Dockerfile, CI) and its own
  `go` directive to 1.27 when it adopts gglib `v0.1.0`. Raising a service's
  `go` directive also changes its GODEBUG defaults; check that per service.
- D4: `otel` becomes a real module.
- D5: the first release tags every module `v0.1.0` on one commit. Pointing
  internal requires at current `main` follows from D5: every consumer moves to
  current `main` content anyway, and a module upgraded alone should not pull
  in January-era siblings. Only `grpc → interceptor` and
  `resource/cache → goodns` were strictly broken; the rest are aligned for
  consistency.
- Third-party versions are not upgraded here except `sonic` (needed to build
  on Go 1.26+). Vulnerable dependencies are the next PR.
- `go.work.sum` is deleted and ignored. `make work` creates a local workspace.

## 4. Changes

- `*/go.mod`, `*/go.sum`: `go 1.27.0`; internal requires at `b474e43`;
  `sonic v1.15.0` (`sonic/loader v0.5.0`) in mctx, interceptor, grpc, app;
  `app` lists `google.golang.org/grpc` as a direct require; tidy.
- `otel/go.mod`, `otel/go.sum`: restored from 15e1b11^, then tidied.
- `zerolog/log/log_test.go`: drop the Uptrace call; the test only needs a span
  context. `zerolog` no longer requires `uptrace-go`, and five modules lose it
  as an indirect requirement.
- `Makefile`: `GOWORK=off` for every target; new `modules`, `tidy-check`,
  `build`, `vet`, `test`, `work`; `go-mod-tidy` kept as an alias.
- `.golangci.yml`: `run.go: '1.27'`; `goconst` excluded in tests (like `gosec`),
  because golangci-lint v2.14 flags repeated literals in table tests.
  `.gitignore`: `go.work.sum`.
- New: `LICENSE`, `README.md`, `SECURITY.md`, `.github/CODEOWNERS`.

## 5. Verification

Toolchain Go 1.27.3 (`GOTOOLCHAIN=local`), `GOWORK=off`, golangci-lint
v2.14.0 built with go1.27.3 (v2.8.0 cannot load Go 1.27 modules). The same
checks also passed earlier on Go 1.26.10 with the `go 1.26.0` directive.

- `make tidy-check build vet` — all 17 modules pass.
- `make test` — all 17 modules pass, with no Uptrace variables set.
  Before the test fix, `zerolog/log` failed the same way on `origin/main`.
- `make lint` — 0 issues in all 17 modules.
- `make work` creates a `go.work` with 17 modules; building `app`, `grpc` and
  `otel` through the workspace passes. The generated files are ignored by git.
- Third-party requires: the only changes are the sonic upgrade and removed
  indirect requirements (uptrace-go and its exporter tree, agoda otelzerolog,
  grpc-gateway, older genproto/x/net/grpc lines that are no longer needed).

## 6. Human decisions

None open for this PR. The ruleset for `main` (who approves PRs authored from
the Stumble account) is decided before it is enabled.

## 7. Outcome

- [x] Hygiene changes made and verified locally.
- [ ] PR opened, CI-free review by Yumin, merged.

## 8. Remaining work

- Dependency and vulnerability updates (govulncheck: resource/hatchet,
  zerolog, http, resource/wpgx → wpgx v0.5.0).
- CI workflow (per-module matrix) and required check.
- Release script, per-module CHANGELOG, `v0.1.0` tags in release order.
- Repository settings: rulesets for `main` and `*/v*` tags.
