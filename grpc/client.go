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
	recoveryinterceptor "github.com/ggsrc/gglib/interceptor/grpc/recovery"
)

type ClientConfig struct {
	RavenDSN string `required:"true"`
	Verbose  bool   `default:"false"`
}

type Client struct {
	serverName string
	clientName string
	conf       *ClientConfig
}

func NewClient(serverName, clientName string, envPrefix string) *Client {
	conf := &ClientConfig{}
	envconfig.MustProcess(envPrefix, conf)
	return &Client{
		serverName: serverName,
		clientName: clientName,
		conf:       conf,
	}
}

func NewClientWithDefaultEnvPrefix(serverName, clientName string) *Client {
	return NewClient(serverName, clientName, "grpc")
}

func (c *Client) Dial(ctx context.Context, addr string, opts ...grpc.DialOption) (conn *grpc.ClientConn, err error) {
	logger := zerolog.DefaultContextLogger
	if logger == nil {
		logger = zerolog.Ctx(ctx)
	}

	loggableEvents := []logging.LoggableEvent{}
	if c.conf.Verbose {
		loggableEvents = append(loggableEvents, logging.StartCall)
		loggableEvents = append(loggableEvents, logging.FinishCall)
	}

	defaultOpts := []grpc.DialOption{
		grpc.WithDefaultCallOptions(grpc.UseCompressor(gzip.Name)),
		grpc.WithUnaryInterceptor(chainUnaryClient(
			recoveryinterceptor.SentryUnaryClientInterceptor(c.conf.RavenDSN),
			contextinterceptor.ContextUnaryClientInterceptor(),
			logging.UnaryClientInterceptor(InterceptorLogger(*logger), logging.WithLogOnEvents(loggableEvents...)),
			grpc_prometheus.UnaryClientInterceptor,
		)),

		grpc.WithStreamInterceptor(grpc_prometheus.StreamClientInterceptor),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	opts = append(defaultOpts, opts...)

	return grpc.NewClient(addr, opts...)
}

// From https://github.com/grpc-ecosystem/go-grpc-middleware/blob/master/chain.go
func chainUnaryClient(interceptors ...grpc.UnaryClientInterceptor) grpc.UnaryClientInterceptor {
	n := len(interceptors)

	if n > 1 {
		lastI := n - 1
		return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			var (
				chainHandler grpc.UnaryInvoker
				curI         int
			)

			chainHandler = func(currentCtx context.Context, currentMethod string, currentReq, currentRepl interface{}, currentConn *grpc.ClientConn, currentOpts ...grpc.CallOption) error {
				if curI == lastI {
					return invoker(currentCtx, currentMethod, currentReq, currentRepl, currentConn, currentOpts...)
				}
				curI++
				err := interceptors[curI](currentCtx, currentMethod, currentReq, currentRepl, currentConn, chainHandler, currentOpts...)
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
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
