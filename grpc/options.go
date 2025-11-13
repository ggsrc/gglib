package grpc

import "google.golang.org/grpc"

type ClientOption func(*ClientConfig)
type ServerOption func(*ServerConfig)

func WithClientRavenDSN(ravenDSN string) ClientOption {
	return func(c *ClientConfig) {
		c.RavenDSN = ravenDSN
	}
}

func WithClientVerbose(verbose bool) ClientOption {
	return func(c *ClientConfig) {
		c.Verbose = verbose
	}
}

func WithServerRavenDSN(ravenDSN string) ServerOption {
	return func(c *ServerConfig) {
		c.RavenDSN = ravenDSN
	}
}

func WithServerVerbose(verbose bool) ServerOption {
	return func(c *ServerConfig) {
		c.Verbose = verbose
	}
}

func WithServerDebug(debug bool) ServerOption {
	return func(c *ServerConfig) {
		c.Debug = debug
	}
}

func WithServerPort(port int) ServerOption {
	return func(c *ServerConfig) {
		c.Port = port
	}
}

func WithServerUnaryInterceptors(ii ...grpc.UnaryServerInterceptor) ServerOption {
	return func(c *ServerConfig) { c.UnaryInterceptors = append(c.UnaryInterceptors, ii...) }
}

func WithServerGRPCServerOptions(opts ...grpc.ServerOption) ServerOption {
	return func(c *ServerConfig) { c.GRPCServerOptions = append(c.GRPCServerOptions, opts...) }
}
