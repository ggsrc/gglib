package health

func InitHealthCheck(hc ...HealthCheckable) *Server {
	checker := NewWithDefaultEnvPrefix(nil, hc...)
	return checker
}
