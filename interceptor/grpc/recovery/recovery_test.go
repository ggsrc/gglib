package recovery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	Method     string `json:"grpc_method"`
	PanicStack string `json:"panic.stack"`
}

func ctxWithLogger(buf *bytes.Buffer) context.Context {
	return zerolog.New(buf).WithContext(context.Background())
}

func readEntry(t *testing.T, buf *bytes.Buffer) logEntry {
	t.Helper()
	var e logEntry
	require.NoError(t, json.Unmarshal(buf.Bytes(), &e), buf.String())
	return e
}

type fakeStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s fakeStream) Context() context.Context { return s.ctx }

func TestUnaryServerPanic(t *testing.T) {
	for name, value := range map[string]any{
		"string": "something broke",
		"error":  errors.New("boom"),
		"int":    42,
	} {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			resp, err := UnaryServerInterceptor()(ctxWithLogger(&buf), "req",
				&grpc.UnaryServerInfo{FullMethod: "/svc/Crash"},
				func(context.Context, any) (any, error) { panic(value) })

			assert.Nil(t, resp)
			// Used to be a plain error, which went on the wire as Unknown.
			assert.Equal(t, codes.Internal, status.Code(err))
			assert.Equal(t, "internal error", status.Convert(err).Message(), "the panic value is not sent")

			e := readEntry(t, &buf)
			assert.Equal(t, "error", e.Level)
			assert.Equal(t, "grpc server panic", e.Message)
			assert.Equal(t, "/svc/Crash", e.Method)
			assert.Contains(t, e.Error, "panic:")
			assert.Contains(t, e.PanicStack, "recovery_test.go")
		})
	}
}

func TestUnaryServerPassesThrough(t *testing.T) {
	var buf bytes.Buffer
	want := status.Error(codes.NotFound, "nope")
	resp, err := UnaryServerInterceptor()(ctxWithLogger(&buf), "in",
		&grpc.UnaryServerInfo{FullMethod: "/svc/M"},
		func(_ context.Context, req any) (any, error) { return req.(string) + "-out", want })
	assert.Equal(t, "in-out", resp)
	assert.Equal(t, want, err)
	assert.Empty(t, buf.String())
}

// A panic in a streaming handler used to crash the process.
func TestStreamServerPanic(t *testing.T) {
	var buf bytes.Buffer
	err := StreamServerInterceptor()(nil, fakeStream{ctx: ctxWithLogger(&buf)},
		&grpc.StreamServerInfo{FullMethod: "/svc/Watch"},
		func(any, grpc.ServerStream) error { panic("stream boom") })

	assert.Equal(t, codes.Internal, status.Code(err))
	e := readEntry(t, &buf)
	assert.Equal(t, "/svc/Watch", e.Method)
	assert.Contains(t, e.Error, "stream boom")
}

func TestStreamServerPassesThrough(t *testing.T) {
	var buf bytes.Buffer
	want := errors.New("done")
	err := StreamServerInterceptor()(nil, fakeStream{ctx: ctxWithLogger(&buf)},
		&grpc.StreamServerInfo{FullMethod: "/svc/Watch"},
		func(any, grpc.ServerStream) error { return want })
	assert.Equal(t, want, err)
	assert.Empty(t, buf.String())
}

func TestUnaryClientPanic(t *testing.T) {
	var buf bytes.Buffer
	err := UnaryClientInterceptor()(ctxWithLogger(&buf), "/svc/Call", "req", nil, nil,
		func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
			panic("client boom")
		})
	assert.Equal(t, codes.Internal, status.Code(err))
	e := readEntry(t, &buf)
	assert.Equal(t, "grpc client panic", e.Message)
	assert.Equal(t, "/svc/Call", e.Method)
}

func TestUnaryClientPassesThrough(t *testing.T) {
	var buf bytes.Buffer
	want := status.Error(codes.Unavailable, "down")
	err := UnaryClientInterceptor()(ctxWithLogger(&buf), "/svc/Call", "req", nil, nil,
		func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error { return want })
	assert.Equal(t, want, err)
	assert.Empty(t, buf.String())
}
