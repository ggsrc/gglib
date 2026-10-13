package app

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	gggrpc "github.com/ggsrc/gglib/grpc"
	"github.com/ggsrc/gglib/resource"
)

type writableResource struct {
	closed  atomic.Bool
	stopped chan struct{}
}

func (r *writableResource) Name() string                { return "request-dependency" }
func (r *writableResource) Init(context.Context) error  { return nil }
func (r *writableResource) Start(context.Context) error { return nil }
func (r *writableResource) OK(context.Context) error    { return nil }
func (r *writableResource) Stop(context.Context) error {
	r.closed.Store(true)
	close(r.stopped)
	return nil
}

func (r *writableResource) write() error {
	if r.closed.Load() {
		return errors.New("closed pool")
	}
	return nil
}

type heldRequest struct {
	healthpb.UnimplementedHealthServer
	started    chan struct{}
	release    chan struct{}
	dependency *writableResource
}

func (h *heldRequest) Check(
	ctx context.Context,
	_ *healthpb.HealthCheckRequest,
) (*healthpb.HealthCheckResponse, error) {
	close(h.started)
	select {
	case <-h.release:
		if err := h.dependency.write(); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
	case <-ctx.Done():
		return nil, status.FromContextError(ctx.Err()).Err()
	}
}

type drainingListener struct {
	*bufconn.Listener
	once     sync.Once
	draining chan struct{}
}

func (l *drainingListener) Close() error {
	l.once.Do(func() { close(l.draining) })
	return l.Listener.Close()
}

func pendingRPC(t *testing.T) (*App, *heldRequest, *drainingListener, <-chan error) {
	t.Helper()
	dependency := &writableResource{stopped: make(chan struct{})}
	handler := &heldRequest{
		started:    make(chan struct{}),
		release:    make(chan struct{}),
		dependency: dependency,
	}
	server := gggrpc.NewServerWithDefaultEnvPrefix()
	healthpb.RegisterHealthServer(server.Server(), handler)
	listener := &drainingListener{
		Listener: bufconn.Listen(1024 * 1024),
		draining: make(chan struct{}),
	}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Server().Stop)
	connection, err := googlegrpc.NewClient(
		"passthrough:///shutdown-test",
		googlegrpc.WithTransportCredentials(insecure.NewCredentials()),
		googlegrpc.WithContextDialer(
			func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) },
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	result := make(chan error, 1)
	go func() {
		_, err := healthpb.NewHealthClient(connection).
			Check(t.Context(), &healthpb.HealthCheckRequest{})
		result <- err
	}()
	select {
	case <-handler.started:
	case <-time.After(3 * time.Second):
		t.Fatal("RPC did not start")
	}
	return &App{
		grpcServer:      server,
		resourceManager: resource.NewResourceManager([]resource.Resource{dependency}),
	}, handler, listener, result
}

func TestStopPreservesDependencyUntilInflightRPCFinishes(t *testing.T) {
	application, handler, listener, rpcResult := pendingRPC(t)
	stopResult := make(chan error, 1)
	go func() { stopResult <- application.Stop(t.Context()) }()
	select {
	case <-listener.draining:
	case <-time.After(3 * time.Second):
		t.Fatal("server did not begin draining")
	}
	select {
	case <-handler.dependency.stopped:
		t.Fatal("dependency closed while RPC still needs to settle")
	case <-time.After(100 * time.Millisecond):
	}
	close(handler.release)
	if err := <-rpcResult; err != nil {
		t.Fatalf("in-flight settlement failed: %v", err)
	}
	if err := <-stopResult; err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}
	select {
	case <-handler.dependency.stopped:
	default:
		t.Fatal("dependency not closed after drain")
	}
}

func TestStopDeadlineForceStopsRPCAndClosesDependency(t *testing.T) {
	application, handler, _, rpcResult := pendingRPC(t)
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if err := application.Stop(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline error, got %v", err)
	}
	select {
	case <-rpcResult:
	case <-time.After(3 * time.Second):
		t.Fatal("RPC not canceled by forced stop")
	}
	select {
	case <-handler.dependency.stopped:
	default:
		t.Fatal("dependency not closed after force stop")
	}
}
