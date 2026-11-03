# feat(zerolog): Setup with one level knob, JSON or console, trace ids (G5)

Tracking: alva-ai/mono-meta#1115. Stacked on #38 (log.Ctx). Second G5
building block (the first is `otel.Setup`, #40). Decisions: G9 = A (JSON in
clusters), G11 = B (OTLP logs on by default).

## 1. Current state

Per the G5 survey: six services print colored console lines that Loki
cannot parse into fields, four print JSON; only four write `trace_id`, none
writes `span_id`; three interacting level knobs; the gglib OTLP bridge
(agoda) reads zerolog's private buffer by reflection.

## 2. Behavior after this change

- New `Setup` (see the module changelog): one level knob, JSON in clusters
  and console locally, `trace_id`/`span_id` from the event's context, caller,
  and an OTLP bridge through the official API, on by default (G11), that
  sends every field as an attribute.
- `InitLogger` (used by gglib/app today) keeps its previous behavior; only
  its OTLP path changes from agoda to the official API, feeding the
  provider uptrace installs. `WithBatchSize` is gone (the SDK batches).
- Services adopt `Setup` through the gglib/app rework (next PR) or by
  calling it in `main`.

## 3. Decisions

- `Setup` replaces the global logger; `InitLogger` does not. The format
  change for app-based services therefore lands with the app PR, after G9.
- Field names: `trace_id`, `span_id` (already used by rick and the
  dashboards' Loki queries).
- OTLP bridge (G11 = B, so every service sends its logs): a zerolog hook
  cannot read an event's fields, so the first version sent only the level
  and message, a regression from the agoda bridge, which sent fields. `Setup`
  now adds a second writer that receives each JSON line and turns it into a
  record (fields as attributes, `trace_id`/`span_id` as trace context,
  integers beyond int64 as exact strings). It decodes with
  `encoding/json/jsontext` token by token: about 5 µs and 24 allocations per
  line; decoding into a map with `encoding/json` cost 17 µs. The trace flags
  are not in the line, so records carry no flags. `InitLogger` keeps the
  hook (it cannot wrap the service's writer) until the app rework removes
  it. The contrib `otelzerolog` bridge has no release and is also a hook.

## 4. Changes

- `zerolog/setup.go`, `hooks.go`, `doc.go`; `otel.go`, `init.go` removed;
  `setup_test.go`. `app/app.go`: one-line adaptation to `WithOTLP(true)`.
- `scripts/replaces.sh`: graph-based version (as #36).
- `zerolog/.coverage-floor`: 42.5 → 97.2.

## 5. Verification

- Tests: JSON fields and trace ids, console locally (colored) and in
  clusters (no color), `LOG_FORMAT`, level precedence, OTLP records on by
  default with severity, time, trace and span ids and typed attributes
  (strings, int64, beyond-int64, float, bool, nested map and slice, null,
  error, caller), OTLP with console output, `WithOTLP(false)`, malformed and
  truncated lines dropped, severity mapping, `InitLogger` keeps the
  service's logger. `BenchmarkOTLPWriter`.
- `make tidy-check build vet test lint cover` — all modules pass.

## 7. Outcome

- [x] Implemented and verified locally.
- [ ] CI green; reviewed by Yumin.
