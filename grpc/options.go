package grpc

import (
	recoveryinterceptor "github.com/ggsrc/gglib/interceptor/grpc/recovery"
	"google.golang.org/grpc"
)

type (
	ClientOption func(*ClientConfig)
	ServerOption func(*ServerConfig)
)

func WithClientVerbose(verbose bool) ClientOption {
	return func(c *ClientConfig) {
		c.Verbose = verbose
	}
}

func WithClientPanicHandler(panicHandler recoveryinterceptor.PanicHandler) ClientOption {
	return func(c *ClientConfig) {
		c.panicHandler = panicHandler
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

func WithServerPanicHandler(panicHandler recoveryinterceptor.PanicHandler) ServerOption {
	return func(c *ServerConfig) {
		c.panicHandler = panicHandler
	}
}

func WithServerUnaryInterceptors(ii ...grpc.UnaryServerInterceptor) ServerOption {
	return func(c *ServerConfig) { c.UnaryInterceptors = append(c.UnaryInterceptors, ii...) }
}

func WithServerGRPCServerOptions(opts ...grpc.ServerOption) ServerOption {
	return func(c *ServerConfig) { c.GRPCServerOptions = append(c.GRPCServerOptions, opts...) }
}
