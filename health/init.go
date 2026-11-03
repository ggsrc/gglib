package health

// InitHealthCheck returns a Server with the default environment prefix and
// its own router.
func InitHealthCheck(hc ...HealthCheckable) *Server {
	checker := NewWithDefaultEnvPrefix(nil, hc...)
	return checker
}
