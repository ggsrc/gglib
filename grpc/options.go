package grpc

import (
	"google.golang.org/grpc"
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

// WithServerDebug records request and response bodies in spans. Bodies can
// contain passwords and tokens; it is off unless set.
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

// WithServerUnaryInterceptors appends unary interceptors after the standard
// ones.
func WithServerUnaryInterceptors(ii ...grpc.UnaryServerInterceptor) ServerOption {
	return func(c *ServerConfig) { c.unaryInterceptors = append(c.unaryInterceptors, ii...) }
}

// WithServerGRPCServerOptions appends options passed to grpc.NewServer after
// the defaults. Interceptors added with grpc.ChainUnaryInterceptor or
// grpc.ChainStreamInterceptor run after the standard chain; one added with
// grpc.UnaryInterceptor or grpc.StreamInterceptor runs before it.
func WithServerGRPCServerOptions(opts ...grpc.ServerOption) ServerOption {
	return func(c *ServerConfig) { c.gRPCServerOptions = append(c.gRPCServerOptions, opts...) }
}
