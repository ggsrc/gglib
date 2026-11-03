// Package zerolog sets up logging for a process with one call to Setup: the
// global zerolog logger and zerolog.DefaultContextLogger, one level knob
// (LOG_LEVEL), JSON in clusters and readable output locally, trace_id and
// span_id on every event logged with a span in its context, and optionally
// the same records sent to OpenTelemetry (see gglib/otel).
//
// Use gglib/zerolog/log in code: log.Ctx(ctx) returns the request's logger.
package zerolog
