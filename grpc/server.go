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

	contextinterceptor "github.com/ggsrc/gglib/interceptor/grpc/context"
	errloginterceptor "github.com/ggsrc/gglib/interceptor/grpc/errlog"
	loginterceptor "github.com/ggsrc/gglib/interceptor/grpc/log"
	recoveryinterceptor "github.com/ggsrc/gglib/interceptor/grpc/recovery"
)

// DefaultServerPort is the port Start listens on unless configured otherwise.
const DefaultServerPort = 9090

// Server is a gRPC server with the standard interceptors.
type Server struct {
	conf   *ServerConfig
	server *grpc.Server
}

// ServerConfig configures a Server. Port defaults to DefaultServerPort.
// Debug records request and response bodies in spans (they can contain
// secrets); Verbose logs the start and end of every call.
type ServerConfig struct {
	Debug   bool `default:"false"`
	Port    int  `default:"9090"`
	Verbose bool `default:"false"`

	unaryInterceptors []grpc.UnaryServerInterceptor
	gRPCServerOptions []grpc.ServerOption
}

// NewServerWithOptions returns a Server configured by opts.
func NewServerWithOptions(opts ...ServerOption) *Server {
	conf := &ServerConfig{}
	for _, opt := range opts {
		opt(conf)
	}
	if conf.Port == 0 {
		conf.Port = DefaultServerPort
	}
	return newServer(conf)
}

// NewServerWithDefaultEnvPrefix returns a Server configured from GRPC_*
// environment variables. It panics if they cannot be parsed.
func NewServerWithDefaultEnvPrefix() *Server {
	var conf ServerConfig
	envconfig.MustProcess("grpc", &conf)
	return newServer(&conf)
}

// newServer builds the server. The standard chain, outermost first:
//
//	unary:  metrics → recovery → request context → error log → call log
//	        [→ body capture when Debug] → WithServerUnaryInterceptors
//	stream: metrics → recovery → request context → error log
//
// Metrics are outermost so that a recovered panic is counted as Internal.
// Interceptors passed through WithServerGRPCServerOptions (for example
// grpc.ChainStreamInterceptor) are appended after the standard ones.
func newServer(conf *ServerConfig) *Server {
	logger := zerolog.DefaultContextLogger
	if logger == nil {
		logger = &log.Logger
	}

	var loggableEvents []logging.LoggableEvent
	if conf.Verbose {
		loggableEvents = append(loggableEvents, logging.StartCall, logging.FinishCall)
	}

	unary := []grpc.UnaryServerInterceptor{
		grpc_prometheus.UnaryServerInterceptor,
		recoveryinterceptor.UnaryServerInterceptor(),
		contextinterceptor.ContextUnaryServerInterceptor(),
		errloginterceptor.UnaryServerInterceptor(),
		logging.UnaryServerInterceptor(
			interceptorLogger(*logger),
			logging.WithLogOnEvents(loggableEvents...),
		),
	}
	if conf.Debug {
		unary = append(unary, loginterceptor.LogUnaryServerInterceptor())
	}
	unary = append(unary, conf.unaryInterceptors...)

	stream := []grpc.StreamServerInterceptor{
		grpc_prometheus.StreamServerInterceptor,
		recoveryinterceptor.StreamServerInterceptor(),
		contextinterceptor.ContextStreamServerInterceptor(),
		errloginterceptor.StreamServerInterceptor(),
	}

	opts := make([]grpc.ServerOption, 0, 3+len(conf.gRPCServerOptions))
	opts = append(opts,
		grpc.ChainUnaryInterceptor(unary...),
		grpc.ChainStreamInterceptor(stream...),
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	opts = append(opts, conf.gRPCServerOptions...)

	s := &Server{conf: conf, server: grpc.NewServer(opts...)}
	grpc_prometheus.Register(s.server)
	// Record latency histograms (grpc_server_handling_seconds); dashboards
	// read them.
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
	return err
}
