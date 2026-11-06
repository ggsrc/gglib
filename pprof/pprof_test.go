package pprof

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	rpprof "runtime/pprof"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func get(t *testing.T, h http.Handler, target string, header ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHealth(t *testing.T) {
	rec := get(t, NewWithOptions().Handler(), "/debug/health")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "ok\n", rec.Body.String())
}

func TestOnlyGET(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/debug/health", nil)
	rec := httptest.NewRecorder()
	NewWithOptions().Handler().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestTokenIsRequiredWhenSet(t *testing.T) {
	h := NewWithOptions(WithToken("secret")).Handler()

	assert.Equal(t, http.StatusUnauthorized, get(t, h, "/debug/health").Code)
	assert.Equal(t, http.StatusUnauthorized, get(t, h, "/debug/health", "Authorization", "Bearer wrong").Code)
	assert.Equal(t, http.StatusUnauthorized, get(t, h, "/debug/health", "Authorization", "Basic secret").Code)
	assert.Equal(t, http.StatusUnauthorized, get(t, h, "/debug/health", "Authorization", "Bearer secret extra").Code)
	assert.Equal(t, http.StatusOK, get(t, h, "/debug/health", "Authorization", "Bearer secret").Code)
	assert.Equal(t, http.StatusOK, get(t, h, "/debug/health", "Authorization", "bearer secret").Code)
	assert.Equal(t, http.StatusOK, get(t, h, "/debug/health", "X-Debug-Token", "secret").Code)
}

func TestLookupProfiles(t *testing.T) {
	h := NewWithOptions().Handler()
	for _, name := range []string{"goroutine", "heap"} {
		rec := get(t, h, "/debug/pprof/"+name)
		assert.Equal(t, http.StatusOK, rec.Code, name)
		assert.Equal(t, "application/octet-stream", rec.Header().Get("Content-Type"), name)
		assert.NotEmpty(t, rec.Body.Bytes(), name)

		rec = get(t, h, "/debug/pprof/"+name+"?debug=1")
		assert.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"), name)
		assert.Contains(t, rec.Body.String(), name+" profile:", name)

		// An unusable debug value falls back to the binary format.
		for _, bad := range []string{"x", "-1"} {
			rec = get(t, h, "/debug/pprof/"+name+"?debug="+bad)
			assert.Equal(t, "application/octet-stream", rec.Header().Get("Content-Type"), name)
		}
	}
}

// Only the listed endpoints exist; the full net/http/pprof index (and its
// trace endpoint) is not exposed.
func TestOtherEndpointsAreNotServed(t *testing.T) {
	h := NewWithOptions().Handler()
	for _, path := range []string{"/debug/pprof/", "/debug/pprof/trace", "/debug/pprof/cmdline"} {
		assert.Equal(t, http.StatusNotFound, get(t, h, path).Code, path)
	}
}

type failingWriter struct{ header http.Header }

func (w *failingWriter) Header() http.Header     { return w.header }
func (*failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }
func (*failingWriter) WriteHeader(int)           {}

func TestLookupWriteErrorIsHandled(t *testing.T) {
	w := &failingWriter{header: http.Header{}}
	handleLookup("goroutine")(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/debug/pprof/goroutine?debug=1", nil))
	assert.Equal(t, "text/plain; charset=utf-8", w.header.Get("Content-Type"))
}

func TestCPUProfileValidation(t *testing.T) {
	h := NewWithOptions(WithCPUProfileSeconds(1, 2)).Handler()
	for _, q := range []string{"0", "-3", "x"} {
		rec := get(t, h, "/debug/pprof/profile?seconds="+q)
		assert.Equal(t, http.StatusBadRequest, rec.Code, q)
		assert.Contains(t, rec.Body.String(), "positive integer", q)
	}
	rec := get(t, h, "/debug/pprof/profile?seconds=3")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "seconds exceeds max 2")
}

func TestCPUProfileRunsForTheRequestedTime(t *testing.T) {
	h := NewWithOptions(WithCPUProfileSeconds(1, 1)).Handler()
	start := time.Now()
	rec := get(t, h, "/debug/pprof/profile")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/octet-stream", rec.Header().Get("Content-Type"))
	assert.NotEmpty(t, rec.Body.Bytes())
	assert.GreaterOrEqual(t, time.Since(start), time.Second)
}

// A client that goes away ends the profile early, and only one CPU profile
// runs at a time.
func TestCPUProfileOneAtATimeAndCancel(t *testing.T) {
	s := NewWithOptions()
	h := s.Handler()

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan *httptest.ResponseRecorder)
	go func() {
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/debug/pprof/profile", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		done <- rec
	}()
	require.Eventually(t, s.cpuActive.Load, 5*time.Second, 5*time.Millisecond)

	rec := get(t, h, "/debug/pprof/profile")
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)

	cancel()
	select {
	case rec := <-done:
		assert.Equal(t, http.StatusOK, rec.Code)
	case <-time.After(5 * time.Second):
		t.Fatal("a canceled request must end the profile")
	}
	assert.False(t, s.cpuActive.Load())
}

