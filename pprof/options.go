package pprof

// Option configures a Server created with NewWithOptions.
type Option func(*Config)

// WithEnabled turns the server on or off.
func WithEnabled(enabled bool) Option { return func(c *Config) { c.Enabled = enabled } }

// WithAddr sets the listen address.
func WithAddr(addr string) Option { return func(c *Config) { c.Addr = addr } }

// WithToken requires the token on every request.
func WithToken(token string) Option { return func(c *Config) { c.Token = token } }

// WithCPUProfileSeconds sets the default and the maximum CPU profile length.
func WithCPUProfileSeconds(defaultSeconds, maxSeconds int) Option {
	return func(c *Config) {
		c.CPUProfileSeconds = defaultSeconds
		c.CPUProfileMaxSeconds = maxSeconds
	}
}
