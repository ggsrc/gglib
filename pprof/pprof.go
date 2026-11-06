package pprof

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"net/http"
	rpprof "runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// DefaultAddr is the address the server listens on unless configured
// otherwise.
const DefaultAddr = ":6060"

// DefaultCPUProfileSeconds is both the default and the default maximum length
// of a CPU profile.
const DefaultCPUProfileSeconds = 10

// Config configures a Server.
type Config struct {
	// Enabled turns the server on. A disabled server's Start does nothing.
	Enabled bool `default:"false"`
	// Addr is the listen address; port 0 picks a free port.
	Addr string `default:":6060"`
	// Token, when set, must be sent as "Authorization: Bearer <token>" or
	// "X-Debug-Token: <token>".
	Token string `default:""`
	// CPUProfileSeconds is the CPU profile length when the request does not
	// give ?seconds=.
	CPUProfileSeconds int `default:"10" split_words:"true"`
	// CPUProfileMaxSeconds is the longest CPU profile a request may ask for.
	CPUProfileMaxSeconds int `default:"10" split_words:"true"`
}

// Server is the profiling HTTP server.
type Server struct {
	conf      Config
	cpuActive atomic.Bool

	mu         sync.Mutex
	httpServer *http.Server
	addr       net.Addr
	serveErr   error
}

// NewWithDefaultEnvPrefix returns a Server configured from DEBUG_PPROF_*
// environment variables (DEBUG_PPROF_ENABLED, DEBUG_PPROF_ADDR,
// DEBUG_PPROF_TOKEN, DEBUG_PPROF_CPU_PROFILE_SECONDS,
// DEBUG_PPROF_CPU_PROFILE_MAX_SECONDS).
func NewWithDefaultEnvPrefix() *Server {
	return New("debug_pprof")
}

// New returns a Server configured from environment variables with the given
// prefix. It panics if they cannot be parsed.
func New(envPrefix string) *Server {
	var conf Config
	envconfig.MustProcess(envPrefix, &conf)
	return newServer(conf)
}

// NewWithOptions returns an enabled Server configured by opts, with the same
// defaults as the environment otherwise.
func NewWithOptions(opts ...Option) *Server {
	conf := Config{
		Enabled:              true,
		Addr:                 DefaultAddr,
		CPUProfileSeconds:    DefaultCPUProfileSeconds,
		CPUProfileMaxSeconds: DefaultCPUProfileSeconds,
	}
	for _, opt := range opts {
		opt(&conf)
	}
	return newServer(conf)
}

func newServer(conf Config) *Server {
	if conf.CPUProfileSeconds <= 0 {
		conf.CPUProfileSeconds = DefaultCPUProfileSeconds
	}
	if conf.CPUProfileMaxSeconds <= 0 {
		conf.CPUProfileMaxSeconds = DefaultCPUProfileSeconds
	}
	if conf.CPUProfileSeconds > conf.CPUProfileMaxSeconds {
		conf.CPUProfileSeconds = conf.CPUProfileMaxSeconds
	}
	return &Server{conf: conf}
}

// Enabled reports whether Start will serve.
func (s *Server) Enabled() bool {
	return s.conf.Enabled
}

// Handler returns the server's routes, for tests or for mounting them on
// another server. It applies the same method and token checks.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/health", s.guard(handleHealth))
	mux.HandleFunc("/debug/pprof/goroutine", s.guard(handleLookup("goroutine")))
	mux.HandleFunc("/debug/pprof/heap", s.guard(handleLookup("heap")))
	mux.HandleFunc("/debug/pprof/profile", s.guard(s.handleCPUProfile))
	return mux
}

// Start listens on the configured address and serves in a background
// goroutine. It does nothing when the server is disabled, and returns an
// error if the address cannot be bound or the server was already started.
func (s *Server) Start(ctx context.Context) error {
	if !s.conf.Enabled {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.httpServer != nil {
		return errors.New("pprof: server already started")
	}
	var lc net.ListenConfig
	lis, err := lc.Listen(ctx, "tcp", s.conf.Addr)
	if err != nil {
		return fmt.Errorf("pprof: listen: %w", err)
	}
	s.serve(lis)
	return nil
}

// serve serves on lis in a background goroutine. s.mu must be held.
func (s *Server) serve(lis net.Listener) {
	srv := &http.Server{
		Handler:           s.Handler(),
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

// Addr returns the address the server listens on, or nil before Start or when
// disabled.
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

// OK returns the error that stopped the server, if one has occurred.
func (s *Server) OK(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.serveErr
}

func (s *Server) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.conf.Token != "" &&
			!tokenEqual(bearerToken(r.Header.Get("Authorization")), s.conf.Token) &&
			!tokenEqual(r.Header.Get("X-Debug-Token"), s.conf.Token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok\n"))
}

// handleLookup writes a named runtime profile: binary by default, text with
// ?debug=1 or higher.
func handleLookup(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		debug := parseDebugParam(r)
		if debug > 0 {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		} else {
			w.Header().Set("Content-Type", "application/octet-stream")
		}
		if err := rpprof.Lookup(name).WriteTo(w, debug); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func (s *Server) handleCPUProfile(w http.ResponseWriter, r *http.Request) {
	if !s.cpuActive.CompareAndSwap(false, true) {
		http.Error(w, "cpu profile already running", http.StatusTooManyRequests)
		return
	}
	defer s.cpuActive.Store(false)

	seconds := s.conf.CPUProfileSeconds
	if raw := r.URL.Query().Get("seconds"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			http.Error(w, "seconds must be a positive integer", http.StatusBadRequest)
			return
		}
		seconds = n
	}
	if seconds > s.conf.CPUProfileMaxSeconds {
		http.Error(w, fmt.Sprintf("seconds exceeds max %d", s.conf.CPUProfileMaxSeconds), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	if err := rpprof.StartCPUProfile(w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rpprof.StopCPUProfile()

	timer := time.NewTimer(time.Duration(seconds) * time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-r.Context().Done():
	}
}

func parseDebugParam(r *http.Request) int {
	n, err := strconv.Atoi(r.URL.Query().Get("debug"))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func bearerToken(header string) string {
	fields := strings.Fields(header)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
		return ""
	}
	return fields[1]
}

func tokenEqual(got, want string) bool {
	if got == "" || want == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}
