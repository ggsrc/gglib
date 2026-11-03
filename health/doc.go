// Package health serves Kubernetes probes at /health/ready and
// /health/alive, on port 8080 unless configured otherwise.
//
// Readiness follows the registered checks (database, Redis, …): while they
// fail, the pod gets no traffic, and it gets traffic again as soon as they
// pass. Liveness does not restart the pod because a dependency is down; it
// fails only when checks have failed for longer than StuckAfter (default
// three minutes: the dependency is back but this instance did not recover)
// or when the check loop itself stops running.
package health