// When another CPU profile is already running in the process, the request
// fails instead of hanging.
func TestCPUProfileAlreadyInUse(t *testing.T) {
	require.NoError(t, rpprof.StartCPUProfile(io.Discard))
	defer rpprof.StopCPUProfile()

	rec := get(t, NewWithOptions().Handler(), "/debug/pprof/profile")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestDisabledServerDoesNothing(t *testing.T) {
	s := NewWithOptions(WithEnabled(false), WithAddr("127.0.0.1:0"))
	assert.False(t, s.Enabled())
	require.NoError(t, s.Start(t.Context()))
	assert.Nil(t, s.Addr())
	require.NoError(t, s.Stop(t.Context()))
	require.NoError(t, s.OK(t.Context()))
}

func TestServes(t *testing.T) {
	s := NewWithOptions(WithAddr("127.0.0.1:0"))
	assert.True(t, s.Enabled())
	require.NoError(t, s.Start(t.Context()))
	t.Cleanup(func() { _ = s.Stop(context.Background()) })

	healthURL := fmt.Sprintf("http://%s/debug/health", s.Addr())
	resp, err := httpGet(t, healthURL)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	assert.Equal(t, "ok\n", string(body))
	require.NoError(t, s.OK(t.Context()))

	require.EqualError(t, s.Start(t.Context()), "pprof: server already started")
	require.NoError(t, s.Stop(t.Context()))
	resp, err = httpGet(t, healthURL)
	if resp != nil {
		_ = resp.Body.Close()
	}
	require.Error(t, err)
}

func httpGet(t *testing.T, url string) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	require.NoError(t, err)
	return http.DefaultClient.Do(req)
}

func TestStartReturnsBindError(t *testing.T) {
	var lc net.ListenConfig
	taken, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = taken.Close() })

	err = NewWithOptions(WithAddr(taken.Addr().String())).Start(t.Context())
	require.ErrorContains(t, err, "pprof: listen: ")
}

type failingListener struct{ net.Listener }

func (failingListener) Accept() (net.Conn, error) { return nil, errors.New("accept failed") }

func TestOKReportsServeError(t *testing.T) {
	var lc net.ListenConfig
	lis, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = lis.Close() })

	s := NewWithOptions()
	s.mu.Lock()
	s.serve(failingListener{lis})
	s.mu.Unlock()

	require.Eventually(t, func() bool { return s.OK(t.Context()) != nil }, 5*time.Second, 10*time.Millisecond)
	require.EqualError(t, s.OK(t.Context()), "accept failed")
}

func TestConfig(t *testing.T) {
	for _, k := range []string{"ENABLED", "ADDR", "TOKEN", "CPU_PROFILE_SECONDS", "CPU_PROFILE_MAX_SECONDS"} {
		t.Setenv("DEBUG_PPROF_"+k, "")
		require.NoError(t, os.Unsetenv("DEBUG_PPROF_"+k)) // t.Setenv restores it after the test
	}
	s := NewWithDefaultEnvPrefix()
	assert.Equal(t, Config{Addr: DefaultAddr, CPUProfileSeconds: 10, CPUProfileMaxSeconds: 10}, s.conf)
	assert.False(t, s.Enabled())

	t.Setenv("DEBUG_PPROF_ENABLED", "true")
	t.Setenv("DEBUG_PPROF_ADDR", ":7070")
	t.Setenv("DEBUG_PPROF_TOKEN", "tok")
	t.Setenv("DEBUG_PPROF_CPU_PROFILE_SECONDS", "30")
	t.Setenv("DEBUG_PPROF_CPU_PROFILE_MAX_SECONDS", "20")
	// The default length is capped at the maximum.
	assert.Equal(t, Config{Enabled: true, Addr: ":7070", Token: "tok", CPUProfileSeconds: 20, CPUProfileMaxSeconds: 20}, NewWithDefaultEnvPrefix().conf)

	// Non-positive lengths fall back to the default.
	s = NewWithOptions(WithCPUProfileSeconds(0, -1))
	assert.Equal(t, 10, s.conf.CPUProfileSeconds)
	assert.Equal(t, 10, s.conf.CPUProfileMaxSeconds)

	t.Setenv("CUSTOM_ENABLED", "not-a-bool")
	assert.Panics(t, func() { New("custom") })
}

func TestBearerToken(t *testing.T) {
	assert.Equal(t, "abc", bearerToken("Bearer abc"))
	assert.Empty(t, bearerToken(""))
	assert.Empty(t, bearerToken("abc"))
	assert.False(t, tokenEqual("", ""))
}
