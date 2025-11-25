package app

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/uptrace/uptrace-go/uptrace"

	"github.com/ggsrc/gglib/grpc"
	"github.com/ggsrc/gglib/health"
	"github.com/ggsrc/gglib/metric"
	"github.com/ggsrc/gglib/resource"
	"github.com/ggsrc/gglib/zerolog"
	"github.com/ggsrc/gglib/zerolog/log"
)

var DefaultResourceShutDownTimeout = 30 * time.Second

type App struct {
	options         *Options
	grpcServer      *grpc.Server
	healthChecker   *health.Server
	metricServer    *metric.Server
	resourceManager resource.ResourceManager
}

func NewApp(opts ...Option) *App {
	options := &Options{}
	for _, opt := range opts {
		opt(options)
	}

	if options.GRPCServer == nil {
		panic("GRPCServer is required")
	}
	if options.ResourceManager == nil {
		panic("ResourceManager is required")
	}

	metricServer := metric.NewWithDefaultEnvPrefix()
	healthChecker := health.InitHealthCheck(options.ResourceManager, metricServer)
	return &App{
		options:         options,
		grpcServer:      options.GRPCServer,
		healthChecker:   healthChecker,
		metricServer:    metricServer,
		resourceManager: options.ResourceManager,
	}
}

func (a *App) Start(ctx context.Context) {
	if a.options.OTELEnabled {
		uptrace.ConfigureOpentelemetry(
			uptrace.WithDeploymentEnvironment(a.options.Env),
			uptrace.WithServiceVersion(a.options.ServerVersion),
			uptrace.WithDSN(a.options.OTELDSN),
		)
		zerolog.InitLogger(
			a.options.Debug,
			zerolog.WithOTLP(),
			zerolog.WithBatchSize(a.options.OTELBatchSize),
		)
	} else {
		zerolog.InitLogger(a.options.Debug)
	}
	var wg sync.WaitGroup
	grpcErrCh, healthErrCh := make(chan error, 1),
		make(chan error, 1)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if a.grpcServer != nil {
			log.Warn().Msg("GRPC server start")
			grpcErrCh <- a.grpcServer.Start()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if a.healthChecker != nil {
			log.Warn().Msg("Health checker start")
			healthErrCh <- a.healthChecker.Start()
		}
	}()

	if err := a.resourceManager.Start(ctx); err != nil {
		log.Error().Err(err).Msg("resource manager error; shutting down")
		_ = a.Stop(ctx)
		return
	}

	if err := a.metricServer.Start(ctx); err != nil {
		log.Error().Err(err).Msg("metric server error; shutting down")
		_ = a.Stop(ctx)
		return
	}

	// Monitor system signal like SIGINT and SIGTERM
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	select {
	case osSig := <-sig:
		log.Error().Msgf("received signal %s; shutting down", osSig)
		_ = a.Stop(ctx)
	case err := <-healthErrCh:
		log.Error().Err(err).Msg("health server error; shutting down")
		_ = a.Stop(ctx)
	}
}

func (a *App) Stop(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, DefaultResourceShutDownTimeout)
	defer cancel()

	// shutdown services concurrently and wait for all to finish, e.g. grpc server, cronjob, etc.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// shutdown grpc server
		if a.grpcServer != nil {
			if err := a.grpcServer.Shutdown(ctx); err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("failed to shutdown grpc server")
			}
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		if a.resourceManager != nil {
			if err := a.resourceManager.Stop(ctx); err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("failed to shutdown cronjob")
			}
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		if a.healthChecker != nil {
			a.healthChecker.Stop()
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		if a.metricServer != nil {
			if err := a.metricServer.Stop(ctx); err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("failed to shutdown metricer")
			}
		}
	}()

	wg.Wait()
	return nil
}
