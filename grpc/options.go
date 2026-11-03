package grpc

import (
	"google.golang.org/grpc"

	recoveryinterceptor "github.com/ggsrc/gglib/interceptor/grpc/recovery"
)

type (
	// ClientOption configures a Client.
	ClientOption func(*ClientConfig)
	// ServerOption configures a Server.
	ServerOption func(*ServerConfig)
)

// WithClientVerbose logs the start and end of every client call.
func WithClientVerbose(verbose bool) ClientOption {
	return func(c *ClientConfig) {
		c.Verbose = verbose
	}
}

// WithClientPanicHandler sets the handler called when a client interceptor
// recovers from a panic.
func WithClientPanicHandler(panicHandler recoveryinterceptor.PanicHandler) ClientOption {
	return func(c *ClientConfig) {
		c.panicHandler = panicHandler
	}
}

// WithClientUnaryInterceptors appends unary interceptors after the standard
// ones.
func WithClientUnaryInterceptors(ii ...grpc.UnaryClientInterceptor) ClientOption {
	return func(c *ClientConfig) { c.unaryInterceptors = append(c.unaryInterceptors, ii...) }
}

// WithServerVerbose logs the start and end of every call.
func WithServerVerbose(verbose bool) ServerOption {
	return func(c *ServerConfig) {
		c.Verbose = verbose
	}
}

// WithServerDebug records request and response bodies in spans, as is always
// done outside production.
func WithServerDebug(debug bool) ServerOption {
	return func(c *ServerConfig) {
		c.Debug = debug
	}
}

// WithServerPort sets the port Start listens on.
func WithServerPort(port int) ServerOption {
	return func(c *ServerConfig) {
		c.Port = port
	}
}

// WithServerPanicHandler sets the handler called when the server recovers from
// a panic in a unary call.
func WithServerPanicHandler(panicHandler recoveryinterceptor.PanicHandler) ServerOption {
	return func(c *ServerConfig) {
		c.panicHandler = panicHandler
	}
}

// WithServerUnaryInterceptors appends unary interceptors after the standard
// ones.
func WithServerUnaryInterceptors(ii ...grpc.UnaryServerInterceptor) ServerOption {
	return func(c *ServerConfig) { c.unaryInterceptors = append(c.unaryInterceptors, ii...) }
}

// WithServerGRPCServerOptions appends options passed to grpc.NewServer after
// the defaults.
func WithServerGRPCServerOptions(opts ...grpc.ServerOption) ServerOption {
	return func(c *ServerConfig) { c.gRPCServerOptions = append(c.gRPCServerOptions, opts...) }
}
