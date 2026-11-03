package health

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog/log"
)

// HttpRouter is the subset of *http.ServeMux that Server needs. Pass one to
// serve the probes on an existing router.
type HttpRouter interface { //nolint:revive,staticcheck // exported name used by services
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
	ServeHTTP(writer http.ResponseWriter, request *http.Request)
}

// HealthCheckable is anything that can report its own health, such as a
// resource.
type HealthCheckable interface {
	OK(ctx context.Context) error
}

// Checkable is a single health check.
type Checkable func(context.Context) error

// Config configures a Server.
//
// Readiness fails while more than ReadyCount consecutive check rounds have
// failed, and recovers by itself when the checks pass again. Liveness does
// not depend on the checks, except as a safety net: it fails when the checks
// have kept failing for longer than StuckAfter (an instance whose
// dependencies came back but which did not recover gets restarted), or when
// the check loop itself has stopped running.
type Config struct {
	Port          int           `default:"8080"`
	ProbeInterval time.Duration `default:"5s"`
	ProbeTimeout  time.Duration `default:"5s"`
	ReadyCount    int
	StuckAfter    time.Duration `default:"5m"`
	Ready         bool          `default:"true"`
	Alive         bool          `default:"true"`
}

// Server answers liveness (/health/alive) and readiness (/health/ready)
// probes from the results of its checks, which run in the background every
// ProbeInterval.
type Server struct {
	conf *Config

	hooksLock sync.RWMutex
	hooks     []*hook

	ready, alive, stopped atomic.Bool

	stateLock    sync.Mutex
	checked      bool      // at least one round finished
	consecutive  int       // failed rounds in a row
	failingSince time.Time // start of the current failing streak
	lastRound    time.Time // when the last round finished

	httpRouter HttpRouter
	done       chan struct{}
	closeOnce  sync.Once
	now        func() time.Time
}

type hook struct {
	check   Checkable
	running atomic.Bool
}

// NewWithDefaultEnvPrefix returns a Server configured from HEALTHCHECK_*
// environment variables. See New.
func NewWithDefaultEnvPrefix(httpRouter HttpRouter, hc ...HealthCheckable) *Server {
	return New("healthcheck", httpRouter, hc...)
}

// New returns a Server configured from environment variables with the given
// prefix, and starts its check loop. It panics if the variables cannot be
// parsed. A nil httpRouter gets a new *http.ServeMux.
func New(envPrefix string, httpRouter HttpRouter, hc ...HealthCheckable) *Server {
	conf := &Config{}
	envconfig.MustProcess(envPrefix, conf)
	return NewWithConfig(conf, httpRouter, hc...)
}

// NewWithConfig returns a Server with the given configuration and starts its
// check loop.
func NewWithConfig(conf *Config, httpRouter HttpRouter, hc ...HealthCheckable) *Server {
	return newServer(conf, httpRouter, time.Now, hc...)
}

func newServer(conf *Config, httpRouter HttpRouter, now func() time.Time, hc ...HealthCheckable) *Server {
	s := &Server{conf: conf, httpRouter: httpRouter, done: make(chan struct{}), now: now}
	for _, h := range hc {
		s.hooks = append(s.hooks, &hook{check: h.OK})
	}
	s.ready.Store(conf.Ready)
	s.alive.Store(conf.Alive)
	if s.httpRouter == nil {
		s.httpRouter = http.NewServeMux()
	}
	s.httpRouter.HandleFunc("/health/ready", s.readinessHandler)
	s.httpRouter.HandleFunc("/health/alive", s.livenessHandler)
	go s.loop()
	return s
}

// AddHooks registers more checks.
func (s *Server) AddHooks(hooks ...Checkable) {
	s.hooksLock.Lock()
	defer s.hooksLock.Unlock()
	for _, c := range hooks {
		s.hooks = append(s.hooks, &hook{check: c})
	}
}

// Ready sets whether the service is ready to serve.
func (s *Server) Ready(serviceReady bool) { s.ready.Store(serviceReady) }

// Alive sets whether the service is alive.
func (s *Server) Alive(serviceAlive bool) { s.alive.Store(serviceAlive) }

