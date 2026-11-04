package grpc

import (
	"bytes"
	"context"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	contextinterceptor "github.com/ggsrc/gglib/interceptor/grpc/context"
)

// healthServer behaves according to the requested service name.
type healthServer struct {
	healthpb.UnimplementedHealthServer
}

func (h *healthServer) Check(_ context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	switch req.GetService() {
	case "panic":
		panic("unary boom")
	case "notfound":
		return nil, status.Error(codes.NotFound, "no such service")
	}
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}

func (h *healthServer) Watch(req *healthpb.HealthCheckRequest, stream grpc.ServerStreamingServer[healthpb.HealthCheckResponse]) error {
	if req.GetService() == "panic" {
		panic("stream boom")
	}
	return stream.Send(&healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING})
}

// start runs s with a healthServer on a free port and returns a client.
func start(t *testing.T, s *Server, dialOpts ...grpc.DialOption) healthpb.HealthClient {
	t.Helper()
	healthpb.RegisterHealthServer(s.Server(), &healthServer{})
	var lc net.ListenConfig
	lis, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = s.Serve(lis) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
	})

	conn, err := NewClientWithOptions().Dial(t.Context(), lis.Addr().String(), dialOpts...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return healthpb.NewHealthClient(conn)
}

func handledCount(t *testing.T, method, code string) float64 {
	t.Helper()
	mfs, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)
	for _, mf := range mfs {
		if mf.GetName() != "grpc_server_handled_total" {
			continue
		}
		for _, m := range mf.GetMetric() {
			labels := map[string]string{}
			for _, l := range m.GetLabel() {
				labels[l.GetName()] = l.GetValue()
			}
			if labels["grpc_method"] == method && labels["grpc_code"] == code {
				return m.GetCounter().GetValue()
			}
		}
	}
	return 0
}

// A panic used to come back as Unknown and was missing from
// grpc_server_handled_total.
func TestUnaryPanicIsInternalAndCounted(t *testing.T) {
	client := start(t, NewServerWithOptions())
	before := handledCount(t, "Check", "Internal")

	_, err := client.Check(t.Context(), &healthpb.HealthCheckRequest{Service: "panic"})
	assert.Equal(t, codes.Internal, status.Code(err))
	assert.InDelta(t, before+1, handledCount(t, "Check", "Internal"), 0.001)

	resp, err := client.Check(t.Context(), &healthpb.HealthCheckRequest{})
	require.NoError(t, err, "the server keeps serving")
	assert.Equal(t, healthpb.HealthCheckResponse_SERVING, resp.GetStatus())
}

// A panic in a streaming handler used to crash the process.
func TestStreamPanicIsRecovered(t *testing.T) {
	client := start(t, NewServerWithOptions())

	stream, err := client.Watch(t.Context(), &healthpb.HealthCheckRequest{Service: "panic"})
	require.NoError(t, err)
	_, err = stream.Recv()
	assert.Equal(t, codes.Internal, status.Code(err))

	stream, err = client.Watch(t.Context(), &healthpb.HealthCheckRequest{})
	require.NoError(t, err)
	resp, err := stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, healthpb.HealthCheckResponse_SERVING, resp.GetStatus())
}

// grpc.StreamInterceptor in the server options used to panic ("already
// set"); stream interceptors added either way now run.
func TestServerOptionInterceptorsCompose(t *testing.T) {
	var chained, single atomic.Int32
	count := func(n *atomic.Int32) grpc.StreamServerInterceptor {
		return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, h grpc.StreamHandler) error {
			n.Add(1)
			return h(srv, ss)
		}
	}
	var unary atomic.Int32
	s := NewServerWithOptions(
		WithServerGRPCServerOptions(
			grpc.StreamInterceptor(count(&single)),
			grpc.ChainStreamInterceptor(count(&chained)),
		),
		WithServerUnaryInterceptors(func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
			unary.Add(1)
			return h(ctx, req)
		}),
	)
	client := start(t, s)

	stream, err := client.Watch(t.Context(), &healthpb.HealthCheckRequest{})
	require.NoError(t, err)
	_, err = stream.Recv()
	require.NoError(t, err)
	_, err = client.Check(t.Context(), &healthpb.HealthCheckRequest{})
	require.NoError(t, err)

	assert.Equal(t, int32(1), single.Load())
	assert.Equal(t, int32(1), chained.Load())
	assert.Equal(t, int32(1), unary.Load())
}

// A grpc.WithUnaryInterceptor dial option used to replace the whole
// standard client chain.
func TestClientDialInterceptorDoesNotReplaceChain(t *testing.T) {
	var userCalls atomic.Int32
	userInterceptor := grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		userCalls.Add(1)
		return invoker(ctx, method, req, reply, cc, opts...)
	})
	client := start(t, NewServerWithOptions(), userInterceptor)
	before := clientHandled(t, "Check")

	_, err := client.Check(t.Context(), &healthpb.HealthCheckRequest{})
	require.NoError(t, err)
	assert.Equal(t, int32(1), userCalls.Load())
	assert.InDelta(t, before+1, clientHandled(t, "Check"), 0.001, "the standard metrics interceptor still ran")
}

func clientHandled(t *testing.T, method string) float64 {
	t.Helper()
	mfs, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)
	var total float64
	for _, mf := range mfs {
		if mf.GetName() != "grpc_client_handled_total" {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "grpc_method" && l.GetValue() == method {
					total += m.GetCounter().GetValue()
				}
			}
		}
	}
	return total
}

func recordSpans(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec)))
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	return rec
}

