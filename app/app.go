// Package app runs a service process: logging and telemetry, the metrics and
// health servers, the shared resources and, optionally, a gRPC server.
//
// A service calls New first, so that the health probes are answered while it
// initializes, then Init with the resources it needs to build its handlers,
// then Run:
//
//	a, err := app.New(ctx, app.WithServerName("my-service"))
//	if err != nil { ... }
//	rm := resource.NewResourceManager([]resource.Resource{db, cache})
//	if err := a.Init(ctx, rm); err != nil { ... }
//	srv := buildServer(db, cache) // registers the gRPC services
//	if err := a.Run(ctx, srv, rm); err != nil { ... }
//
// Until Run has started everything, the pod is alive and not ready: it gets
// no traffic and is not restarted, however long the dependencies take to come
// up. On SIGINT or SIGTERM, Run stops the process in order: readiness fails,
// in-flight RPCs drain, resources stop (last registered first), then the
// servers, and telemetry is flushed last.
package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"github.com/ggsrc/gglib/env"
	"github.com/ggsrc/gglib/grpc"
	"github.com/ggsrc/gglib/health"
	"github.com/ggsrc/gglib/metric"
	"github.com/ggsrc/gglib/otel"
	"github.com/ggsrc/gglib/resource"
	ggzerolog "github.com/ggsrc/gglib/zerolog"
	"github.com/ggsrc/gglib/zerolog/log"
)

// DefaultResourceShutDownTimeout is the default for WithShutdownTimeout: how
// long Stop waits for in-flight RPCs to finish and for resources to stop.
var DefaultResourceShutDownTimeout = 30 * time.Second

// The wait between two attempts in Init starts at initRetryStart and doubles
// up to initRetryCap.
const (
	initRetryStart = time.Second
	initRetryCap   = 30 * time.Second
)

// telemetryFlushTimeout bounds the final telemetry flush in Stop, which runs
// after the shutdown timeout may already have been used up by the drain.
const telemetryFlushTimeout = 5 * time.Second

// App is a running service process. Create it with New.
type App struct {
	conf      *config
	health    *health.Server
	metrics   *metric.Server
	telemetry otel.ShutdownFunc // nil when telemetry was set up elsewhere
	healthErr chan error

	mu        sync.Mutex
	grpc      *grpc.Server
	resources []resource.ResourceManager

	stopOnce sync.Once
	stopErr  error
}

// New sets up logging (gglib/zerolog.Setup) and telemetry (gglib/otel.Setup),
// and starts the metrics and health servers. The health server reports the
// process alive and not ready until Run has started everything.
//
// If gglib/otel.Setup was already called in this process, New leaves
// telemetry, and flushing it, to the caller. Any other setup failure, or a
// metrics or health port that cannot be bound, is returned.
func New(ctx context.Context, opts ...Option) (*App, error) {
	conf := newConfig(opts)

	logOpts := []ggzerolog.Option{}
	if conf.debug {
		logOpts = append(logOpts, ggzerolog.WithLevel(zerolog.DebugLevel))
	}
	ggzerolog.Setup(append(logOpts, conf.logging...)...)

	a := &App{conf: conf, healthErr: make(chan error, 1)}
	shutdown, err := otel.Setup(ctx, telemetryOptions(conf)...)
	switch {
	case errors.Is(err, otel.ErrAlreadySetup):
		log.Ctx(ctx).Warn().Msg("app: telemetry was set up before New; the caller flushes it")
	case err != nil:
		return nil, fmt.Errorf("app: telemetry: %w", err)
	default:
		a.telemetry = shutdown
	}

	a.metrics = metric.NewWithDefaultEnvPrefix()
	if err := a.metrics.Start(ctx); err != nil {
		return nil, errors.Join(fmt.Errorf("app: %w", err), a.flushTelemetry(ctx))
	}
	a.health = health.InitHealthCheck(a.metrics) //nolint:contextcheck // the check loop outlives New's context
	a.health.Ready(false)
	lis, err := a.health.Listen(ctx)
	if err != nil {
		a.health.Close()
		return nil, errors.Join(fmt.Errorf("app: %w", err), a.metrics.Stop(ctx), a.flushTelemetry(ctx))
	}
	go func() {
		if err := a.health.Serve(lis); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.healthErr <- err
		}
	}()
	log.Ctx(ctx).Info().Str("env", env.Env()).Str("service", env.ServiceName()).Msg("app: starting")
	return a, nil
}

// telemetryOptions passes the server name and version as fallbacks: the
// deployment's SERVICE_NAME / OTEL_SERVICE_NAME and SERVICE_VERSION / GIT_SHA
// win, so that service names in Uptrace do not change.
func telemetryOptions(conf *config) []otel.Option {
	var opts []otel.Option
	if conf.serverName != "" && env.ServiceName() == env.UnknownServiceName {
		opts = append(opts, otel.WithServiceName(conf.serverName))
	}
	if conf.serverVersion != "" && env.ServiceVersion() == "" {
		opts = append(opts, otel.WithServiceVersion(conf.serverVersion))
	}
	return append(opts, conf.telemetry...)
}