// Stop makes the readiness probe fail for good, so that the pod stops
// receiving traffic while it drains. Liveness keeps passing: a failing
// liveness probe would restart the container in the middle of the drain.
func (s *Server) Stop() { s.stopped.Store(true) }

// Close stops the background check loop.
func (s *Server) Close() { s.closeOnce.Do(func() { close(s.done) }) }

// Start serves the probes and blocks until the HTTP server fails.
func (s *Server) Start() error {
	var lc net.ListenConfig
	lis, err := lc.Listen(context.Background(), "tcp", fmt.Sprintf(":%d", s.conf.Port))
	if err != nil {
		return err
	}
	srv := &http.Server{Handler: s.httpRouter, ReadHeaderTimeout: 5 * time.Second}
	return srv.Serve(lis)
}

// Router returns the router the probes are served on.
func (s *Server) Router() HttpRouter { return s.httpRouter }

func (s *Server) loop() {
	ticker := time.NewTicker(s.conf.ProbeInterval)
	defer ticker.Stop()
	for {
		s.round()
		select {
		case <-s.done:
			return
		case <-ticker.C:
		}
	}
}

// round runs every check once, each bounded by ProbeTimeout, and records the
// result. A check still running from an earlier round counts as failed and
// is not started again.
func (s *Server) round() {
	s.hooksLock.RLock()
	hooks := append([]*hook(nil), s.hooks...)
	s.hooksLock.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), s.conf.ProbeTimeout)
	defer cancel()
	err := runHooks(ctx, hooks)

	s.stateLock.Lock()
	defer s.stateLock.Unlock()
	now := s.now()
	s.checked, s.lastRound = true, now
	if err == nil {
		if s.consecutive > 0 {
			log.Info().Int("failed_rounds", s.consecutive).Msg("health checks pass again")
		}
		s.consecutive, s.failingSince = 0, time.Time{}
		return
	}
	if s.consecutive == 0 {
		s.failingSince = now
		log.Error().Err(err).Msg("health check failed")
	} else {
		log.Debug().Err(err).Int("failed_rounds", s.consecutive+1).Msg("health check still failing")
	}
	s.consecutive++
}

func runHooks(ctx context.Context, hooks []*hook) error {
	errs := make(chan error, len(hooks))
	started := 0
	var busy error
	for _, h := range hooks {
		if !h.running.CompareAndSwap(false, true) {
			busy = errors.New("health check from an earlier round has not returned")
			continue
		}
		started++
		go func() {
			defer h.running.Store(false)
			errs <- h.check(ctx)
		}()
	}
	if busy != nil {
		return busy
	}
	for range started {
		select {
		case err := <-errs:
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return fmt.Errorf("health check timed out: %w", ctx.Err())
		}
	}
	return nil
}

func (s *Server) readinessHandler(w http.ResponseWriter, _ *http.Request) {
	s.stateLock.Lock()
	ok := s.checked && s.consecutive <= s.conf.ReadyCount
	s.stateLock.Unlock()
	writeStatus(w, ok && s.ready.Load() && !s.stopped.Load())
}

func (s *Server) livenessHandler(w http.ResponseWriter, _ *http.Request) {
	writeStatus(w, s.alive.Load() && !s.stuck())
}

// stuck reports whether the checks have failed for longer than StuckAfter,
// or the check loop has not finished a round for three intervals plus a
// timeout.
func (s *Server) stuck() bool {
	s.stateLock.Lock()
	defer s.stateLock.Unlock()
	now := s.now()
	if !s.failingSince.IsZero() && now.Sub(s.failingSince) > s.conf.StuckAfter {
		return true
	}
	return s.checked && now.Sub(s.lastRound) > 3*s.conf.ProbeInterval+s.conf.ProbeTimeout
}

func writeStatus(w http.ResponseWriter, ok bool) {
	if ok {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
}

// GoCheck runs the checks in parallel and returns the first error, or ctx's
// error if ctx ends first.
func GoCheck(ctx context.Context, toCheck ...Checkable) error {
	hooks := make([]*hook, len(toCheck))
	for i, c := range toCheck {
		hooks[i] = &hook{check: c}
	}
	return runHooks(ctx, hooks)
}