func serverSpanAttrs(rec *tracetest.SpanRecorder) map[string]string {
	out := map[string]string{}
	for _, s := range rec.Ended() {
		for _, kv := range s.Attributes() {
			if kv.Value.Type() == attribute.STRING {
				out[string(kv.Key)] = kv.Value.AsString()
			}
		}
	}
	return out
}

// Outside production, request and response bodies used to be recorded in
// spans by default. Now only WithServerDebug(true) does it.
func TestBodiesRecordedOnlyWithDebug(t *testing.T) {
	t.Setenv("ENV", "stg")

	rec := recordSpans(t)
	client := start(t, NewServerWithOptions())
	_, err := client.Check(t.Context(), &healthpb.HealthCheckRequest{Service: "secret"})
	require.NoError(t, err)
	assert.NotContains(t, serverSpanAttrs(rec), "grpc.request", "off by default, even in stg")

	rec = recordSpans(t)
	client = start(t, NewServerWithOptions(WithServerDebug(true)))
	_, err = client.Check(t.Context(), &healthpb.HealthCheckRequest{Service: "secret"})
	require.NoError(t, err)
	attrs := serverSpanAttrs(rec)
	assert.Contains(t, attrs["grpc.request"], "secret")
	assert.Contains(t, attrs["grpc.response"], "SERVING")
}

func TestVerboseLogsCallsWithContextLogger(t *testing.T) {
	var buf bytes.Buffer
	prev := zerolog.DefaultContextLogger
	l := zerolog.New(&buf)
	zerolog.DefaultContextLogger = &l
	t.Cleanup(func() { zerolog.DefaultContextLogger = prev })

	client := start(t, NewServerWithOptions(WithServerVerbose(true)))
	_, err := client.Check(t.Context(), &healthpb.HealthCheckRequest{Service: "notfound"})
	require.Equal(t, codes.NotFound, status.Code(err))

	out := buf.String()
	assert.Contains(t, out, "started call")
	assert.Contains(t, out, "finished call")
	assert.Contains(t, out, "grpc server error", "errlog")
}

func TestInterceptorLoggerLevels(t *testing.T) {
	var buf bytes.Buffer
	logger := interceptorLogger(zerolog.New(&buf))
	ctx := context.Background()
	for _, lvl := range []int{-4, 0, 4, 8, 99} {
		logger.Log(ctx, loggingLevel(lvl), "m", "k", "v")
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 5, "an unknown level logs at error instead of panicking")
	assert.Contains(t, lines[0], `"level":"debug"`)
	assert.Contains(t, lines[1], `"level":"info"`)
	assert.Contains(t, lines[2], `"level":"warn"`)
	assert.Contains(t, lines[3], `"level":"error"`)
	assert.Contains(t, lines[4], `"level":"error"`)
	assert.Contains(t, lines[0], `"k":"v"`)

	// A logger in the context wins over the fallback.
	var ctxBuf bytes.Buffer
	ctx = zerolog.New(&ctxBuf).WithContext(ctx)
	logger.Log(ctx, loggingLevel(0), "from ctx")
	assert.Contains(t, ctxBuf.String(), "from ctx")
}

func TestConstructorsAndLifecycle(t *testing.T) {
	assert.Equal(t, DefaultServerPort, NewServerWithOptions().conf.Port)
	assert.Equal(t, 1234, NewServerWithOptions(WithServerPort(1234)).conf.Port)

	t.Setenv("GRPC_PORT", "")
	require.NoError(t, os.Unsetenv("GRPC_PORT"))
	t.Setenv("GRPC_VERBOSE", "true")
	s := NewServerWithDefaultEnvPrefix()
	assert.Equal(t, DefaultServerPort, s.conf.Port)
	assert.True(t, s.conf.Verbose)

	assert.True(t, NewClientWithDefaultEnvPrefix().conf.Verbose)
	assert.True(t, NewClientWithOptions(WithClientVerbose(true)).conf.Verbose)
	assert.Len(t, NewClientWithOptions(WithClientUnaryInterceptors(grpc_prometheus.UnaryClientInterceptor)).conf.unaryInterceptors, 1)

	// Start on a taken port fails; Shutdown with an expired context forces a stop.
	var lc net.ListenConfig
	taken, err := lc.Listen(t.Context(), "tcp", ":0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = taken.Close() })
	busy := NewServerWithOptions(WithServerPort(taken.Addr().(*net.TCPAddr).Port))
	require.Error(t, busy.Start())

	free := NewServerWithOptions(WithServerPort(0))
	errCh := make(chan error, 1)
	go func() { errCh <- free.Start() }()
	time.Sleep(50 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = free.Shutdown(ctx)
	select {
	case <-errCh:
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return after Shutdown")
	}
}

// A server that echoes the metadata it received.
type mdServer struct {
	healthpb.UnimplementedHealthServer
	got chan metadata.MD
}

func (m *mdServer) Check(ctx context.Context, _ *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	m.got <- md
	return &healthpb.HealthCheckResponse{}, nil
}

func TestClientForwardingMode(t *testing.T) {
	srv := NewServerWithOptions()
	ms := &mdServer{got: make(chan metadata.MD, 1)}
	healthpb.RegisterHealthServer(srv.Server(), ms)
	var lc net.ListenConfig
	lis, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	conn, err := NewClientWithOptions(WithClientMetadataForwarding(contextinterceptor.ForwardAllowlist)).
		Dial(t.Context(), lis.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	// As if inside a server handler that received these headers.
	ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs(
		"app-ctx-bin", `{"user":{"user_id":"7"}}`,
		"authorization", "Bearer END-USER",
	))
	_, err = healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{})
	require.NoError(t, err)
	got := <-ms.got
	assert.Equal(t, []string{`{"user":{"user_id":"7"}}`}, got.Get("app-ctx-bin"))
	assert.Empty(t, got.Get("authorization"))
}