// Init initializes the resources of rm. While a resource fails to initialize,
// for example because the database is unreachable, Init logs the error and
// tries again, waiting one second and then twice as long each time, up to 30
// seconds, until it succeeds or ctx is done. Resources already initialized
// are not initialized again (see resource.ResourceManager.Init), so each
// resource's Init must be safe to retry after a failure, as the gglib
// resources are.
//
// Meanwhile the process stays alive and not ready: Kubernetes neither routes
// traffic to it nor restarts it. Init returns ctx's error, joined with the
// last failure, only when ctx is done.
func (a *App) Init(ctx context.Context, rm resource.ResourceManager) error {
	return a.initWithRetry(ctx, rm, initRetryStart, initRetryCap)
}

func (a *App) initWithRetry(ctx context.Context, rm resource.ResourceManager, wait, maxWait time.Duration) error {
	for attempt := 1; ; attempt++ {
		err := rm.Init(ctx)
		if err == nil {
			if attempt > 1 {
				log.Ctx(ctx).Info().Int("attempts", attempt).Msg("app: resources initialized")
			}
			return nil
		}
		log.Ctx(ctx).Error().Err(err).Int("attempt", attempt).Dur("retry_in", wait).
			Msg("app: resources not initialized; retrying")
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.Join(fmt.Errorf("app: init: %w", err), ctx.Err())
		case <-timer.C:
		}
		wait = min(2*wait, maxWait)
	}
}

// Run starts the resources of each rm, in order, adds their checks to the
// health probes, starts srv if it is not nil (a worker process can run
// without gRPC), and marks the process ready. It then blocks until SIGINT or
// SIGTERM, until ctx is done, or until the gRPC or health server fails, and
// stops everything (see Stop). It returns the failure that ended it, if any,
// joined with the errors from stopping.
func (a *App) Run(ctx context.Context, srv *grpc.Server, rms ...resource.ResourceManager) error {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)

	stopCtx := context.WithoutCancel(ctx)
	for _, rm := range rms {
		a.mu.Lock()
		a.resources = append(a.resources, rm)
		a.mu.Unlock()
		if err := rm.Start(ctx); err != nil {
			return errors.Join(fmt.Errorf("app: start resources: %w", err), a.Stop(stopCtx))
		}
		a.health.AddHooks(rm.OK)
	}

	grpcErr := make(chan error, 1)
	if srv != nil {
		lis, err := srv.Listen(ctx)
		if err != nil {
			return errors.Join(fmt.Errorf("app: %w", err), a.Stop(stopCtx))
		}
		a.mu.Lock()
		a.grpc = srv
		a.mu.Unlock()
		go func() { grpcErr <- srv.Serve(lis) }()
	}

	a.health.Ready(true)
	log.Ctx(ctx).Info().Bool("grpc", srv != nil).Msg("app: ready")

	var cause error
	select {
	case s := <-sig:
		log.Ctx(ctx).Info().Str("signal", s.String()).Msg("app: shutting down")
	case <-ctx.Done():
		log.Ctx(ctx).Info().Msg("app: context done; shutting down")
	case err := <-grpcErr:
		if err == nil {
			err = errors.New("stopped")
		}
		cause = fmt.Errorf("app: grpc server: %w", err)
	case err := <-a.healthErr:
		cause = fmt.Errorf("app: health server: %w", err)
	}
	if cause != nil {
		log.Ctx(ctx).Error().Err(cause).Msg("app: shutting down")
	}
	return errors.Join(cause, a.Stop(stopCtx))
}

// Stop shuts the process down, within the shutdown timeout
// (WithShutdownTimeout): readiness fails, in-flight RPCs drain (at the
// deadline the gRPC server is stopped), the resources given to Run stop
// (last registered first), then the metrics and health servers stop, and
// telemetry is flushed last, so that the shutdown's own logs and spans are
// sent. Liveness keeps passing until the end, so the pod is not restarted
// during a long drain. Only the first call acts; later calls return its
// result.
func (a *App) Stop(ctx context.Context) error {
	a.stopOnce.Do(func() { a.stopErr = a.stop(ctx) })
	return a.stopErr
}

func (a *App) stop(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, a.conf.shutdownTimeout)
	defer cancel()

	a.health.Stop()
	a.mu.Lock()
	srv, rms := a.grpc, a.resources
	a.mu.Unlock()

	var errs []error
	if srv != nil {
		// Shutdown returns after the drain, or force-stops at the deadline.
		// Only then release the dependencies the handlers use.
		if err := srv.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("app: grpc shutdown: %w", err))
		}
	}
	for _, rm := range slices.Backward(rms) {
		if err := rm.Stop(ctx); err != nil {
			errs = append(errs, fmt.Errorf("app: stop resources: %w", err))
		}
	}
	if err := a.metrics.Stop(ctx); err != nil {
		errs = append(errs, fmt.Errorf("app: metrics shutdown: %w", err))
	}
	if err := a.health.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("app: health shutdown: %w", err))
	}
	err := errors.Join(errs...)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("app: stopped with errors")
	} else {
		log.Ctx(ctx).Info().Msg("app: stopped")
	}
	return errors.Join(err, a.flushTelemetry(ctx))
}

// flushTelemetry flushes and stops telemetry, with its own short deadline.
func (a *App) flushTelemetry(ctx context.Context) error {
	if a.telemetry == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), telemetryFlushTimeout)
	defer cancel()
	if err := a.telemetry(ctx); err != nil {
		return fmt.Errorf("app: telemetry flush: %w", err)
	}
	return nil
}
