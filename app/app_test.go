package app

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gggrpc "github.com/ggsrc/gglib/grpc"
	"github.com/ggsrc/gglib/otel"
	"github.com/ggsrc/gglib/resource"
	ggzerolog "github.com/ggsrc/gglib/zerolog"
)

// testEnv makes every server pick a free port and the health loop fast.
func testEnv(t *testing.T) {
	t.Helper()
	t.Setenv("METRIC_PORT", "0")
	t.Setenv("HEALTHCHECK_PORT", "0")
	t.Setenv("HEALTHCHECK_PROBEINTERVAL", "5ms")
	t.Setenv("GRPC_PORT", "0")
}

func newApp(t *testing.T, opts ...Option) *App {
	t.Helper()
	testEnv(t)
	prev := zerolog.GlobalLevel()
	t.Cleanup(func() { zerolog.SetGlobalLevel(prev) })
	a, err := New(t.Context(), opts...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = a.Stop(context.Background()) })
	return a
}

func probe(a *App, path string) int {
	rec := httptest.NewRecorder()
	a.health.Router().ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil))
	return rec.Code
}

func waitReady(t *testing.T, a *App) {
	t.Helper()
	require.Eventually(t, func() bool { return probe(a, "/health/ready") == http.StatusOK }, 5*time.Second, 2*time.Millisecond,
		"never ready")
}

type recorder struct {
	mu     sync.Mutex
	events []string
}

func (r *recorder) add(e string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
}

func (r *recorder) list() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.events...)
}

type fakeResource struct {
	name         string
	rec          *recorder
	initFailures atomic.Int32 // Init fails this many more times
	inits        atomic.Int32
	startErr     error
	okErr        error
}

func (f *fakeResource) Name() string { return f.name }
func (f *fakeResource) Init(context.Context) error {
	f.inits.Add(1)
	if f.initFailures.Add(-1) >= 0 {
		return errors.New(f.name + " unreachable")
	}
	return nil
}

func (f *fakeResource) Start(context.Context) error {
	f.rec.add("start " + f.name)
	return f.startErr
}

func (f *fakeResource) Stop(context.Context) error {
	f.rec.add("stop " + f.name)
	return nil
}
func (f *fakeResource) OK(context.Context) error { return f.okErr }

func managerOf(rs ...resource.Resource) resource.ResourceManager {
	return resource.NewResourceManager(rs)
}

// The probes are answered from New on: alive, and not ready until Run.
func TestNewServesProbesAliveNotReady(t *testing.T) {
	a := newApp(t)
	assert.Equal(t, http.StatusOK, probe(a, "/health/alive"))
	time.Sleep(20 * time.Millisecond) // several check rounds
	assert.Equal(t, http.StatusServiceUnavailable, probe(a, "/health/ready"))
	assert.Equal(t, http.StatusOK, probe(a, "/health/alive"))
	assert.NotNil(t, a.metrics.Addr(), "metrics are served")
}

// A database that is down at start-up used to make Init fail and the process
// exit, so Kubernetes restarted it with a growing back-off.
func TestInitRetriesUntilTheDependencyIsUp(t *testing.T) {
	a := newApp(t)
	db := &fakeResource{name: "db", rec: &recorder{}}
	db.initFailures.Store(2)
	cache := &fakeResource{name: "cache", rec: &recorder{}}

	require.NoError(t, a.initWithRetry(t.Context(), managerOf(cache, db), time.Millisecond, 2*time.Millisecond))
	assert.Equal(t, int32(3), db.inits.Load())
	assert.Equal(t, int32(1), cache.inits.Load(), "resources already initialized are not initialized again")
	assert.Equal(t, http.StatusServiceUnavailable, probe(a, "/health/ready"))
	assert.Equal(t, http.StatusOK, probe(a, "/health/alive"), "retrying does not restart the pod")
}

