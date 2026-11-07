# Changelog

All notable changes to `github.com/ggsrc/gglib/safego` are listed here. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the
module is versioned on its own with tags `safego/vX.Y.Z`.

## Unreleased

## v0.1.0 - 2026-11-07

First release.

### Added

- `Run`, `Go` and `Async`: run a function, in the current goroutine or a new
  one, with a panic recovery boundary. A panic is logged once with its value,
  stack and label through the context's logger and becomes `ErrPanic`.
- `Group` (`NewGroup`, `WithContext`): an `errgroup.Group` whose goroutines
  recover panics, with `Go`, `TryGo`, `SetLimit`, `Wait`, and `GoInto` for
  branches whose error the caller handles instead of failing the group.
