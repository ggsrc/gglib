package metric

type Option func(*Config)

func WithPort(port int) Option {
	return func(c *Config) {
		c.Port = port
	}
}
