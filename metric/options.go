package metric

// Option configures a Server.
type Option func(*Config)

// WithPort sets the port to listen on.
func WithPort(port int) Option {
	return func(c *Config) {
		c.Port = port
	}
}
