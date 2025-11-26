package app

import (
	"github.com/ggsrc/gglib/grpc"
	"github.com/ggsrc/gglib/resource"
)

// Options holds the configuration for the App
type Options struct {
	ServerName      string
	ServerVersion   string
	Debug           bool
	Env             string
	OTELEnabled     bool
	OTELDSN         string
	OTELBatchSize   int
	GRPCServer      *grpc.Server
	ResourceManager resource.ResourceManager
}

// Option is a functional option for configuring the App
type Option func(*Options)

// WithServerName sets the server name
func WithServerName(name string) Option {
	return func(o *Options) {
		o.ServerName = name
	}
}

// WithDebug enables or disables debug mode
func WithDebug(debug bool) Option {
	return func(o *Options) {
		o.Debug = debug
	}
}

// WithEnv sets the environment (e.g., "development", "staging", "production")
func WithEnv(env string) Option {
	return func(o *Options) {
		o.Env = env
	}
}

// WithOTEL enables OpenTelemetry with the provided DSN
func WithOTEL(dsn string, batchSize int, serverVersion string) Option {
	return func(o *Options) {
		o.OTELEnabled = true
		o.OTELDSN = dsn
		o.OTELBatchSize = batchSize
		o.ServerVersion = serverVersion
	}
}

// WithGRPCServer sets the gRPC server
func WithGRPCServer(server *grpc.Server) Option {
	return func(o *Options) {
		o.GRPCServer = server
	}
}

// WithResourceManager sets the resource manager
func WithResourceManager(rm resource.ResourceManager) Option {
	return func(o *Options) {
		o.ResourceManager = rm
	}
}
