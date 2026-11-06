# Changelog

All notable changes to `github.com/ggsrc/gglib/ratelimit` are listed here. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the
module is versioned on its own with tags `ratelimit/vX.Y.Z`.

## Unreleased

### Added

- First release: `Limiter` with `Allow` and `Reset`, `Decision`, and the
  `Limit` helpers `PerSecond`, `PerMinute` and `PerHour`, over redis_rate v10
  and go-redis v9.
