package errlog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fullEntry struct {
	Level     string `json:"level"`
	Code      string `json:"grpc_code"`
	Method    string `json:"grpc_method"`
	SubErrors string `json:"sub_errors"`
	Message   string `json:"message"`
}

func serverCall(t *testing.T, err error) fullEntry {
	t.Helper()
	var buf bytes.Buffer
	_, got := UnaryServerInterceptor()(ctxWithLogger(&buf), "req", serverInfo(),
		func(context.Context, any) (any, error) { return nil, err })
	require.Equal(t, err, got)
	var e fullEntry
	require.NoError(t, json.Unmarshal(buf.Bytes(), &e), buf.String())
	return e
}

// A handler returning context.Canceled (a client that went away) used to be
// logged at Error as Unknown.
func TestContextErrorsUseTheirCodes(t *testing.T) {
	e := serverCall(t, context.Canceled)
	assert.Equal(t, "warn", e.Level)
	assert.Equal(t, "Canceled", e.Code)

	e = serverCall(t, fmt.Errorf("query: %w", context.DeadlineExceeded))
	assert.Equal(t, "DeadlineExceeded", e.Code)

	e = serverCall(t, errors.New("plain"))
	assert.Equal(t, "error", e.Level)
	assert.Equal(t, "Unknown", e.Code)
}

// Details used to be copied into a legacy proto type, losing ErrorInfo
// metadata and turning other detail types into {}.
func TestDetailsAreLoggedAsProtoJSON(t *testing.T) {
	st, err := status.New(codes.ResourceExhausted, "quota").WithDetails(
		&errdetails.ErrorInfo{Reason: "QUOTA", Domain: "alva.ai", Metadata: map[string]string{"limit": "10"}},
		&errdetails.BadRequest{FieldViolations: []*errdetails.BadRequest_FieldViolation{{Field: "name", Description: "empty"}}},
	)
	require.NoError(t, err)

	e := serverCall(t, st.Err())
	var details []map[string]any
	require.NoError(t, json.Unmarshal([]byte(e.SubErrors), &details))
	require.Len(t, details, 2)
	assert.Equal(t, "type.googleapis.com/google.rpc.ErrorInfo", details[0]["@type"])
	assert.Equal(t, map[string]any{"limit": "10"}, details[0]["metadata"])
	assert.Equal(t, "type.googleapis.com/google.rpc.BadRequest", details[1]["@type"])
	assert.NotEmpty(t, details[1]["fieldViolations"])
}

func TestNoDetailsNoField(t *testing.T) {
	e := serverCall(t, status.Error(codes.Internal, "x"))
	assert.Empty(t, e.SubErrors)
}

type fakeStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s fakeStream) Context() context.Context { return s.ctx }

func TestStreamServerInterceptor(t *testing.T) {
	var buf bytes.Buffer
	want := status.Error(codes.NotFound, "gone")
	err := StreamServerInterceptor()(nil, fakeStream{ctx: ctxWithLogger(&buf)},
		&grpc.StreamServerInfo{FullMethod: "/svc/Watch"},
		func(any, grpc.ServerStream) error { return want })
	require.Equal(t, want, err)
	var e fullEntry
	require.NoError(t, json.Unmarshal(buf.Bytes(), &e))
	assert.Equal(t, "warn", e.Level)
	assert.Equal(t, "/svc/Watch", e.Method)

	buf.Reset()
	require.NoError(t, StreamServerInterceptor()(nil, fakeStream{ctx: ctxWithLogger(&buf)},
		&grpc.StreamServerInfo{FullMethod: "/svc/Watch"},
		func(any, grpc.ServerStream) error { return nil }))
	assert.Empty(t, buf.String())
}
