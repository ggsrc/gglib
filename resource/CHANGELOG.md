# Changelog

All notable changes to `github.com/ggsrc/gglib/resource` are listed here. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the
module is versioned on its own with tags `resource/vX.Y.Z`.

## Unreleased

## v0.1.0 - 2026-11-03

First release of the module on its own.

Changes since `main` at `b474e43` (`v0.0.0-20261013145132-b474e434e730`),
the last commit before the first release work.

### Requirements

- Requires Go 1.27 (`go 1.27.0`).
- `go.mod` is complete on its own; sibling gglib modules are required at
  matching versions.

### Fixed

- **`Stop` stops every resource even when some fail** and returns all errors
  joined. Before, the first error skipped the rest (a failed Redis close
  leaked the database pool and the workers).
- `Init` can be retried: it continues with the resource that failed instead
  of initializing the earlier ones again (which failed with "already
  initialized"). `app.Init` relies on this to retry until the dependencies
  are reachable.
- `Start` passes the caller's context to each resource. Before, it passed an
  errgroup context that was canceled as soon as `Start` returned, so any
  background work tied to it stopped at once.
- A failed `Start` stops every resource and returns both the start error
  and any stop errors.
- Errors name the resource (`resource cache: stop: …`); `OK` reports every
  failing resource, not just the first.
- `Stop` only acts on the first call (`app` and services call it twice on
  the failure path).
- No more `fmt.Printf` to stdout on start failures.
- `AddResource` is safe for concurrent use; `NewResourceManager` copies the
  slice it is given.

### Changed

- **`Stop` runs in reverse registration order.** Register a dependency (a
  database pool) before the resources that use it; they then stop before it.
  Before, `Stop` ran in registration order, so services that wanted users
  stopped first registered them first. Reverse that registration order when
  upgrading.
- `Start` runs the resources one by one, in order, instead of in parallel.

### Removed

- `HealthStatus` (unused).
- `resource/goroutine` (no users).

### Changed

- Errors are wrapped with `fmt.Errorf` instead of `github.com/pkg/errors`;
  the error texts are unchanged.
- Every exported identifier is documented.
