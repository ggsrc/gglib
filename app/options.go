package app

import (
	"time"

	"github.com/ggsrc/gglib/otel"
	ggzerolog "github.com/ggsrc/gglib/zerolog"
)

// Option configures New.
type Option func(*config)

type config struct {
	serverName, serverVersion string
	debug                     bool
	telemetry                 []otel.Option
	logging                   []ggzerolog.Option
	shutdownTimeout           time.Duration
}

func newConfig(opts []Option) *config {
	c := &config{shutdownTimeout: DefaultResourceShutDownTimeout}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// WithServerName sets the telemetry service name used when the deployment
// sets neither SERVICE_NAME nor OTEL_SERVICE_NAME.
func WithServerName(name string) Option {
	return func(c *config) { c.serverName = name }
}

// WithServerVersion sets the telemetry service version used when the
// deployment sets neither SERVICE_VERSION nor GIT_SHA and the binary carries
// no VCS revision.
func WithServerVersion(version string) Option {
	return func(c *config) { c.serverVersion = version }
}

// WithDebug sets the log level to debug when debug is true. Otherwise the
// level comes from LOG_LEVEL (default info); see gglib/zerolog.Setup.
func WithDebug(debug bool) Option {
	return func(c *config) { c.debug = debug }
}

// WithTelemetry passes options to gglib/otel.Setup, for example
// otel.WithRuntimeMetrics(true).
func WithTelemetry(opts ...otel.Option) Option {
	return func(c *config) { c.telemetry = append(c.telemetry, opts...) }
}

// WithLogging passes options to gglib/zerolog.Setup, for example
// zerolog.WithOTLP(false).
func WithLogging(opts ...ggzerolog.Option) Option {
	return func(c *config) { c.logging = append(c.logging, opts...) }
}

// WithShutdownTimeout bounds Stop: the gRPC drain and stopping the resources
// (default DefaultResourceShutDownTimeout, read when New is called).
func WithShutdownTimeout(d time.Duration) Option {
	return func(c *config) { c.shutdownTimeout = d }
}
