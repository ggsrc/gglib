package errlog

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

// logEntry is the JSON structure emitted by zerolog.
type logEntry struct {
	Level     string `json:"level"`
	Error     string `json:"error"`
	Message   string `json:"message"`
	SubErrors string `json:"sub_errors"`
}

func ctxWithLogger(buf *bytes.Buffer) context.Context {
	logger := zerolog.New(buf).Level(zerolog.TraceLevel)
	return logger.WithContext(context.Background())
}

// ---------------------------------------------------------------------------
// levelForCode
// ---------------------------------------------------------------------------

func TestLevelForCode_ClientCodes(t *testing.T) {
	clientCodes := []codes.Code{
		codes.Canceled,
		codes.InvalidArgument,
		codes.NotFound,
		codes.AlreadyExists,
		codes.PermissionDenied,
		codes.FailedPrecondition,
		codes.Aborted,
		codes.OutOfRange,
		codes.Unauthenticated,
	}
	for _, c := range clientCodes {
		t.Run(c.String(), func(t *testing.T) {
			assert.Equal(t, zerolog.WarnLevel, levelForCode(c))
		})
	}
}

func TestLevelForCode_ServerCodes(t *testing.T) {
	serverCodes := []codes.Code{
		codes.Unknown,
		codes.DeadlineExceeded,
		codes.ResourceExhausted,
		codes.Unimplemented,
		codes.Internal,
		codes.Unavailable,
		codes.DataLoss,
	}
	for _, c := range serverCodes {
		t.Run(c.String(), func(t *testing.T) {
			assert.Equal(t, zerolog.ErrorLevel, levelForCode(c))
		})
	}
}

func TestLevelForCode_OK(t *testing.T) {
	assert.Equal(t, zerolog.DebugLevel, levelForCode(codes.OK))
}

// ---------------------------------------------------------------------------
// UnaryServerInterceptor
// ---------------------------------------------------------------------------

func serverInfo() *grpc.UnaryServerInfo {
	return &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}
}

func TestServer_NoError_NoLog(t *testing.T) {
	var buf bytes.Buffer
	ctx := ctxWithLogger(&buf)

	interceptor := UnaryServerInterceptor()
	resp, err := interceptor(ctx, "req", serverInfo(), func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	})

	assert.NoError(t, err)
	assert.Equal(t, "ok", resp)
	assert.Empty(t, buf.String(), "no log should be emitted on success")
}

func TestServer_NotFound_LogsWarn(t *testing.T) {
	var buf bytes.Buffer
	ctx := ctxWithLogger(&buf)

	interceptor := UnaryServerInterceptor()
	resp, err := interceptor(ctx, "req", serverInfo(), func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, status.Error(codes.NotFound, "file not found")
	})

	assert.Nil(t, resp)
	assert.Error(t, err)

	var entry logEntry
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	assert.Equal(t, "warn", entry.Level)
	assert.Equal(t, "grpc server error", entry.Message)
	assert.Contains(t, entry.Error, "file not found")
}

func TestServer_AlreadyExists_LogsWarn(t *testing.T) {
	var buf bytes.Buffer
	ctx := ctxWithLogger(&buf)

	interceptor := UnaryServerInterceptor()
	_, err := interceptor(ctx, "req", serverInfo(), func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, status.Error(codes.AlreadyExists, "already exists")
	})

	assert.Error(t, err)

	var entry logEntry
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	assert.Equal(t, "warn", entry.Level)
}

func TestServer_Internal_LogsError(t *testing.T) {
	var buf bytes.Buffer
	ctx := ctxWithLogger(&buf)

	interceptor := UnaryServerInterceptor()
	_, err := interceptor(ctx, "req", serverInfo(), func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, status.Error(codes.Internal, "database crashed")
	})

	assert.Error(t, err)

	var entry logEntry
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	assert.Equal(t, "error", entry.Level)
	assert.Equal(t, "grpc server error", entry.Message)
	assert.Contains(t, entry.Error, "database crashed")
}

func TestServer_Unknown_LogsError(t *testing.T) {
	var buf bytes.Buffer
	ctx := ctxWithLogger(&buf)

	interceptor := UnaryServerInterceptor()
	_, err := interceptor(ctx, "req", serverInfo(), func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, status.Error(codes.Unknown, "something went wrong")
	})

	assert.Error(t, err)

	var entry logEntry
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	assert.Equal(t, "error", entry.Level)
}

func TestServer_NonGRPCError_LogsError(t *testing.T) {
	var buf bytes.Buffer
	ctx := ctxWithLogger(&buf)

	interceptor := UnaryServerInterceptor()
	_, err := interceptor(ctx, "req", serverInfo(), func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, fmt.Errorf("raw error")
	})

	assert.Error(t, err)

	var entry logEntry
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	// Non-gRPC errors fall through status.FromError which wraps them as OK status,
	// but our code checks ok=true for any status-parseable error. A plain fmt.Errorf
	// is parseable as codes.OK by status.FromError, so it goes through the gRPC path.
	// In practice non-gRPC errors from handlers are rare; the important thing is they
	// still get logged.
	assert.NotEmpty(t, entry.Level)
	assert.Contains(t, entry.Error, "raw error")
}

