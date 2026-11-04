# env

```sh
go get github.com/ggsrc/gglib/env
```

`env` answers three questions: which environment the process runs in
(production, staging, …), what the service is called, and which version it
is. Other gglib modules use it to choose behaviour, for example the log format
([zerolog](../zerolog)) or the telemetry resource attributes ([otel](../otel)).

Every call reads the environment variables again; nothing is cached, so a value
set with `os.Setenv` early in `main` (or in a test) is seen.

## API

| Name | Returns |
|---|---|
| `Env() string` | The environment. Reads `ENV`, or `OTEL_DEPLOYMENT_ENVIRONMENT` when `ENV` is empty; trims spaces and lowercases; maps known spellings to a canonical name (table below); returns any other value as is (lowercased). |
| `IsProduction() bool` | Whether this is production. **An unknown environment name counts as production.** |
| `IsStaging()`, `IsBeta()`, `IsUnitTest()`, `IsLocal()` | Whether `Env()` is `stg`, `beta`, `test` or `local`. With both variables empty, the environment is `local`. |
| `ServiceName() string` | `SERVICE_NAME`, else `OTEL_SERVICE_NAME`, else `UnknownServiceName` (`"UnknownService"`). |
| `ServiceVersion() string` | `SERVICE_VERSION`, else `GIT_SHA`, else the VCS revision recorded in the binary, else `""`. |
| `Production`, `Staging`, `Beta`, `Test`, `Local` | The canonical names `"prd"`, `"stg"`, `"beta"`, `"test"`, `"local"`. |

## Environment names

| `ENV` (any case) | `Env()` | `IsProduction()` |
|---|---|---|
| `prd`, `prod`, `production` | `prd` | true |
| `stg`, `staging` | `stg` | false |
| `beta` | `beta` | false |
| `test` | `test` | false |
| empty, `local` | `local` | false |
| anything else, e.g. `dev` | unchanged (lowercased) | **true** |

Why unknown means production: some behaviour should only be on outside
production, such as recording request bodies. Treating a misspelled name as
non-production could switch it on in production; treating it as production
only switches it off somewhere it was wanted. Set `ENV` explicitly in every
deployment.

## Example

```go
if !env.IsProduction() {
	opts = append(opts, grpc.WithServerDebug(true))
}
log.Info().Str("env", env.Env()).Str("version", env.ServiceVersion()).Msg("starting")
```

## See also

- [otel](../otel) — `service.name` in telemetry prefers `OTEL_SERVICE_NAME`,
  while `ServiceName()` prefers `SERVICE_NAME`. Set one of them, or the same
  value in both.
- [docs/configuration.md](../docs/configuration.md) — every variable gglib reads.
