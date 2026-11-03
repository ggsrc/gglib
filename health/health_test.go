package health

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clock is a settable time source.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// dep is a check whose result the test controls.
type dep struct{ err atomic.Value }

func (d *dep) set(err error) { d.err.Store(&err) }
func (d *dep) OK(context.Context) error {
	if p, ok := d.err.Load().(*error); ok {
		return *p
	}
	return nil
}

func testConfig() *Config {
	return &Config{ProbeInterval: 5 * time.Millisecond, ProbeTimeout: 50 * time.Millisecond, StuckAfter: time.Minute, Ready: true, Alive: true}
}

func probe(t *testing.T, s *Server, path string) int {
	t.Helper()
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
	return rec.Code
}

func eventually(t *testing.T, s *Server, path string, want int) {
	t.Helper()
	require.Eventually(t, func() bool { return probe(t, s, path) == want }, 2*time.Second, 2*time.Millisecond, "%s never returned %d", path, want)
}

func newTest(t *testing.T, conf *Config, c *clock, hc ...HealthCheckable) *Server {
	t.Helper()
	s := newServer(conf, nil, c.now, hc...)
	t.Cleanup(s.Close)
	return s
}

func TestHealthy(t *testing.T) {
	c := &clock{t: time.Unix(1000, 0)}
	s := newTest(t, testConfig(), c, &dep{})
	eventually(t, s, "/health/ready", http.StatusOK)
	assert.Equal(t, http.StatusOK, probe(t, s, "/health/alive"))
}

// Readiness used to pass before any check had run.
func TestNotReadyBeforeFirstRound(t *testing.T) {
	block := make(chan struct{})
	t.Cleanup(func() { close(block) })
	conf := testConfig()
	conf.ProbeTimeout = time.Hour
	s := newTest(t, conf, &clock{t: time.Unix(1000, 0)}, checkFunc(func(ctx context.Context) error { <-block; return nil }))
	assert.Equal(t, http.StatusServiceUnavailable, probe(t, s, "/health/ready"))
	assert.Equal(t, http.StatusOK, probe(t, s, "/health/alive"))
}

type checkFunc func(context.Context) error

func (f checkFunc) OK(ctx context.Context) error { return f(ctx) }

// A dependency outage used to fail liveness too, restarting every pod at
// once (G4). Now readiness fails, liveness holds, and both recover.
func TestDependencyOutageAndRecovery(t *testing.T) {
	c := &clock{t: time.Unix(1000, 0)}
	d := &dep{}
	s := newTest(t, testConfig(), c, d)
	eventually(t, s, "/health/ready", http.StatusOK)

	d.set(errors.New("db down"))
	eventually(t, s, "/health/ready", http.StatusServiceUnavailable)
	assert.Equal(t, http.StatusOK, probe(t, s, "/health/alive"), "an outage alone does not restart the pod")

	d.set(nil)
	eventually(t, s, "/health/ready", http.StatusOK)
	assert.Equal(t, http.StatusOK, probe(t, s, "/health/alive"))
}

// G4: if checks keep failing past StuckAfter (for example the dependency is
// back but this instance's client never recovers), liveness fails so the pod
// is restarted.
func TestStuckAfterFailsLiveness(t *testing.T) {
	c := &clock{t: time.Unix(1000, 0)}
	d := &dep{}
	d.set(errors.New("stuck client"))
	s := newTest(t, testConfig(), c, d)
	eventually(t, s, "/health/ready", http.StatusServiceUnavailable)
	assert.Equal(t, http.StatusOK, probe(t, s, "/health/alive"))

	// The fake clock jumps; wait for a round at the new time so that the
	// loop-stopped check does not fire.
	c.advance(59 * time.Second)
	eventually(t, s, "/health/alive", http.StatusOK)
	c.advance(2 * time.Second)
	eventually(t, s, "/health/alive", http.StatusServiceUnavailable)

	d.set(nil)
	eventually(t, s, "/health/alive", http.StatusOK)
	eventually(t, s, "/health/ready", http.StatusOK)
}

// A check that never returns used to keep the state at "healthy" forever.
func TestHangingCheckTimesOut(t *testing.T) {
	block := make(chan struct{})
	t.Cleanup(func() { close(block) })
	var calls atomic.Int32
	hang := checkFunc(func(ctx context.Context) error {
		calls.Add(1)
		<-block // ignores ctx on purpose
		return nil
	})
	s := newTest(t, testConfig(), &clock{t: time.Unix(1000, 0)}, &dep{}, hang)
	eventually(t, s, "/health/ready", http.StatusServiceUnavailable)
	time.Sleep(30 * time.Millisecond)
	assert.Equal(t, int32(1), calls.Load(), "a hanging check is not started again while it runs")
}

