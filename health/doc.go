// Package health serves Kubernetes liveness and readiness probes at
// /health/alive and /health/ready, on port 8080 unless configured otherwise.
// A background loop runs the registered checks every ProbeInterval and counts
// consecutive failures.
package health
