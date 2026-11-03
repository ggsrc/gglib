// Package otel sets up OpenTelemetry for a process with one call to Setup.
//
// Setup always installs an SDK tracer provider, so spans get real trace ids
// even when nothing is exported; logs can then carry a trace_id. Export goes
// over OTLP/HTTP to the endpoint in the standard OTEL_EXPORTER_OTLP_*
// variables; with no endpoint configured, nothing is exported. Traces are
// exported by default; metrics and logs only when turned on, because
// Prometheus (gglib/metric) and stdout logs are the primary pipelines.
//
// Every service gets the same resource attributes: service.name (from
// OTEL_SERVICE_NAME or SERVICE_NAME), service.version (SERVICE_VERSION,
// GIT_SHA or the VCS revision), deployment.environment.name (ENV), the host,
// and k8s.pod.name / k8s.namespace.name when POD_NAME / POD_NAMESPACE are
// set. OTEL_RESOURCE_ATTRIBUTES and options override them.
//
// Call Setup once, early in main, before building gRPC servers, clients or
// database pools (they capture the global providers when created), and call
// the returned shutdown last, so that buffered data is sent.
package otel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Defaults for the signals other than traces. Metrics are off because
// dashboards, alerts and autoscaling read Prometheus, not OTel metrics
// (decision G10). Logs are on, so that a trace in Uptrace shows its log
// records (decision G11); stdout logs still reach Loki as before.
const (
	DefaultMetrics = false
	DefaultLogs    = true
)

// ShutdownFunc flushes and stops everything Setup installed. Only its first
// call does anything.
type ShutdownFunc func(context.Context) error

var setupCalled atomic.Bool

// ErrAlreadySetup is returned by Setup when it has already been called in
// this process.
var ErrAlreadySetup = errors.New("otel: Setup already called")

// Setup installs the global tracer provider and propagator, and the meter and
// logger providers when they are turned on, and returns the function that
// flushes and stops them. It returns an error if called twice in a process.
func Setup(ctx context.Context, opts ...Option) (ShutdownFunc, error) {
	if !setupCalled.CompareAndSwap(false, true) {
		return nil, ErrAlreadySetup
	}
	cfg := newConfig(opts)

	res, err := newResource(ctx, cfg)
	if err != nil {
		return nil, err
	}

	var shutdowns []func(context.Context) error
	fail := func(err error) (ShutdownFunc, error) {
		for _, shutdown := range slices.Backward(shutdowns) {
			_ = shutdown(ctx)
		}
		return nil, err
	}

	// Traces.
	tpOpts := []sdktrace.TracerProviderOption{sdktrace.WithResource(res)}
	if cfg.sampler != nil {
		tpOpts = append(tpOpts, sdktrace.WithSampler(cfg.sampler))
	}
	if cfg.traces && exporting("TRACES") {
		exp, err := otlptracehttp.New(ctx)
		if err != nil {
			return fail(fmt.Errorf("otel: trace exporter: %w", err))
		}
		tpOpts = append(tpOpts, sdktrace.WithBatcher(exp))
	}
	for _, sp := range cfg.spanProcessors {
		tpOpts = append(tpOpts, sdktrace.WithSpanProcessor(sp))
	}
	tp := sdktrace.NewTracerProvider(tpOpts...)
	shutdowns = append(shutdowns, tp.Shutdown)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(cfg.propagator)

	// Metrics.
	readers := append([]sdkmetric.Reader(nil), cfg.metricReaders...)
	if cfg.metrics && exporting("METRICS") {
		exp, err := otlpmetrichttp.New(ctx)
		if err != nil {
			return fail(fmt.Errorf("otel: metric exporter: %w", err))
		}
		readers = append(readers, sdkmetric.NewPeriodicReader(exp))
	}
	if len(readers) > 0 {
		mpOpts := []sdkmetric.Option{sdkmetric.WithResource(res)}
		for _, r := range readers {
			mpOpts = append(mpOpts, sdkmetric.WithReader(r))
		}
		mp := sdkmetric.NewMeterProvider(mpOpts...)
		shutdowns = append(shutdowns, mp.Shutdown)
		otel.SetMeterProvider(mp)
		if cfg.runtimeMetrics {
			if err := runtime.Start(runtime.WithMeterProvider(mp)); err != nil {
				return fail(fmt.Errorf("otel: runtime metrics: %w", err))
			}
		}
	}

	// Logs.
	processors := append([]sdklog.Processor(nil), cfg.logProcessors...)
	if cfg.logs && exporting("LOGS") {
		exp, err := otlploghttp.New(ctx)
		if err != nil {
			return fail(fmt.Errorf("otel: log exporter: %w", err))
		}
		processors = append(processors, sdklog.NewBatchProcessor(exp))
	}
	if len(processors) > 0 {
		lpOpts := []sdklog.LoggerProviderOption{sdklog.WithResource(res)}
		for _, p := range processors {
			lpOpts = append(lpOpts, sdklog.WithProcessor(p))
		}
		lp := sdklog.NewLoggerProvider(lpOpts...)
		shutdowns = append(shutdowns, lp.Shutdown)
		otel.SetLoggerProvider(lp)
	}

	return newShutdown(shutdowns), nil
}

// newShutdown stops the providers in the order they were installed: traces,
// then metrics, then logs, so that log records written while flushing spans
// are still sent.
func newShutdown(fns []func(context.Context) error) ShutdownFunc {
	var once sync.Once
	return func(ctx context.Context) error {
		var errs []error
		once.Do(func() {
			for _, fn := range fns {
				if err := fn(ctx); err != nil {
					errs = append(errs, err)
				}
			}
		})
		return errors.Join(errs...)
	}
}

// exporting reports whether signal ("TRACES", "METRICS" or "LOGS") has an
// OTLP endpoint and the SDK is not disabled with OTEL_SDK_DISABLED.
func exporting(signal string) bool {
	if disabled, err := strconv.ParseBool(os.Getenv("OTEL_SDK_DISABLED")); err == nil && disabled {
		return false
	}
	return os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" ||
		os.Getenv("OTEL_EXPORTER_OTLP_"+signal+"_ENDPOINT") != ""
}