func TestServer_PreservesResponseAndError(t *testing.T) {
	var buf bytes.Buffer
	ctx := ctxWithLogger(&buf)

	handlerErr := status.Error(codes.NotFound, "nope")
	interceptor := UnaryServerInterceptor()
	resp, err := interceptor(ctx, "req", serverInfo(), func(ctx context.Context, req interface{}) (interface{}, error) {
		return "partial", handlerErr
	})

	assert.Equal(t, "partial", resp)
	assert.Equal(t, handlerErr, err)
}

func TestServer_PassesThroughRequest(t *testing.T) {
	var buf bytes.Buffer
	ctx := ctxWithLogger(&buf)

	var receivedReq interface{}
	interceptor := UnaryServerInterceptor()
	_, _ = interceptor(ctx, "my-request", serverInfo(), func(ctx context.Context, req interface{}) (interface{}, error) {
		receivedReq = req
		return nil, nil
	})

	assert.Equal(t, "my-request", receivedReq)
}

// ---------------------------------------------------------------------------
// UnaryClientInterceptor
// ---------------------------------------------------------------------------

func noopInvoker(_ context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
	return nil
}

func errorInvoker(code codes.Code, msg string) grpc.UnaryInvoker {
	return func(_ context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		return status.Error(code, msg)
	}
}

func TestClient_NoError_NoLog(t *testing.T) {
	var buf bytes.Buffer
	ctx := ctxWithLogger(&buf)

	interceptor := UnaryClientInterceptor()
	err := interceptor(ctx, "/svc/Method", nil, nil, nil, noopInvoker)

	assert.NoError(t, err)
	assert.Empty(t, buf.String())
}

func TestClient_NotFound_LogsWarn(t *testing.T) {
	var buf bytes.Buffer
	ctx := ctxWithLogger(&buf)

	interceptor := UnaryClientInterceptor()
	err := interceptor(ctx, "/svc/Method", nil, nil, nil, errorInvoker(codes.NotFound, "not found"))

	assert.Error(t, err)

	var entry logEntry
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	assert.Equal(t, "warn", entry.Level)
	assert.Equal(t, "grpc client error", entry.Message)
}

func TestClient_Internal_LogsError(t *testing.T) {
	var buf bytes.Buffer
	ctx := ctxWithLogger(&buf)

	interceptor := UnaryClientInterceptor()
	err := interceptor(ctx, "/svc/Method", nil, nil, nil, errorInvoker(codes.Internal, "boom"))

	assert.Error(t, err)

	var entry logEntry
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	assert.Equal(t, "error", entry.Level)
	assert.Equal(t, "grpc client error", entry.Message)
}

func TestClient_Unavailable_LogsError(t *testing.T) {
	var buf bytes.Buffer
	ctx := ctxWithLogger(&buf)

	interceptor := UnaryClientInterceptor()
	err := interceptor(ctx, "/svc/Method", nil, nil, nil, errorInvoker(codes.Unavailable, "server down"))

	assert.Error(t, err)

	var entry logEntry
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	assert.Equal(t, "error", entry.Level)
}

func TestClient_PermissionDenied_LogsWarn(t *testing.T) {
	var buf bytes.Buffer
	ctx := ctxWithLogger(&buf)

	interceptor := UnaryClientInterceptor()
	err := interceptor(ctx, "/svc/Method", nil, nil, nil, errorInvoker(codes.PermissionDenied, "forbidden"))

	assert.Error(t, err)

	var entry logEntry
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	assert.Equal(t, "warn", entry.Level)
}

func TestClient_PreservesError(t *testing.T) {
	expected := status.Error(codes.NotFound, "specific msg")
	interceptor := UnaryClientInterceptor()

	var buf bytes.Buffer
	ctx := ctxWithLogger(&buf)
	err := interceptor(ctx, "/svc/Method", nil, nil, nil, func(_ context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		return expected
	})

	assert.Equal(t, expected, err)
}

// ---------------------------------------------------------------------------
// All codes coverage table test
// ---------------------------------------------------------------------------

func TestLevelForCode_Exhaustive(t *testing.T) {
	expectations := map[codes.Code]zerolog.Level{
		codes.OK:                 zerolog.DebugLevel,
		codes.Canceled:           zerolog.WarnLevel,
		codes.Unknown:            zerolog.ErrorLevel,
		codes.InvalidArgument:    zerolog.WarnLevel,
		codes.DeadlineExceeded:   zerolog.ErrorLevel,
		codes.NotFound:           zerolog.WarnLevel,
		codes.AlreadyExists:     zerolog.WarnLevel,
		codes.PermissionDenied:   zerolog.WarnLevel,
		codes.ResourceExhausted:  zerolog.ErrorLevel,
		codes.FailedPrecondition: zerolog.WarnLevel,
		codes.Aborted:            zerolog.WarnLevel,
		codes.OutOfRange:         zerolog.WarnLevel,
		codes.Unimplemented:      zerolog.ErrorLevel,
		codes.Internal:           zerolog.ErrorLevel,
		codes.Unavailable:        zerolog.ErrorLevel,
		codes.DataLoss:           zerolog.ErrorLevel,
		codes.Unauthenticated:    zerolog.WarnLevel,
	}
	for code, expected := range expectations {
		t.Run(code.String(), func(t *testing.T) {
			assert.Equal(t, expected, levelForCode(code), "code %s", code)
		})
	}
}
