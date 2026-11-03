// Package context propagates the request context (mctx.AppContext) between
// services through gRPC metadata.
//
// Services in the cluster trust each other: the caller's identity
// (app-ctx-bin) travels with every call. Other incoming metadata — tokens,
// cookies, routing markers — stays at the hop where it arrived. See
// ForwardKeys and ForwardMode.
package context

import (
	"context"
	"os"
	"slices"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	pkgmetadata "github.com/ggsrc/gglib/interceptor/metadata"
	"github.com/ggsrc/gglib/mctx"
	"github.com/ggsrc/gglib/zerolog/log"
)

// ForwardKeys are the incoming metadata keys a client call forwards by
// default: the caller's identity, the request id that ties log lines
// together, and trace context for services without OpenTelemetry (W3C and
// Istio's B3).
var ForwardKeys = []string{
	pkgmetadata.CTX_KEY_APP_CTX,
	"x-request-id",
	"traceparent",
	"tracestate",
	"x-b3-traceid",
	"x-b3-spanid",
	"x-b3-parentspanid",
	"x-b3-sampled",
	"x-b3-flags",
	"b3",
}

// ForwardMode selects what ContextUnaryClientInterceptor does with incoming
// metadata keys that are not in ForwardKeys.
type ForwardMode int

const (
	// ForwardLogOnly forwards them, as before, and logs each key name once
	// per process (never the value), so that a rollout can confirm nothing
	// depends on them before switching to ForwardAllowlist.
	ForwardLogOnly ForwardMode = iota
	// ForwardAllowlist drops them.
	ForwardAllowlist
)

// ForwardModeEnv names the environment variable that sets the default mode:
// "allowlist" selects ForwardAllowlist; anything else, ForwardLogOnly.
const ForwardModeEnv = "GGLIB_GRPC_METADATA_FORWARDING"

// DefaultForwardMode returns the mode set by ForwardModeEnv.
func DefaultForwardMode() ForwardMode {
	if strings.EqualFold(strings.TrimSpace(os.Getenv(ForwardModeEnv)), "allowlist") {
		return ForwardAllowlist
	}
	return ForwardLogOnly
}

// ContextUnaryServerInterceptor restores the AppContext from the incoming
// "app-ctx-bin" metadata, binds a logger with request_id and user_id to the
// context, and stores the "metadata" metadata value under
// metadata.ContextKeyMetadata. A malformed app context is logged (its length,
// never its content) and the call proceeds without one.
func ContextUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		return handler(withAppContext(ctx), req)
	}
}

// ContextStreamServerInterceptor does for streaming calls what
// ContextUnaryServerInterceptor does for unary ones.
func ContextStreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return handler(srv, &serverStream{ServerStream: ss, ctx: withAppContext(ss.Context())})
	}
}

type serverStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *serverStream) Context() context.Context { return s.ctx }

func withAppContext(ctx context.Context) context.Context {
	md, _ := metadata.FromIncomingContext(ctx)
	if raw := first(md, pkgmetadata.CTX_KEY_APP_CTX); raw != "" {
		appCtx, err := mctx.StringToAppCtx(raw)
		if err != nil {
			log.Ctx(ctx).Warn().Int("app_ctx_bytes", len(raw)).Msg("ignoring malformed app context")
		} else {
			ctx = mctx.ContextWithAppCtx(ctx, appCtx)
			// Bind ctx so that hooks reading the span from the event work.
			logger := log.Ctx(ctx).With().
				Str("request_id", appCtx.CommonParams.RequestID).
				Str("user_id", appCtx.User.UserID).
				Ctx(ctx).
				Logger()
			ctx = logger.WithContext(ctx)
		}
	}
	return context.WithValue(ctx, pkgmetadata.ContextKeyMetadata, first(md, pkgmetadata.CTX_KEY_METADATA))
}

func first(md metadata.MD, key string) string {
	if v := md.Get(key); len(v) > 0 {
		return v[0]
	}
	return ""
}

// ContextUnaryClientInterceptor forwards incoming metadata of the current
// server call to the outgoing call. A key the caller set explicitly on the
// outgoing context replaces the incoming value instead of being added to
// it. Which other keys are forwarded depends on mode (default:
// DefaultForwardMode).
func ContextUnaryClientInterceptor(mode ...ForwardMode) grpc.UnaryClientInterceptor {
	m := DefaultForwardMode()
	if len(mode) > 0 {
		m = mode[0]
	}
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return invoker(forwardContext(ctx, method, m), method, req, reply, cc, opts...)
	}
}

func forwardContext(ctx context.Context, method string, mode ForwardMode) context.Context {
	incoming, ok := metadata.FromIncomingContext(ctx)
	if !ok || len(incoming) == 0 {
		return ctx
	}
	outgoing, _ := metadata.FromOutgoingContext(ctx)
	md := outgoing.Copy()
	if md == nil {
		md = metadata.MD{}
	}
	for key, values := range incoming {
		if _, set := md[key]; set || isTransportKey(key) {
			continue
		}
		if !slices.Contains(ForwardKeys, key) {
			if mode == ForwardAllowlist {
				continue
			}
			logWouldDrop(ctx, key, method)
		}
		md[key] = append([]string(nil), values...)
	}
	return metadata.NewOutgoingContext(ctx, md)
}

// isTransportKey reports keys that belong to the HTTP/2 transport of one hop
// and are never forwarded in any mode.
func isTransportKey(key string) bool {
	switch {
	case strings.HasPrefix(key, ":"), strings.HasPrefix(key, "grpc-"):
		return true
	case key == "content-type", key == "user-agent", key == "te", key == "host":
		return true
	}
	return false
}

var loggedKeys sync.Map

func logWouldDrop(ctx context.Context, key, method string) {
	if _, seen := loggedKeys.LoadOrStore(key, struct{}{}); seen {
		return
	}
	log.Ctx(ctx).Info().
		Str("metadata_key", key).
		Str("grpc_method", method).
		Str("setting", ForwardModeEnv+"=allowlist").
		Msg("forwarded a metadata key that the allowlist mode would drop")
}
