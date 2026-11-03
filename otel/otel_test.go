package otel

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"
)

// receiver is a fake OTLP/HTTP collector.
type receiver struct {
	mu     sync.Mutex
	bodies map[string][][]byte
}

func startReceiver(t *testing.T) *receiver {
	t.Helper()
	r := &receiver{bodies: map[string][][]byte{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		b, _ := io.ReadAll(req.Body)
		r.mu.Lock()
		r.bodies[req.URL.Path] = append(r.bodies[req.URL.Path], b)
		r.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", srv.URL)
	return r
}

func (r *receiver) paths() map[string]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[string]int{}
	for p, b := range r.bodies {
		out[p] = len(b)
	}
	return out
}

func (r *receiver) traces(t *testing.T) *collectortrace.ExportTraceServiceRequest {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	require.NotEmpty(t, r.bodies["/v1/traces"])
	var req collectortrace.ExportTraceServiceRequest
	require.NoError(t, proto.Unmarshal(r.bodies["/v1/traces"][0], &req))
	return &req
}

// cleanEnv clears the variables Setup reads and allows a new Setup.
func cleanEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "OTEL_EXPORTER_OTLP_LOGS_ENDPOINT",
		"OTEL_SDK_DISABLED", "OTEL_SERVICE_NAME", "OTEL_RESOURCE_ATTRIBUTES",
		"SERVICE_NAME", "SERVICE_VERSION", "GIT_SHA", "ENV", "OTEL_DEPLOYMENT_ENVIRONMENT",
		"POD_NAME", "POD_NAMESPACE", "OTEL_TRACES_SAMPLER",
	} {
		t.Setenv(k, "")
		require.NoError(t, os.Unsetenv(k))
	}
	allowSetupAgain()
	t.Cleanup(allowSetupAgain)
}

func setup(t *testing.T, opts ...Option) ShutdownFunc {
	t.Helper()
	shutdown, err := Setup(t.Context(), opts...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = shutdown(context.Background()) })
	return shutdown
}

func resourceAttrs(req *collectortrace.ExportTraceServiceRequest) map[string]string {
	out := map[string]string{}
	for _, kv := range req.GetResourceSpans()[0].GetResource().GetAttributes() {
		out[kv.GetKey()] = kv.GetValue().GetStringValue()
	}
	return out
}

func TestTracesExportedWithStandardResource(t *testing.T) {
	cleanEnv(t)
	rcv := startReceiver(t)
	t.Setenv("OTEL_SERVICE_NAME", "svc")
	t.Setenv("ENV", "stg")
	t.Setenv("SERVICE_VERSION", "abc123")
	t.Setenv("POD_NAME", "svc-7d9")
	t.Setenv("POD_NAMESPACE", "app")

	shutdown := setup(t)
	_, span := otel.Tracer("test").Start(t.Context(), "work")
	span.End()
	require.NoError(t, shutdown(t.Context()))

	req := rcv.traces(t)
	attrs := resourceAttrs(req)
	assert.Equal(t, "svc", attrs["service.name"])
	assert.Equal(t, "abc123", attrs["service.version"])
	assert.Equal(t, "stg", attrs["deployment.environment.name"])
	assert.Equal(t, "stg", attrs["deployment.environment"], "kept for Uptrace")
	assert.Equal(t, "svc-7d9", attrs["k8s.pod.name"])
	assert.Equal(t, "app", attrs["k8s.namespace.name"])
	assert.NotEmpty(t, attrs["host.name"])
	assert.Equal(t, "work", req.GetResourceSpans()[0].GetScopeSpans()[0].GetSpans()[0].GetName())
}

func TestDefaultSignalsAreTracesAndLogs(t *testing.T) {
	cleanEnv(t)
	rcv := startReceiver(t)
	shutdown := setup(t)

	_, span := otel.Tracer("test").Start(t.Context(), "work")
	span.End()
	counter, err := otel.Meter("test").Int64Counter("c")
	require.NoError(t, err)
	counter.Add(t.Context(), 1)
	var rec otellog.Record
	rec.SetBody(attribute.StringValue("hello"))
	otel.GetLoggerProvider().Logger("test").Emit(t.Context(), rec)
	require.NoError(t, shutdown(t.Context()))

	assert.Equal(t, map[string]int{"/v1/traces": 1, "/v1/logs": 1}, rcv.paths(), "metrics are off by default (G10), logs on (G11)")
}

