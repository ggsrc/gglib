package health

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog/log"
)

// HttpRouter is the subset of *http.ServeMux that Server needs. Pass one to
// serve the probes on an existing router.
type HttpRouter interface { //nolint:revive,staticcheck // exported name kept until the health rework
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
	ServeHTTP(writer http.ResponseWriter, request *http.Request)
}

// Server answers liveness and readiness probes from the results of its checks.
type Server struct {
	conf      *Config
	hooks     []Checkable
	stop      bool
	hooksLock sync.RWMutex

	consecutive int

	ready bool
	alive bool

	httpRouter HttpRouter
}

// HealthCheckable is anything that can report its own health, such as a
// resource.
type HealthCheckable interface {
	OK(ctx context.Context) error
}

// Config configures a Server. ReadyCount and LiveCount are the numbers of
// consecutive failed checks that are still tolerated by the readiness and
// liveness probes.
type Config struct {
	Port          int `default:"8080"`
	ReadyCount    int
	LiveCount     int           `default:"3"`
	ProbeInterval time.Duration `default:"5s"`
	ProbeTimeout  time.Duration `default:"5s"`
	Ready         bool          `default:"true"`
	Alive         bool          `default:"true"`
}

// NewWithDefaultEnvPrefix returns a Server configured from HEALTHCHECK_*
// environment variables. See New.
func NewWithDefaultEnvPrefix(httpRouter HttpRouter, hc ...HealthCheckable) *Server {
	return New("healthcheck", httpRouter, hc...)
}

// New returns a Server configured from environment variables with the given
// prefix, and starts its check loop. It panics if the variables cannot be
// parsed. A nil httpRouter makes Start create its own.
func New(envPrefix string, httpRouter HttpRouter, hc ...HealthCheckable) *Server {
	conf := &Config{}
	envconfig.MustProcess(envPrefix, conf)
	var hooks []Checkable
	for _, h := range hc {
		hooks = append(hooks, h.OK)
	}
	s := &Server{
		conf:       conf,
		hooks:      hooks,
		ready:      conf.Ready,
		alive:      conf.Alive,
		httpRouter: httpRouter,
	}
	go s.start()
	return s
}

func (s *Server) start() {
	for {
		<-time.After(s.conf.ProbeInterval)
		ctx, cancel := context.WithTimeout(context.Background(), s.conf.ProbeTimeout)
		s.hooksLock.RLock()
		if err := GoCheck(ctx, s.hooks...); err != nil {
			s.consecutive++
			log.Error().Err(err).Msg("healthcheck failed")
		} else {
			s.consecutive = 0
		}
		s.hooksLock.RUnlock()
		cancel()
	}
}

func (s *Server) readinessHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.ready {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if s.stop {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if s.consecutive <= s.conf.ReadyCount {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	}
}

// AddHooks registers more checks.
func (s *Server) AddHooks(hooks ...Checkable) {
	s.hooksLock.Lock()
	defer s.hooksLock.Unlock()
	s.hooks = append(s.hooks, hooks...)
}

// Ready sets whether the service is ready to serve.
func (s *Server) Ready(serviceReady bool) {
	s.ready = serviceReady
}

// Alive sets whether the service is alive.
func (s *Server) Alive(serviceAlive bool) {
	s.alive = serviceAlive
}

func (s *Server) livenessHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.stop {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if s.consecutive <= s.conf.LiveCount {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	}
}

// Start serves the probes and blocks until the HTTP server fails.
func (s *Server) Start() error {
	if s.httpRouter == nil {
		mux := http.NewServeMux()
		mux.HandleFunc("/health/ready", s.readinessHandler())
		mux.HandleFunc("/health/alive", s.livenessHandler())
		s.httpRouter = mux
	}
	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", s.conf.Port),
		Handler:           s.httpRouter,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return httpServer.ListenAndServe()
}

// Router returns the router the probes are served on.
func (s *Server) Router() HttpRouter {
	return s.httpRouter
}

// Stop makes both probes fail, so that the pod stops receiving traffic.
func (s *Server) Stop() {
	s.stop = true
}

// Checkable is a single health check.
type Checkable func(context.Context) error

// GoCheck is a helper to run multiple check functions in parallel and fail if one fails.
func GoCheck(ctx context.Context, toCheck ...Checkable) error {
	ch := make(chan error, len(toCheck))
	for _, f := range toCheck {
		go func(f func(context.Context) error) {
			ch <- f(ctx)
		}(f)
	}
	for range toCheck {
		if err := <-ch; err != nil {
			return err
		}
	}
	return nil
}