func TestInitGivesUpWhenTheContextIsDone(t *testing.T) {
	a := newApp(t)
	db := &fakeResource{name: "db", rec: &recorder{}}
	db.initFailures.Store(1 << 30)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
	defer cancel()
	err := a.initWithRetry(ctx, managerOf(db), time.Millisecond, 5*time.Millisecond)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.ErrorContains(t, err, "db unreachable")
	assert.Greater(t, db.inits.Load(), int32(2))

	canceled, cancelNow := context.WithCancel(t.Context())
	cancelNow()
	require.ErrorIs(t, a.Init(canceled, managerOf(db)), context.Canceled)
}

func run(t *testing.T, a *App, ctx context.Context, srv *gggrpc.Server, rms ...resource.ResourceManager) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx, srv, rms...) }()
	return done
}

func wait(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return")
		return nil
	}
}

func TestRunServesThenStopsInReverseOrder(t *testing.T) {
	a := newApp(t)
	rec := &recorder{}
	db := &fakeResource{name: "db", rec: rec}
	worker := &fakeResource{name: "worker", rec: rec}
	srv := gggrpc.NewServerWithDefaultEnvPrefix()

	ctx, cancel := context.WithCancel(t.Context())
	done := run(t, a, ctx, srv, managerOf(db), managerOf(worker))
	waitReady(t, a)
	assert.Same(t, srv, a.grpc)

	cancel()
	require.NoError(t, wait(t, done))
	assert.Equal(t, []string{"start db", "start worker", "stop worker", "stop db"}, rec.list())
	assert.Equal(t, http.StatusServiceUnavailable, probe(a, "/health/ready"))
	require.NoError(t, a.Stop(t.Context()), "a second Stop returns the first result")
}

// Readiness follows the resources' checks once Run has started them.
func TestRunReadinessFollowsTheResources(t *testing.T) {
	a := newApp(t)
	db := &fakeResource{name: "db", rec: &recorder{}, okErr: errors.New("db down")}
	ctx, cancel := context.WithCancel(t.Context())
	done := run(t, a, ctx, nil, managerOf(db))
	time.Sleep(30 * time.Millisecond)
	assert.Equal(t, http.StatusServiceUnavailable, probe(a, "/health/ready"))
	cancel()
	require.NoError(t, wait(t, done))
}

// A worker process runs without gRPC: metrics, probes and resources only.
func TestRunWithoutGRPC(t *testing.T) {
	a := newApp(t)
	rec := &recorder{}
	ctx, cancel := context.WithCancel(t.Context())
	done := run(t, a, ctx, nil, managerOf(&fakeResource{name: "worker", rec: rec}))
	waitReady(t, a)
	cancel()
	require.NoError(t, wait(t, done))
	assert.Equal(t, []string{"start worker", "stop worker"}, rec.list())
}

func TestRunStopsOnSIGTERM(t *testing.T) {
	a := newApp(t)
	done := run(t, a, context.Background(), nil)
	waitReady(t, a)
	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGTERM))
	require.NoError(t, wait(t, done))
}

func TestRunFailsWhenAResourceDoesNotStart(t *testing.T) {
	a := newApp(t)
	rec := &recorder{}
	db := &fakeResource{name: "db", rec: rec}
	worker := &fakeResource{name: "worker", rec: rec, startErr: errors.New("no queue")}
	err := wait(t, run(t, a, t.Context(), gggrpc.NewServerWithDefaultEnvPrefix(), managerOf(db), managerOf(worker)))
	require.ErrorContains(t, err, "app: start resources")
	require.ErrorContains(t, err, "no queue")
	assert.Contains(t, rec.list(), "stop db", "resources already started are stopped")
	assert.Nil(t, a.grpc, "the gRPC server was not started")
}