func TestLogsCanBeTurnedOff(t *testing.T) {
	cleanEnv(t)
	rcv := startReceiver(t)
	shutdown := setup(t, WithLogs(false))

	var rec otellog.Record
	rec.SetBody(attribute.StringValue("hello"))
	otel.GetLoggerProvider().Logger("test").Emit(t.Context(), rec)
	_, span := otel.Tracer("test").Start(t.Context(), "work")
	span.End()
	require.NoError(t, shutdown(t.Context()))

	assert.Equal(t, map[string]int{"/v1/traces": 1}, rcv.paths())
}

func TestMetricsAndLogsWhenOn(t *testing.T) {
	cleanEnv(t)
	rcv := startReceiver(t)
	shutdown := setup(t, WithMetrics(true), WithLogs(true), WithRuntimeMetrics(true), WithTraces(false))

	counter, err := otel.Meter("test").Int64Counter("c")
	require.NoError(t, err)
	counter.Add(t.Context(), 1)
	var rec otellog.Record
	rec.SetBody(attribute.StringValue("hello"))
	otel.GetLoggerProvider().Logger("test").Emit(t.Context(), rec)
	_, span := otel.Tracer("test").Start(t.Context(), "not exported")
	span.End()
	require.NoError(t, shutdown(t.Context()))

	paths := rcv.paths()
	assert.Positive(t, paths["/v1/metrics"])
	assert.Positive(t, paths["/v1/logs"])
	assert.Zero(t, paths["/v1/traces"], "WithTraces(false)")
}

// Without an endpoint nothing is exported, but spans still get real ids, so
// logs can carry a trace_id.
func TestNoEndpointStillRecordsIDs(t *testing.T) {
	cleanEnv(t)
	rec := tracetest.NewSpanRecorder()
	setup(t, WithSpanProcessor(rec))

	_, span := otel.Tracer("test").Start(t.Context(), "local")
	assert.True(t, span.SpanContext().IsValid())
	span.End()
	require.Len(t, rec.Ended(), 1)
	_, isSDK := otel.GetTracerProvider().(*sdktrace.TracerProvider)
	assert.True(t, isSDK)
}

func TestSDKDisabledExportsNothing(t *testing.T) {
	cleanEnv(t)
	rcv := startReceiver(t)
	t.Setenv("OTEL_SDK_DISABLED", "true")
	shutdown := setup(t, WithMetrics(true), WithLogs(true))
	_, span := otel.Tracer("test").Start(t.Context(), "x")
	span.End()
	require.NoError(t, shutdown(t.Context()))
	assert.Empty(t, rcv.paths())
}

