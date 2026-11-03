package log

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
)

func attrs(t *testing.T, req, resp any) map[string]string {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	ctx, span := tp.Tracer("test").Start(context.Background(), "call")
	_, err := LogUnaryServerInterceptor()(ctx, req, &grpc.UnaryServerInfo{},
		func(context.Context, any) (any, error) { return resp, nil })
	require.NoError(t, err)
	span.End()

	out := map[string]string{}
	for _, kv := range rec.Ended()[0].Attributes() {
		out[string(kv.Key)] = kv.Value.AsString()
	}
	return out
}

func TestRecordsBodies(t *testing.T) {
	req, err := structpb.NewStruct(map[string]any{"name": "a"})
	require.NoError(t, err)
	got := attrs(t, req, map[string]int{"n": 1})
	assert.JSONEq(t, `{"name":"a"}`, got["grpc.request"], "protos use protojson")
	assert.JSONEq(t, `{"n":1}`, got["grpc.response"])
}

func TestNilResponseNotRecorded(t *testing.T) {
	got := attrs(t, "x", nil)
	assert.Contains(t, got, "grpc.request")
	assert.NotContains(t, got, "grpc.response")
}

func TestUnencodable(t *testing.T) {
	got := attrs(t, make(chan int), nil)
	assert.True(t, strings.HasPrefix(got["grpc.request"], "<unencodable: "))
}

func TestTruncatesLargeBodies(t *testing.T) {
	got := attrs(t, strings.Repeat("é", MaxBodySize), nil)
	body := got["grpc.request"]
	assert.True(t, strings.HasSuffix(body, "…(truncated)"))
	assert.LessOrEqual(t, len(body), MaxBodySize+len("…(truncated)"))
	assert.True(t, strings.HasPrefix(body, `"é`))
	assert.NotContains(t, body, "�", "cut on a rune boundary")
}

func TestNotRecordingSpan(t *testing.T) {
	called := false
	resp, err := LogUnaryServerInterceptor()(context.Background(), "x", &grpc.UnaryServerInfo{},
		func(context.Context, any) (any, error) { called = true; return "y", nil })
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, "y", resp)
}
