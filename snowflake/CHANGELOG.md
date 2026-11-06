# Changelog

All notable changes to `github.com/ggsrc/gglib/snowflake` are listed here. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the
module is versioned on its own with tags `snowflake/vX.Y.Z`.

## Unreleased

## v0.1.0 - 2026-11-06

### Added

- First release: `NewNode` takes a node number from a Redis counter
  (`<appName>:snowflake:nodeID`, modulo 1024); `Init`, `Generate` and
  `Default` manage a default node. Errors are returned, never panicked, except
  `Generate` before `Init`.