func TestRunFailsWhenTheGRPCPortIsTaken(t *testing.T) {
	a := newApp(t)
	taken, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", ":0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = taken.Close() })
	t.Setenv("GRPC_PORT", strconv.Itoa(taken.Addr().(*net.TCPAddr).Port))

	rec := &recorder{}
	err = wait(t, run(t, a, t.Context(), gggrpc.NewServerWithDefaultEnvPrefix(), managerOf(&fakeResource{name: "db", rec: rec})))
	require.ErrorContains(t, err, "grpc: listen")
	assert.Equal(t, []string{"start db", "stop db"}, rec.list())
	assert.Equal(t, http.StatusServiceUnavailable, probe(a, "/health/ready"))
}

// The gRPC server's error used to be dropped: the process kept running
// without serving.
func TestRunStopsWhenTheGRPCServerStops(t *testing.T) {
	a := newApp(t)
	srv := gggrpc.NewServerWithDefaultEnvPrefix()
	done := run(t, a, t.Context(), srv)
	waitReady(t, a)
	srv.Server().Stop()
	require.EqualError(t, wait(t, done), "app: grpc server: stopped")
}

func TestRunStopsWhenTheHealthServerFails(t *testing.T) {
	a := newApp(t)
	done := run(t, a, t.Context(), nil)
	waitReady(t, a)
	a.healthErr <- errors.New("boom")
	require.EqualError(t, wait(t, done), "app: health server: boom")
}

func TestNewFailsWhenAPortIsTaken(t *testing.T) {
	taken, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", ":0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = taken.Close() })
	port := strconv.Itoa(taken.Addr().(*net.TCPAddr).Port)

	testEnv(t)
	t.Setenv("METRIC_PORT", port)
	_, err = New(t.Context())
	require.ErrorContains(t, err, "metric: listen")

	t.Setenv("METRIC_PORT", "0")
	t.Setenv("HEALTHCHECK_PORT", port)
	_, err = New(t.Context())
	require.ErrorContains(t, err, "health: listen")
}

// Telemetry is set up once per process; a later New leaves it to its owner.
func TestTelemetrySetUpElsewhere(t *testing.T) {
	newApp(t)
	a := newApp(t)
	assert.Nil(t, a.telemetry)
	require.NoError(t, a.flushTelemetry(t.Context()))
}

func TestStopFlushesTelemetryLast(t *testing.T) {
	a := newApp(t)
	var flushed atomic.Bool
	a.telemetry = func(ctx context.Context) error {
		flushed.Store(true)
		require.NoError(t, ctx.Err(), "the flush gets its own deadline")
		return errors.New("collector down")
	}
	a.conf.shutdownTimeout = time.Nanosecond
	err := a.Stop(t.Context())
	require.ErrorContains(t, err, "app: telemetry flush: collector down")
	assert.True(t, flushed.Load())
}

// The deployment's service name and version win over the options, so that
// service names in Uptrace do not change.
func TestTelemetryOptionsAreFallbacks(t *testing.T) {
	for _, k := range []string{"SERVICE_NAME", "OTEL_SERVICE_NAME", "SERVICE_VERSION", "GIT_SHA"} {
		t.Setenv(k, "")
		require.NoError(t, os.Unsetenv(k))
	}
	conf := newConfig([]Option{WithServerName("svc"), WithServerVersion("v1"), WithTelemetry(otel.WithRuntimeMetrics(true))})
	assert.Len(t, telemetryOptions(conf), 3)

	t.Setenv("OTEL_SERVICE_NAME", "svc-server")
	t.Setenv("GIT_SHA", "abc123")
	assert.Len(t, telemetryOptions(conf), 1, "only WithTelemetry's option")
}

func TestOptions(t *testing.T) {
	a := newApp(t, WithDebug(true), WithLogging(ggzerolog.WithoutCaller()), WithShutdownTimeout(time.Minute))
	assert.Equal(t, zerolog.DebugLevel, zerolog.GlobalLevel())
	assert.Equal(t, time.Minute, a.conf.shutdownTimeout)
	assert.Len(t, a.conf.logging, 1)
	assert.Equal(t, DefaultResourceShutDownTimeout, newConfig(nil).shutdownTimeout)
}
