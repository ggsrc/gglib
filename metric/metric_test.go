package metric

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func get(t *testing.T, url string) (int, string, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(body), nil
}

func TestServesMetrics(t *testing.T) {
	s := NewWithOptions(WithPort(0))
	assert.Nil(t, s.Addr(), "no address before Start")
	require.NoError(t, s.Start(t.Context()))
	t.Cleanup(func() { _ = s.Stop(context.Background()) })

	code, body, err := get(t, fmt.Sprintf("http://%s/metrics", s.Addr()))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, "go_goroutines", "serves the default registry")
	require.NoError(t, s.OK(t.Context()))

	code, _, err = get(t, fmt.Sprintf("http://%s/other", s.Addr()))
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, code)
}

// Start used to return nil when the port was taken, and OK reported the error
// only once.
func TestStartReturnsBindError(t *testing.T) {
	var lc net.ListenConfig
	taken, err := lc.Listen(t.Context(), "tcp", ":0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = taken.Close() })

	s := NewWithOptions(WithPort(taken.Addr().(*net.TCPAddr).Port))
	err = s.Start(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "metric: listen")
	require.NoError(t, s.Stop(t.Context()), "Stop after a failed Start")
}

func TestStartTwice(t *testing.T) {
	s := NewWithOptions(WithPort(0))
	require.NoError(t, s.Start(t.Context()))
	t.Cleanup(func() { _ = s.Stop(context.Background()) })
	require.Error(t, s.Start(t.Context()))
}

// Stop before Start used to panic with a nil pointer dereference.
func TestStopBeforeStart(t *testing.T) {
	require.NoError(t, NewWithOptions().Stop(t.Context()))
}

func TestStopShutsDown(t *testing.T) {
	s := NewWithOptions(WithPort(0))
	require.NoError(t, s.Start(t.Context()))
	url := fmt.Sprintf("http://%s/metrics", s.Addr())

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, s.Stop(ctx))

	_, _, err := get(t, url)
	require.Error(t, err, "nothing listens after Stop")
	require.NoError(t, s.OK(t.Context()), "a graceful stop is not an error")
}

type failingListener struct{ net.Listener }

func (failingListener) Accept() (net.Conn, error) { return nil, errors.New("accept failed") }

func TestOKReportsServeErrorEveryTime(t *testing.T) {
	var lc net.ListenConfig
	lis, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = lis.Close() })

	s := NewWithOptions()
	s.mu.Lock()
	s.serve(failingListener{lis})
	s.mu.Unlock()

	require.Eventually(t, func() bool { return s.OK(t.Context()) != nil }, 5*time.Second, 10*time.Millisecond)
	for range 3 {
		require.EqualError(t, s.OK(t.Context()), "accept failed")
	}
}

func TestConfig(t *testing.T) {
	assert.Equal(t, DefaultPort, NewWithOptions().conf.Port, "NewWithOptions used to default to port 0")
	assert.Equal(t, 1234, NewWithOptions(WithPort(1234)).conf.Port)

	t.Setenv("METRIC_PORT", "")
	require.NoError(t, os.Unsetenv("METRIC_PORT")) // t.Setenv restores it after the test
	assert.Equal(t, DefaultPort, NewWithDefaultEnvPrefix().conf.Port)

	t.Setenv("METRIC_PORT", "4321")
	assert.Equal(t, 4321, NewWithDefaultEnvPrefix().conf.Port)

	t.Setenv("CUSTOM_PORT", "5555")
	assert.Equal(t, 5555, New("custom").conf.Port)

	t.Setenv("CUSTOM_PORT", "not-a-number")
	assert.Panics(t, func() { New("custom") })
}
