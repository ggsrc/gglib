# Changelog

All notable changes to `github.com/ggsrc/gglib/env` are listed here. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the
module is versioned on its own with tags `env/vX.Y.Z`.

## Unreleased

## v0.1.0 - 2026-11-03

First release of the module on its own.

Changes since `main` at `b474e43` (`v0.0.0-20261013145132-b474e434e730`),
the last commit before the first release work.

### Requirements

- Requires Go 1.27 (`go 1.27.0`).
- `go.mod` is complete on its own; sibling gglib modules are required at
  matching versions.

### Changed

- `Env` trims and lower-cases `ENV` (falling back to
  `OTEL_DEPLOYMENT_ENVIRONMENT`) and returns canonical names: `prd`
  (also `prod`, `production`), `stg` (also `staging`), `beta`, `test`, and
  `local` for an empty value.
- **`IsProduction` is true for any value it does not recognize.** Before,
  only an exact `prd` counted, so `PRD`, `prod` or a typo turned on behavior
  meant for other environments, such as recording request bodies in spans.
- All `Is*` functions derive from `Env`, so they agree with it. Before,
  `ENV=""` with `OTEL_DEPLOYMENT_ENVIRONMENT=prd` gave `Env()=="prd"` but
  `IsLocal()==true`.
- `ServiceName` and `ServiceVersion` read the environment on every call
  instead of once at program start, so `os.Setenv` in `main` is seen.
  `ServiceVersion` falls back to the binary's VCS revision.
- Only `SERVICE_NAME`/`OTEL_SERVICE_NAME` and `SERVICE_VERSION`/`GIT_SHA` are
  read; the `service_name`, `service-name`, `service_version`,
  `service-version` and `OTEL_SERVICE_VERSION` spellings are gone.
- No `init` functions.
- Every exported identifier is documented.

### Added

- Constants `Production`, `Staging`, `Beta`, `Test`, `Local`.

### Removed

- `SetServiceName`, `SetServiceVersion`, `HostName`, `SetHostName`,
  `UnknownHostName`, `BuildTime`, `SetBuildTime`, `UnknownBuildTime`: no
  service or gglib module used them, and the setters raced with the getters.
