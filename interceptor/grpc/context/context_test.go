package context

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	pkgmetadata "github.com/ggsrc/gglib/interceptor/metadata"
	"github.com/ggsrc/gglib/mctx"
)

const appCtxJSON = `{"user":{"user_id":"42"},"common_params":{"request_id":"r1"}}`

func loggerCtx(buf *bytes.Buffer) context.Context {
	return zerolog.New(buf).WithContext(context.Background())
}

func TestUnaryServerRestoresAppContext(t *testing.T) {
	var buf bytes.Buffer
	ctx := metadata.NewIncomingContext(loggerCtx(&buf), metadata.Pairs(
		pkgmetadata.CTX_KEY_APP_CTX, appCtxJSON,
		pkgmetadata.CTX_KEY_METADATA, "m1",
	))
	var got context.Context
	_, err := ContextUnaryServerInterceptor()(ctx, nil, &grpc.UnaryServerInfo{}, func(c context.Context, _ any) (any, error) {
		got = c
		return nil, nil
	})
	require.NoError(t, err)

	appCtx, ok := mctx.AppCtxFromContext(got)
	require.True(t, ok)
	assert.Equal(t, "42", appCtx.User.UserID)
	assert.Equal(t, "m1", got.Value(pkgmetadata.ContextKeyMetadata))

	zerolog.Ctx(got).Info().Msg("in handler")
	assert.Contains(t, buf.String(), `"request_id":"r1"`)
	assert.Contains(t, buf.String(), `"user_id":"42"`)
}

// The parse error used to be logged at Error with the raw header, tokens
// included.
func TestMalformedAppContextLogsLengthOnly(t *testing.T) {
	var buf bytes.Buffer
	raw := `{"user":{"user_id":"42","access_token":"SECRET-TOKEN"`
	ctx := metadata.NewIncomingContext(loggerCtx(&buf), metadata.Pairs(pkgmetadata.CTX_KEY_APP_CTX, raw))
	var got context.Context
	_, err := ContextUnaryServerInterceptor()(ctx, nil, &grpc.UnaryServerInfo{}, func(c context.Context, _ any) (any, error) {
		got = c
		return nil, nil
	})
	require.NoError(t, err, "the call proceeds")
	_, ok := mctx.AppCtxFromContext(got)
	assert.False(t, ok)

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	assert.Equal(t, "warn", entry["level"])
	assert.InDelta(t, len(raw), entry["app_ctx_bytes"], 0)
	assert.NotContains(t, buf.String(), "SECRET-TOKEN")
}

func TestNoAppContext(t *testing.T) {
	var got context.Context
	_, err := ContextUnaryServerInterceptor()(context.Background(), nil, &grpc.UnaryServerInfo{}, func(c context.Context, _ any) (any, error) {
		got = c
		return nil, nil
	})
	require.NoError(t, err)
	_, ok := mctx.AppCtxFromContext(got)
	assert.False(t, ok)
	assert.Empty(t, got.Value(pkgmetadata.ContextKeyMetadata))
}

type fakeStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s fakeStream) Context() context.Context { return s.ctx }

// Streaming calls used to get no app context at all.
func TestStreamServerRestoresAppContext(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(pkgmetadata.CTX_KEY_APP_CTX, appCtxJSON))
	var got context.Context
	err := ContextStreamServerInterceptor()(nil, fakeStream{ctx: ctx}, &grpc.StreamServerInfo{}, func(_ any, ss grpc.ServerStream) error {
		got = ss.Context()
		return nil
	})
	require.NoError(t, err)
	appCtx, ok := mctx.AppCtxFromContext(got)
	require.True(t, ok)
	assert.Equal(t, "42", appCtx.User.UserID)
}

// outgoingOf runs the client interceptor and returns the metadata the
// invoker received.
func outgoingOf(t *testing.T, ctx context.Context, mode ...ForwardMode) metadata.MD {
	t.Helper()
	var got metadata.MD
	err := ContextUnaryClientInterceptor(mode...)(ctx, "/svc/Method", nil, nil, nil,
		func(c context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
			got, _ = metadata.FromOutgoingContext(c)
			return nil
		})
	require.NoError(t, err)
	return got
}

