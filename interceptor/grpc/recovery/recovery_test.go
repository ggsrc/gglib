package recovery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type logEntry struct {
	Level      string `json:"level"`
	Error      string `json:"error"`
	Message    string `json:"message"`
	PanicStack string `json:"panic.stack"`
}

func ctxWithLogger(buf *bytes.Buffer) context.Context {
	logger := zerolog.New(buf).Level(zerolog.TraceLevel)
	return logger.WithContext(context.Background())
}

func serverInfo(method string) *grpc.UnaryServerInfo {
	return &grpc.UnaryServerInfo{FullMethod: method}
}

// ---------------------------------------------------------------------------
// UnaryServerInterceptor — panic recovery
// ---------------------------------------------------------------------------

func TestServer_PanicRecovery_ReturnsError(t *testing.T) {
	var buf bytes.Buffer
	ctx := ctxWithLogger(&buf)

	interceptor := UnaryServerInterceptor(nil)
	resp, err := interceptor(ctx, "req", serverInfo("/test.Svc/Crash"), func(ctx context.Context, req interface{}) (interface{}, error) {
		panic("something broke")
	})

	assert.Nil(t, resp)
	require.Error(t, err)
	assert.Equal(t, "server Internal Error", err.Error())
}

func TestServer_PanicRecovery_LogsPanic(t *testing.T) {
	var buf bytes.Buffer
	ctx := ctxWithLogger(&buf)

	interceptor := UnaryServerInterceptor(nil)
	_, _ = interceptor(ctx, "req", serverInfo("/test.Svc/Crash"), func(ctx context.Context, req interface{}) (interface{}, error) {
		panic("kaboom")
	})

	var entry logEntry
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	assert.Equal(t, "error", entry.Level)
	assert.Contains(t, entry.Message, "grpc server panic")
	assert.Contains(t, entry.Message, "test.Svc/Crash")
	assert.Contains(t, entry.Error, "[panic] kaboom")
	assert.NotEmpty(t, entry.PanicStack, "should contain stack trace")
}

func TestServer_PanicRecovery_CallsPanicHandler(t *testing.T) {
	var buf bytes.Buffer
	ctx := ctxWithLogger(&buf)

	var handlerCalled bool
	var capturedMethod string
	var capturedR any
	handler := func(_ context.Context, method string, r any, _ []byte) {
		handlerCalled = true
		capturedMethod = method
		capturedR = r
	}

	interceptor := UnaryServerInterceptor(handler)
	_, _ = interceptor(ctx, "req", serverInfo("/test.Svc/Explode"), func(ctx context.Context, req interface{}) (interface{}, error) {
		panic("boom")
	})

	assert.True(t, handlerCalled)
	assert.Equal(t, "/test.Svc/Explode", capturedMethod)
	assert.Equal(t, "boom", capturedR)
}

func TestServer_PanicRecovery_NilPanicHandler(t *testing.T) {
	var buf bytes.Buffer
	ctx := ctxWithLogger(&buf)

	interceptor := UnaryServerInterceptor(nil)
	_, err := interceptor(ctx, "req", serverInfo("/test.Svc/Crash"), func(ctx context.Context, req interface{}) (interface{}, error) {
		panic("safe with nil handler")
	})

	require.Error(t, err)
	assert.Equal(t, "server Internal Error", err.Error())
}

func TestServer_PanicRecovery_NonStringPanic(t *testing.T) {
	var buf bytes.Buffer
	ctx := ctxWithLogger(&buf)

	interceptor := UnaryServerInterceptor(nil)
	_, err := interceptor(ctx, "req", serverInfo("/test.Svc/Crash"), func(ctx context.Context, req interface{}) (interface{}, error) {
		panic(42)
	})

	require.Error(t, err)
	assert.Equal(t, "server Internal Error", err.Error())

	var entry logEntry
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	assert.Contains(t, entry.Error, "[panic] 42")
}

func TestServer_PanicRecovery_ErrorPanic(t *testing.T) {
	var buf bytes.Buffer
	ctx := ctxWithLogger(&buf)

	interceptor := UnaryServerInterceptor(nil)
	_, err := interceptor(ctx, "req", serverInfo("/test.Svc/Crash"), func(ctx context.Context, req interface{}) (interface{}, error) {
		panic(fmt.Errorf("wrapped error"))
	})

	require.Error(t, err)

	var entry logEntry
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	assert.Contains(t, entry.Error, "wrapped error")
}

// ---------------------------------------------------------------------------
// UnaryServerInterceptor — normal flow (no panic)
// ---------------------------------------------------------------------------

func TestServer_NoPanic_PassesThroughSuccess(t *testing.T) {
	var buf bytes.Buffer
	ctx := ctxWithLogger(&buf)

	interceptor := UnaryServerInterceptor(nil)
	resp, err := interceptor(ctx, "req", serverInfo("/test.Svc/OK"), func(ctx context.Context, req interface{}) (interface{}, error) {
		return "result", nil
	})

	assert.NoError(t, err)
	assert.Equal(t, "result", resp)
	assert.Empty(t, buf.String(), "no log on normal flow")
}

