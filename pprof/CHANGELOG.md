# Changelog

All notable changes to `github.com/ggsrc/gglib/pprof` are listed here. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the
module is versioned on its own with tags `pprof/vX.Y.Z`.

## Unreleased

### Added

- First release: a guarded profiling server with goroutine, heap and bounded
  CPU profiles, configured from `DEBUG_PPROF_*` and off by default, with an
  optional token and one CPU profile at a time.
