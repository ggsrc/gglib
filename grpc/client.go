package grpc

import (
	"context"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog"
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

	panicHandler      recoveryinterceptor.PanicHandler
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
	return &Client{
		conf: conf,
	}
}

// NewClient returns a Client configured from environment variables with the
// given prefix. It panics if they cannot be parsed.
func NewClient(envPrefix string) *Client {
	conf := &ClientConfig{}
	envconfig.MustProcess(envPrefix, conf)
	return &Client{
		conf: conf,
	}
}

// NewClientWithDefaultEnvPrefix returns a Client configured from GRPC_*
// environment variables.
func NewClientWithDefaultEnvPrefix() *Client {
	return NewClient("grpc")
}

// Dial creates a client connection to addr. The connection uses gzip,
// insecure transport credentials and the standard interceptors; opts are
// applied after the defaults.
func (c *Client) Dial(
	ctx context.Context,
	addr string,
	opts ...grpc.DialOption,
) (conn *grpc.ClientConn, err error) {
	logger := zerolog.DefaultContextLogger
	if logger == nil {
		logger = zerolog.Ctx(ctx)
	}

	loggableEvents := []logging.LoggableEvent{}
	if c.conf.Verbose {
		loggableEvents = append(loggableEvents, logging.StartCall, logging.FinishCall)
	}

	interceptors := []grpc.UnaryClientInterceptor{
		recoveryinterceptor.UnaryClientInterceptor(c.conf.panicHandler),
		errloginterceptor.UnaryClientInterceptor(),
		contextinterceptor.ContextUnaryClientInterceptor(),
		logging.UnaryClientInterceptor(
			InterceptorLogger(*logger),
			logging.WithLogOnEvents(loggableEvents...),
		),
		grpc_prometheus.UnaryClientInterceptor,
	}
	interceptors = append(interceptors, c.conf.unaryInterceptors...)

	defaultOpts := []grpc.DialOption{
		grpc.WithDefaultCallOptions(grpc.UseCompressor(gzip.Name)),
		grpc.WithUnaryInterceptor(chainUnaryClient(interceptors...)),

		grpc.WithStreamInterceptor(grpc_prometheus.StreamClientInterceptor),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	opts = append(defaultOpts, opts...)

	return grpc.NewClient(addr, opts...)
}

// chainUnaryClient is adapted from
// https://github.com/grpc-ecosystem/go-grpc-middleware/blob/master/chain.go.
func chainUnaryClient(interceptors ...grpc.UnaryClientInterceptor) grpc.UnaryClientInterceptor {
	n := len(interceptors)

	if n > 1 {
		lastI := n - 1
		return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			var (
				chainHandler grpc.UnaryInvoker
				curI         int
			)

			chainHandler = func(currentCtx context.Context, currentMethod string, currentReq, currentRepl any, currentConn *grpc.ClientConn, currentOpts ...grpc.CallOption) error {
				if curI == lastI {
					return invoker(
						currentCtx,
						currentMethod,
						currentReq,
						currentRepl,
						currentConn,
						currentOpts...)
				}
				curI++
				err := interceptors[curI](
					currentCtx,
					currentMethod,
					currentReq,
					currentRepl,
					currentConn,
					chainHandler,
					currentOpts...)
				curI--
				return err
			}

			return interceptors[0](ctx, method, req, reply, cc, chainHandler, opts...)
		}
	}

	if n == 1 {
		return interceptors[0]
	}

	// n == 0; Dummy interceptor maintained for backward compatibility to avoid returning nil.
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