func TestStoppedLoopFailsLiveness(t *testing.T) {
	c := &clock{t: time.Unix(1000, 0)}
	s := newTest(t, testConfig(), c, &dep{})
	eventually(t, s, "/health/ready", http.StatusOK)
	s.Close()
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, http.StatusOK, probe(t, s, "/health/alive"))
	c.advance(time.Second)
	assert.Equal(t, http.StatusServiceUnavailable, probe(t, s, "/health/alive"))
}

// Stop used to fail liveness as well, which could restart a container in
// the middle of its graceful drain.
func TestStopFailsReadinessOnly(t *testing.T) {
	s := newTest(t, testConfig(), &clock{t: time.Unix(1000, 0)}, &dep{})
	eventually(t, s, "/health/ready", http.StatusOK)
	s.Stop()
	assert.Equal(t, http.StatusServiceUnavailable, probe(t, s, "/health/ready"))
	assert.Equal(t, http.StatusOK, probe(t, s, "/health/alive"))
}

func TestFlagsAndReadyCount(t *testing.T) {
	conf := testConfig()
	conf.ReadyCount = 1000
	d := &dep{}
	d.set(errors.New("flaky"))
	s := newTest(t, conf, &clock{t: time.Unix(1000, 0)}, d)
	eventually(t, s, "/health/ready", http.StatusOK)

	s.Ready(false)
	assert.Equal(t, http.StatusServiceUnavailable, probe(t, s, "/health/ready"))
	s.Ready(true)
	s.Alive(false)
	assert.Equal(t, http.StatusServiceUnavailable, probe(t, s, "/health/alive"))
}

func TestAddHooks(t *testing.T) {
	s := newTest(t, testConfig(), &clock{t: time.Unix(1000, 0)})
	eventually(t, s, "/health/ready", http.StatusOK)
	var wg sync.WaitGroup
	for range 5 {
		wg.Go(func() { s.AddHooks(func(context.Context) error { return errors.New("x") }) })
	}
	wg.Wait()
	eventually(t, s, "/health/ready", http.StatusServiceUnavailable)
}

func TestGoCheck(t *testing.T) {
	require.NoError(t, GoCheck(t.Context()))
	require.NoError(t, GoCheck(t.Context(), func(context.Context) error { return nil }))
	want := errors.New("bad")
	require.ErrorIs(t, GoCheck(t.Context(), func(context.Context) error { return nil }, func(context.Context) error { return want }), want)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	block := make(chan struct{})
	defer close(block)
	require.ErrorIs(t, GoCheck(ctx, func(context.Context) error { <-block; return nil }), context.DeadlineExceeded)
}

func TestConstructorsAndStart(t *testing.T) {
	t.Setenv("HEALTHCHECK_PORT", "")
	require.NoError(t, os.Unsetenv("HEALTHCHECK_PORT"))
	s := InitHealthCheck(&dep{})
	t.Cleanup(s.Close)
	assert.Equal(t, 8080, s.conf.Port)
	assert.Equal(t, 3*time.Minute, s.conf.StuckAfter)
	assert.True(t, s.conf.Ready)

	mux := http.NewServeMux()
	custom := NewWithDefaultEnvPrefix(mux)
	t.Cleanup(custom.Close)
	assert.Same(t, mux, custom.Router(), "the probes are registered on the given router")

	var lc net.ListenConfig
	taken, err := lc.Listen(t.Context(), "tcp", ":0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = taken.Close() })
	conf := testConfig()
	conf.Port = taken.Addr().(*net.TCPAddr).Port
	busy := newTest(t, conf, &clock{t: time.Unix(1000, 0)})
	require.Error(t, busy.Start())

	conf2 := testConfig()
	conf2.Port = 0
	free := newTest(t, conf2, &clock{t: time.Unix(1000, 0)})
	errCh := make(chan error, 1)
	go func() { errCh <- free.Start() }()
	select {
	case err := <-errCh:
		t.Fatalf("Start returned early: %v", err)
	case <-time.After(20 * time.Millisecond):
	}

	t.Setenv("HEALTHCHECK_PROBEINTERVAL", "nonsense")
	assert.Panics(t, func() { New("healthcheck", nil) })
}
