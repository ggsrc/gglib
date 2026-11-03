package otel

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Option configures Setup.
type Option func(*config)

type config struct {
	serviceName, serviceVersion, environment string
	attributes                               []attribute.KeyValue

	traces, metrics, logs, runtimeMetrics bool

	sampler        sdktrace.Sampler
	propagator     propagation.TextMapPropagator
	spanProcessors []sdktrace.SpanProcessor
	metricReaders  []sdkmetric.Reader
	logProcessors  []sdklog.Processor
}

func newConfig(opts []Option) *config {
	cfg := &config{
		traces:  true,
		metrics: DefaultMetrics,
		logs:    DefaultLogs,
		propagator: propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{}, propagation.Baggage{}),
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

// WithServiceName sets service.name, overriding OTEL_SERVICE_NAME.
func WithServiceName(name string) Option {
	return func(c *config) { c.serviceName = name }
}

// WithServiceVersion sets service.version.
func WithServiceVersion(version string) Option {
	return func(c *config) { c.serviceVersion = version }
}

// WithEnvironment sets deployment.environment.name, overriding ENV.
func WithEnvironment(env string) Option {
	return func(c *config) { c.environment = env }
}

// WithAttributes adds resource attributes; they override everything else.
func WithAttributes(attrs ...attribute.KeyValue) Option {
	return func(c *config) { c.attributes = append(c.attributes, attrs...) }
}

// WithTraces turns trace export on or off (default on). The tracer provider
// is installed either way.
func WithTraces(on bool) Option {
	return func(c *config) { c.traces = on }
}

// WithMetrics turns OTLP metric export on or off (default DefaultMetrics).
func WithMetrics(on bool) Option {
	return func(c *config) { c.metrics = on }
}

// WithRuntimeMetrics records Go runtime metrics through the meter provider.
// It has an effect only when a meter provider is installed.
func WithRuntimeMetrics(on bool) Option {
	return func(c *config) { c.runtimeMetrics = on }
}

// WithLogs turns OTLP log export on or off (default DefaultLogs). Logs reach
// the logger provider through the zerolog OTLP bridge.
func WithLogs(on bool) Option {
	return func(c *config) { c.logs = on }
}

// WithSampler sets the trace sampler. Without it, the SDK reads
// OTEL_TRACES_SAMPLER and OTEL_TRACES_SAMPLER_ARG (default: parent-based,
// always on).
func WithSampler(s sdktrace.Sampler) Option {
	return func(c *config) { c.sampler = s }
}

// WithPropagator sets the propagator (default: W3C trace context and
// baggage).
func WithPropagator(p propagation.TextMapPropagator) Option {
	return func(c *config) { c.propagator = p }
}

// WithSpanProcessor adds a span processor, for example to record spans in
// tests or to export somewhere else.
func WithSpanProcessor(p sdktrace.SpanProcessor) Option {
	return func(c *config) { c.spanProcessors = append(c.spanProcessors, p) }
}

// WithMetricReader adds a metric reader and installs a meter provider even
// when OTLP metric export is off.
func WithMetricReader(r sdkmetric.Reader) Option {
	return func(c *config) { c.metricReaders = append(c.metricReaders, r) }
}

// WithLogProcessor adds a log processor and installs a logger provider even
// when OTLP log export is off.
func WithLogProcessor(p sdklog.Processor) Option {
	return func(c *config) { c.logProcessors = append(c.logProcessors, p) }
}
