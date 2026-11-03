package metric

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server is an HTTP server that exposes /metrics.
type Server struct {
	conf       *Config
	httpServer *http.Server
	errCh      chan error
}

// Config configures a Server. Port defaults to 4014.
type Config struct {
	Port int `default:"4014"`
}

// NewWithOptions returns a Server configured by opts.
func NewWithOptions(opts ...Option) *Server {
	conf := &Config{}
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

// Start starts serving in a background goroutine and returns immediately.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", s.conf.Port),
		Handler:           mux,
		ReadHeaderTimeout: time.Second * 5,
	}
	s.httpServer = server
	s.errCh = make(chan error)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.errCh <- err
		}
		close(s.errCh)
	}()
	return nil
}

// Stop shuts the server down gracefully, waiting until ctx is done.
func (s *Server) Stop(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// OK returns the error that stopped the server, if one has occurred.
func (s *Server) OK(ctx context.Context) error {
	select {
	case err := <-s.errCh:
		return err
	default:
		return nil
	}
}
