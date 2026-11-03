package grpc

import (
	"context"
	"fmt"
	"net"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"

	"github.com/ggsrc/gglib/env"
	contextinterceptor "github.com/ggsrc/gglib/interceptor/grpc/context"
	errloginterceptor "github.com/ggsrc/gglib/interceptor/grpc/errlog"
	loginterceptor "github.com/ggsrc/gglib/interceptor/grpc/log"
	recoveryinterceptor "github.com/ggsrc/gglib/interceptor/grpc/recovery"
)

// Server is a gRPC server with the standard interceptors.
type Server struct {
	conf   *ServerConfig
	server *grpc.Server
}

// ServerConfig configures a Server. Port defaults to 9090. Debug records
// request and response bodies in spans; Verbose logs the start and end of
// every call.
type ServerConfig struct {
	Debug   bool `default:"false"`
	Port    int  `default:"9090"`
	Verbose bool `default:"false"`

	panicHandler      recoveryinterceptor.PanicHandler
	unaryInterceptors []grpc.UnaryServerInterceptor
	gRPCServerOptions []grpc.ServerOption
}

// NewServerWithOptions returns a Server configured by opts.
func NewServerWithOptions(opts ...ServerOption) *Server {
	conf := &ServerConfig{}
	for _, opt := range opts {
		opt(conf)
	}
	// default port is 9090
	if conf.Port == 0 {
		conf.Port = 9090
	}
	return newServer(conf)
}

// NewServerWithDefaultEnvPrefix returns a Server configured from GRPC_*
// environment variables. It panics if they cannot be parsed.
func NewServerWithDefaultEnvPrefix() *Server {
	var grpcConfig ServerConfig
	envconfig.MustProcess("grpc", &grpcConfig)
	return newServer(&grpcConfig)
}

func newServer(conf *ServerConfig) *Server {
	s := &Server{
		conf: conf,
	}

	logger := zerolog.DefaultContextLogger
	if logger == nil {
		logger = &log.Logger
	}

	loggableEvents := []logging.LoggableEvent{}
	if conf.Verbose {
		loggableEvents = append(loggableEvents, logging.StartCall, logging.FinishCall)
	}

	interceptors := []grpc.UnaryServerInterceptor{
		contextinterceptor.ContextUnaryServerInterceptor(),
		recoveryinterceptor.UnaryServerInterceptor(conf.panicHandler),
		errloginterceptor.UnaryServerInterceptor(),
		logging.UnaryServerInterceptor(
			InterceptorLogger(*logger),
			logging.WithLogOnEvents(loggableEvents...),
		),
		grpc_prometheus.UnaryServerInterceptor,
	}

	if conf.Debug || !env.IsProduction() {
		interceptors = append(interceptors, loginterceptor.LogUnaryServerInterceptor())
	}

	interceptors = append(interceptors, conf.unaryInterceptors...)

	defaultOpts := []grpc.ServerOption{
		grpc.UnaryInterceptor(chainUnaryServer(interceptors...)),
		grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	}
	defaultOpts = append(defaultOpts, conf.gRPCServerOptions...)
	s.server = grpc.NewServer(defaultOpts...)
	grpc_prometheus.Register(s.server)
	// Prometheus histograms are a great way to measure latency distributions of your RPCs. However,
	// since it is bad practice to have metrics of high cardinality the latency monitoring metrics
	// are disabled by default.
	grpc_prometheus.EnableHandlingTimeHistogram()
	return s
}

// Server returns the underlying *grpc.Server, for registering services.
func (s *Server) Server() *grpc.Server {
	return s.server
}

// Start listens on the configured TCP port and serves until the server is
// stopped. It returns a non-nil error unless the server was stopped.
func (s *Server) Start() error {
	var lc net.ListenConfig
	lis, err := lc.Listen(context.Background(), "tcp", fmt.Sprintf(":%d", s.conf.Port))
	if err != nil {
		return err
	}
	return s.Serve(lis)
}

// Serve accepts incoming connections on the listener lis, creating a new
// ServerTransport and service goroutine for each.
// Serve will return a non-nil error unless Stop or GracefulStop is called.
func (s *Server) Serve(lis net.Listener) error {
	return s.server.Serve(lis)
}

// Shutdown stops the server gracefully. It stops the server from
// accepting new connections and RPCs and blocks until all the pending RPCs are
// finished or the context deadline is reached.
func (s *Server) Shutdown(ctx context.Context) (err error) {
	ch := make(chan struct{})
	go func() {
		s.server.GracefulStop()
		close(ch)
	}()
	select {
	case <-ctx.Done():
		s.server.Stop()
		err = ctx.Err()
	case <-ch:
	}
	return
}

// chainUnaryServer is adapted from
// https://github.com/grpc-ecosystem/go-grpc-middleware/blob/master/chain.go.
func chainUnaryServer(interceptors ...grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	n := len(interceptors)

	if n > 1 {
		lastI := n - 1
		return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			var (
				chainHandler grpc.UnaryHandler
				curI         int
			)

			chainHandler = func(currentCtx context.Context, currentReq any) (any, error) {
				if curI == lastI {
					return handler(currentCtx, currentReq)
				}
				curI++
				resp, err := interceptors[curI](currentCtx, currentReq, info, chainHandler)
				curI--
				return resp, err
			}

			return interceptors[0](ctx, req, info, chainHandler)
		}
	}

	if n == 1 {
		return interceptors[0]
	}

	// n == 0; Dummy interceptor maintained for backward compatibility to avoid returning nil.
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		return handler(ctx, req)
	}
}
