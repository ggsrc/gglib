package grpc

import (
	"context"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding/gzip"

	contextinterceptor "github.com/ggsrc/gglib/interceptor/grpc/context"
	errloginterceptor "github.com/ggsrc/gglib/interceptor/grpc/errlog"
	recoveryinterceptor "github.com/ggsrc/gglib/interceptor/grpc/recovery"
)

// ClientConfig configures a Client. Verbose logs the start and end of every
// call.
type ClientConfig struct {
	Verbose bool `default:"false"`

	unaryInterceptors []grpc.UnaryClientInterceptor
}

// Client creates gRPC client connections with the standard interceptors.
type Client struct {
	conf *ClientConfig
}

// NewClientWithOptions returns a Client configured by opts.
func NewClientWithOptions(opts ...ClientOption) *Client {
	conf := &ClientConfig{}
	for _, opt := range opts {
		opt(conf)
	}
	return &Client{conf: conf}
}

// NewClient returns a Client configured from environment variables with the
// given prefix. It panics if they cannot be parsed.
func NewClient(envPrefix string) *Client {
	conf := &ClientConfig{}
	envconfig.MustProcess(envPrefix, conf)
	return &Client{conf: conf}
}

// NewClientWithDefaultEnvPrefix returns a Client configured from GRPC_*
// environment variables.
func NewClientWithDefaultEnvPrefix() *Client {
	return NewClient("grpc")
}

// Dial creates a client connection to addr. The connection uses gzip,
// insecure transport credentials (in-cluster traffic) and the standard
// interceptors, outermost first: recovery → error log → request context →
// call log → metrics → WithClientUnaryInterceptors. opts are applied after
// the defaults; an interceptor passed in opts with grpc.WithUnaryInterceptor
// runs before the standard chain instead of replacing it.
func (c *Client) Dial(
	_ context.Context,
	addr string,
	opts ...grpc.DialOption,
) (*grpc.ClientConn, error) {
	logger := zerolog.DefaultContextLogger
	if logger == nil {
		logger = &log.Logger
	}

	var loggableEvents []logging.LoggableEvent
	if c.conf.Verbose {
		loggableEvents = append(loggableEvents, logging.StartCall, logging.FinishCall)
	}

	unary := []grpc.UnaryClientInterceptor{
		recoveryinterceptor.UnaryClientInterceptor(),
		errloginterceptor.UnaryClientInterceptor(),
		contextinterceptor.ContextUnaryClientInterceptor(),
		logging.UnaryClientInterceptor(
			interceptorLogger(*logger),
			logging.WithLogOnEvents(loggableEvents...),
		),
		grpc_prometheus.UnaryClientInterceptor,
	}
	unary = append(unary, c.conf.unaryInterceptors...)

	dialOpts := make([]grpc.DialOption, 0, 5+len(opts))
	dialOpts = append(dialOpts,
		grpc.WithDefaultCallOptions(grpc.UseCompressor(gzip.Name)),
		grpc.WithChainUnaryInterceptor(unary...),
		grpc.WithChainStreamInterceptor(grpc_prometheus.StreamClientInterceptor),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	dialOpts = append(dialOpts, opts...)
	return grpc.NewClient(addr, dialOpts...)
}
