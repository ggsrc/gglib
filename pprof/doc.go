// Package pprof serves a small, guarded subset of the Go profiling endpoints
// over HTTP: goroutine and heap profiles, and a CPU profile with a bounded
// duration. It is off unless enabled, can require a token, and allows one CPU
// profile at a time.
package pprof