func incoming() metadata.MD {
	return metadata.MD{
		"app-ctx-bin":           {appCtxJSON},
		"x-request-id":          {"req-1"},
		"traceparent":           {"00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"},
		"authorization":         {"Bearer END-USER-SECRET"},
		"cookie":                {"session=abc"},
		"req-params-bin":        {"{}"},
		"x-research-request-id": {"rr-1"},
		"grpc-accept-encoding":  {"gzip"},
		"user-agent":            {"grpc-go/1.85.0"},
		"content-type":          {"application/grpc"},
		":authority":            {"backend:9090"},
	}
}

func TestAllowlistModeForwardsOnlyIdentityAndTracing(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), incoming())
	got := outgoingOf(t, ctx, ForwardAllowlist)
	keys := make([]string, 0, len(got))
	for k := range got {
		keys = append(keys, k)
	}
	assert.ElementsMatch(t, []string{"app-ctx-bin", "x-request-id", "traceparent"}, keys)
}

// Log-only mode keeps today's forwarding of non-transport keys and logs each
// key name the allowlist would drop, once per process, without its value.
func TestLogOnlyModeForwardsAndLogsNames(t *testing.T) {
	loggedKeys.Clear()
	var buf bytes.Buffer
	ctx := metadata.NewIncomingContext(loggerCtx(&buf), incoming())

	got := outgoingOf(t, ctx, ForwardLogOnly)
	assert.Equal(t, []string{"Bearer END-USER-SECRET"}, got["authorization"])
	assert.Equal(t, []string{"rr-1"}, got["x-research-request-id"])
	assert.NotContains(t, got, "grpc-accept-encoding", "transport keys are never forwarded")
	assert.NotContains(t, got, "user-agent")
	assert.NotContains(t, got, ":authority")

	out := buf.String()
	for _, k := range []string{"authorization", "cookie", "req-params-bin", "x-research-request-id"} {
		assert.Contains(t, out, `"metadata_key":"`+k+`"`)
	}
	assert.NotContains(t, out, "END-USER-SECRET")
	assert.NotContains(t, out, `"metadata_key":"app-ctx-bin"`)

	buf.Reset()
	outgoingOf(t, ctx, ForwardLogOnly)
	assert.Empty(t, buf.String(), "each key is logged once")
}

// An explicit outgoing value used to be joined with the forwarded one, so the
// callee got two values.
func TestExplicitOutgoingReplacesIncoming(t *testing.T) {
	for _, mode := range []ForwardMode{ForwardLogOnly, ForwardAllowlist} {
		ctx := metadata.NewIncomingContext(context.Background(), incoming())
		ctx = metadata.AppendToOutgoingContext(ctx, "app-ctx-bin", "service-identity", "authorization", "Bearer SERVICE")
		got := outgoingOf(t, ctx, mode)
		assert.Equal(t, []string{"service-identity"}, got["app-ctx-bin"])
		assert.Equal(t, []string{"Bearer SERVICE"}, got["authorization"])
	}
}

func TestNoIncomingMetadata(t *testing.T) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "k", "v")
	got := outgoingOf(t, ctx, ForwardAllowlist)
	assert.Equal(t, metadata.MD{"k": {"v"}}, got)
	assert.Empty(t, outgoingOf(t, context.Background()))
}

func TestDefaultForwardMode(t *testing.T) {
	t.Setenv(ForwardModeEnv, "")
	assert.Equal(t, ForwardLogOnly, DefaultForwardMode())
	t.Setenv(ForwardModeEnv, " Allowlist ")
	assert.Equal(t, ForwardAllowlist, DefaultForwardMode())

	ctx := metadata.NewIncomingContext(context.Background(), incoming())
	got := outgoingOf(t, ctx)
	assert.NotContains(t, got, "authorization", "the interceptor picks up the environment default")
	t.Setenv(ForwardModeEnv, "something-else")
	assert.Equal(t, ForwardLogOnly, DefaultForwardMode())
}

func TestForwardKeysAreLowercase(t *testing.T) {
	for _, k := range ForwardKeys {
		assert.Equal(t, strings.ToLower(k), k)
	}
}