func TestPerSignalEndpoint(t *testing.T) {
	cleanEnv(t)
	rcv := startReceiver(t)
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	require.NoError(t, os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", endpoint+"/v1/traces")

	shutdown := setup(t, WithMetrics(true))
	_, span := otel.Tracer("test").Start(t.Context(), "x")
	span.End()
	require.NoError(t, shutdown(t.Context()))
	assert.Equal(t, map[string]int{"/v1/traces": 1}, rcv.paths(), "metrics have no endpoint")
}

func TestSampler(t *testing.T) {
	cleanEnv(t)
	rec := tracetest.NewSpanRecorder()
	setup(t, WithSampler(sdktrace.NeverSample()), WithSpanProcessor(rec))
	_, span := otel.Tracer("test").Start(t.Context(), "dropped")
	assert.True(t, span.SpanContext().IsValid(), "ids exist even when not sampled")
	span.End()
	assert.Empty(t, rec.Ended())
}

func TestPropagator(t *testing.T) {
	cleanEnv(t)
	setup(t)
	assert.ElementsMatch(t, []string{"traceparent", "tracestate", "baggage"}, otel.GetTextMapPropagator().Fields())

	cleanEnv(t)
	setup(t, WithPropagator(propagation.TraceContext{}))
	assert.ElementsMatch(t, []string{"traceparent", "tracestate"}, otel.GetTextMapPropagator().Fields())
}

func TestLogProcessorOption(t *testing.T) {
	cleanEnv(t)
	proc := &countingProcessor{}
	shutdown := setup(t, WithLogProcessor(proc))
	var rec otellog.Record
	rec.SetBody(attribute.StringValue("x"))
	otel.GetLoggerProvider().Logger("test").Emit(t.Context(), rec)
	require.NoError(t, shutdown(t.Context()))
	assert.Equal(t, 1, proc.n)
}

type countingProcessor struct{ n int }

func (p *countingProcessor) OnEmit(context.Context, *sdklog.Record) error           { p.n++; return nil }
func (p *countingProcessor) Shutdown(context.Context) error                         { return nil }
func (p *countingProcessor) ForceFlush(context.Context) error                       { return nil }
func (p *countingProcessor) Enabled(context.Context, sdklog.EnabledParameters) bool { return true }

func TestMetricReaderOption(t *testing.T) {
	cleanEnv(t)
	reader := sdkmetric.NewManualReader()
	setup(t, WithMetricReader(reader), WithRuntimeMetrics(true))

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(t.Context(), &rm))
	var names []string
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			names = append(names, m.Name)
		}
	}
	assert.NotEmpty(t, names, "runtime metrics are recorded")
}

func TestSetupTwiceFails(t *testing.T) {
	cleanEnv(t)
	setup(t)
	_, err := Setup(t.Context())
	require.ErrorContains(t, err, "already called")
}

func TestResourcePrecedence(t *testing.T) {
	cleanEnv(t)
	t.Setenv("SERVICE_NAME", "from-service-name")
	res, err := newResource(t.Context(), newConfig(nil))
	require.NoError(t, err)
	assert.Equal(t, "from-service-name", value(res.Set(), "service.name"))
	assert.Equal(t, "local", value(res.Set(), "deployment.environment.name"))

	t.Setenv("OTEL_SERVICE_NAME", "from-otel")
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "team=data,deployment.environment.name=from-attrs")
	res, err = newResource(t.Context(), newConfig(nil))
	require.NoError(t, err)
	assert.Equal(t, "from-otel", value(res.Set(), "service.name"), "OTEL_SERVICE_NAME wins over SERVICE_NAME")
	assert.Equal(t, "data", value(res.Set(), "team"))
	assert.Equal(t, "from-attrs", value(res.Set(), "deployment.environment.name"))

	res, err = newResource(t.Context(), newConfig([]Option{
		WithServiceName("opt"), WithServiceVersion("v9"), WithEnvironment("prd"),
		WithAttributes(attribute.String("team", "core")),
	}))
	require.NoError(t, err)
	assert.Equal(t, "opt", value(res.Set(), "service.name"), "options win")
	assert.Equal(t, "v9", value(res.Set(), "service.version"))
	assert.Equal(t, "prd", value(res.Set(), "deployment.environment.name"))
	assert.Equal(t, "core", value(res.Set(), "team"))
}

func TestResourceWithoutServiceName(t *testing.T) {
	cleanEnv(t)
	res, err := newResource(t.Context(), newConfig(nil))
	require.NoError(t, err)
	assert.Contains(t, value(res.Set(), "service.name"), "unknown_service", "SDK default, not gglib's UnknownService")
}

func value(set *attribute.Set, key string) string {
	v, _ := set.Value(attribute.Key(key))
	return v.AsString()
}

func TestShutdownJoinsErrorsOnce(t *testing.T) {
	calls := 0
	fn := newShutdown([]func(context.Context) error{
		func(context.Context) error { calls++; return errors.New("a") },
		func(context.Context) error { calls++; return nil },
		func(context.Context) error { calls++; return errors.New("c") },
	})
	err := fn(t.Context())
	require.EqualError(t, err, "a\nc")
	assert.Equal(t, 3, calls)
	require.NoError(t, fn(t.Context()))
	assert.Equal(t, 3, calls)
}
