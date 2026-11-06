# Changelog

All notable changes to `github.com/ggsrc/gglib/resource/goroutine` are listed
here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
the module is versioned on its own with tags `resource/goroutine/vX.Y.Z`.

## Unreleased

First release as its own module. The package was part of
`github.com/ggsrc/gglib/resource` until `main` at `b474e43`
(`v0.0.0-20261013145132-b474e434e730`) and was left out of `resource/v0.1.0`.
The import path and the exported API are unchanged.

### Requirements

- Requires Go 1.27 (`go 1.27.0`).
- Requires `github.com/ggsrc/gglib/resource` v0.1.0 or later, so that the old
  copy of the package inside `resource` cannot be selected at the same time.

### Fixed

- **`Run` no longer races with `Stop`.** `Run` called during or after `Stop`
  could call `WaitGroup.Add` concurrently with `Wait`. After `Stop`, `Run`
  logs an error and does not start the function.
- **A second `Init` no longer orphans running goroutines.** It replaced the
  context, so `Stop` canceled only goroutines started after it and waited for
  the others until its deadline. `Init` now has an effect only once.
- **`Stop` before `Init` leaves the manager usable.**

### Changed

- A panic is recovered through `safego.Run`: the log line now includes the
  stack, with message `safego: panic recovered` and label `goroutine <name>`
  (it was `GoroutineManager [<name>] panicked`, without a stack).
- The error log line for a function that returns an error has a `goroutine`
  field with the name; the message is unchanged.
- `GoroutineManager` is asserted to implement `resource.Resource`.
