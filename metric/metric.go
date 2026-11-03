package metric

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	conf       *Config
	httpServer *http.Server
	errCh      chan error
}

type Config struct {
	Port int `default:"4014"`
}

func NewWithOptions(opts ...Option) *Server {
	conf := &Config{}
	for _, opt := range opts {
		opt(conf)
	}
	return &Server{conf: conf}
}

func NewWithDefaultEnvPrefix() *Server {
	return New("metric")
}

func New(envPrefix string) *Server {
	conf := &Config{}
	envconfig.MustProcess(envPrefix, conf)
	return &Server{conf: conf}
}

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

func (s *Server) Stop(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) OK(ctx context.Context) error {
	select {
	case err := <-s.errCh:
		return err
	default:
		return nil
	}
}
