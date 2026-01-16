package grpc

import (
	"google.golang.org/grpc"

	recoveryinterceptor "github.com/ggsrc/gglib/interceptor/grpc/recovery"
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

func WithClientUnaryInterceptors(ii ...grpc.UnaryClientInterceptor) ClientOption {
	return func(c *ClientConfig) { c.unaryInterceptors = append(c.unaryInterceptors, ii...) }
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
	return func(c *ServerConfig) { c.unaryInterceptors = append(c.unaryInterceptors, ii...) }
}

func WithServerGRPCServerOptions(opts ...grpc.ServerOption) ServerOption {
	return func(c *ServerConfig) { c.gRPCServerOptions = append(c.gRPCServerOptions, opts...) }
}