func TestServer_NoPanic_PassesThroughError(t *testing.T) {
	var buf bytes.Buffer
	ctx := ctxWithLogger(&buf)

	handlerErr := status.Error(codes.NotFound, "not found")
	interceptor := UnaryServerInterceptor(nil)
	resp, err := interceptor(ctx, "req", serverInfo("/test.Svc/NotFound"), func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, handlerErr
	})

	assert.Nil(t, resp)
	assert.Equal(t, handlerErr, err, "error should pass through unmodified")
	assert.Empty(t, buf.String(), "recovery should not log returned errors")
}

func TestServer_NoPanic_PassesThroughInternalError(t *testing.T) {
	var buf bytes.Buffer
	ctx := ctxWithLogger(&buf)

	handlerErr := status.Error(codes.Internal, "db error")
	interceptor := UnaryServerInterceptor(nil)
	_, err := interceptor(ctx, "req", serverInfo("/test.Svc/Fail"), func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, handlerErr
	})

	assert.Equal(t, handlerErr, err)
	assert.Empty(t, buf.String(), "recovery should not log returned errors")
}

func TestServer_NoPanic_PassesThroughRequest(t *testing.T) {
	var buf bytes.Buffer
	ctx := ctxWithLogger(&buf)

	var received interface{}
	interceptor := UnaryServerInterceptor(nil)
	_, _ = interceptor(ctx, "my-data", serverInfo("/test.Svc/Echo"), func(ctx context.Context, req interface{}) (interface{}, error) {
		received = req
		return req, nil
	})

	assert.Equal(t, "my-data", received)
}

// ---------------------------------------------------------------------------
// UnaryClientInterceptor — panic recovery
// ---------------------------------------------------------------------------

func noopInvoker(_ context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
	return nil
}

func TestClient_PanicRecovery_ReturnsError(t *testing.T) {
	var buf bytes.Buffer
	ctx := ctxWithLogger(&buf)

	interceptor := UnaryClientInterceptor(nil)
	err := interceptor(ctx, "/test.Svc/Call", nil, nil, nil, func(_ context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		panic("client panic")
	})

	require.Error(t, err)
	assert.Equal(t, "server Internal Error", err.Error())
}

func TestClient_PanicRecovery_LogsPanic(t *testing.T) {
	var buf bytes.Buffer
	ctx := ctxWithLogger(&buf)

	interceptor := UnaryClientInterceptor(nil)
	_ = interceptor(ctx, "/test.Svc/Call", nil, nil, nil, func(_ context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		panic("client boom")
	})

	var entry logEntry
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	assert.Equal(t, "error", entry.Level)
	assert.Contains(t, entry.Message, "grpc client panic")
	assert.Contains(t, entry.Message, "test.Svc/Call")
	assert.Contains(t, entry.Error, "[panic] client boom")
	assert.NotEmpty(t, entry.PanicStack)
}

func TestClient_PanicRecovery_CallsPanicHandler(t *testing.T) {
	var buf bytes.Buffer
	ctx := ctxWithLogger(&buf)

	var handlerCalled bool
	var capturedMethod string
	handler := func(_ context.Context, method string, r any, _ []byte) {
		handlerCalled = true
		capturedMethod = method
	}

	interceptor := UnaryClientInterceptor(handler)
	_ = interceptor(ctx, "/test.Svc/Explode", nil, nil, nil, func(_ context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		panic("client explode")
	})

	assert.True(t, handlerCalled)
	assert.Equal(t, "/test.Svc/Explode", capturedMethod)
}

func TestClient_PanicRecovery_NilPanicHandler(t *testing.T) {
	var buf bytes.Buffer
	ctx := ctxWithLogger(&buf)

	interceptor := UnaryClientInterceptor(nil)
	err := interceptor(ctx, "/test.Svc/Crash", nil, nil, nil, func(_ context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		panic("nil handler safe")
	})

	require.Error(t, err)
	assert.Equal(t, "server Internal Error", err.Error())
}

// ---------------------------------------------------------------------------
// UnaryClientInterceptor — normal flow (no panic)
// ---------------------------------------------------------------------------

func TestClient_NoPanic_PassesThroughSuccess(t *testing.T) {
	var buf bytes.Buffer
	ctx := ctxWithLogger(&buf)

	interceptor := UnaryClientInterceptor(nil)
	err := interceptor(ctx, "/test.Svc/Call", nil, nil, nil, noopInvoker)

	assert.NoError(t, err)
	assert.Empty(t, buf.String())
}

func TestClient_NoPanic_PassesThroughError(t *testing.T) {
	var buf bytes.Buffer
	ctx := ctxWithLogger(&buf)

	expected := status.Error(codes.NotFound, "gone")
	interceptor := UnaryClientInterceptor(nil)
	err := interceptor(ctx, "/test.Svc/Call", nil, nil, nil, func(_ context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		return expected
	})

	assert.Equal(t, expected, err, "error should pass through unmodified")
	assert.Empty(t, buf.String(), "recovery should not log returned errors")
}
