package metric

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// DefaultPort is the port the server listens on unless configured otherwise.
const DefaultPort = 4014

// Server is an HTTP server that exposes the Prometheus default registry at
// /metrics. The endpoint has no authentication; it is meant to be reachable
// only inside the cluster.
type Server struct {
	conf *Config

	mu         sync.Mutex
	httpServer *http.Server
	addr       net.Addr
	serveErr   error
}

// Config configures a Server. Port defaults to DefaultPort; port 0 picks a
// free port, which Addr reports after Start.
type Config struct {
	Port int `default:"4014"`
}

// NewWithOptions returns a Server configured by opts, listening on
// DefaultPort unless WithPort is given.
func NewWithOptions(opts ...Option) *Server {
	conf := &Config{Port: DefaultPort}
	for _, opt := range opts {
		opt(conf)
	}
	return &Server{conf: conf}
}

// NewWithDefaultEnvPrefix returns a Server configured from METRIC_*
// environment variables.
func NewWithDefaultEnvPrefix() *Server {
	return New("metric")
}

// New returns a Server configured from environment variables with the given
// prefix. It panics if they cannot be parsed.
func New(envPrefix string) *Server {
	conf := &Config{}
	envconfig.MustProcess(envPrefix, conf)
	return &Server{conf: conf}
}

// Start listens on the configured port and serves in a background goroutine.
// It returns an error if the port cannot be bound or the server was already
// started.
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.httpServer != nil {
		return errors.New("metric: server already started")
	}

	var lc net.ListenConfig
	lis, err := lc.Listen(ctx, "tcp", fmt.Sprintf(":%d", s.conf.Port))
	if err != nil {
		return fmt.Errorf("metric: listen: %w", err)
	}
	s.serve(lis)
	return nil
}

// serve serves on lis in a background goroutine. s.mu must be held.
func (s *Server) serve(lis net.Listener) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	s.httpServer = srv
	s.addr = lis.Addr()

	go func() {
		err := srv.Serve(lis)
		if errors.Is(err, http.ErrServerClosed) {
			return
		}
		s.mu.Lock()
		s.serveErr = err
		s.mu.Unlock()
	}()
}

// Addr returns the address the server listens on, or nil before Start.
func (s *Server) Addr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addr
}

// Stop shuts the server down gracefully, waiting until ctx is done. It does
// nothing if the server was never started.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	srv := s.httpServer
	s.mu.Unlock()
	if srv == nil {
		return nil
	}
	return srv.Shutdown(ctx)
}

// OK returns the error that stopped the server, if one has occurred. The
// error is reported on every call.
func (s *Server) OK(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.serveErr
}
