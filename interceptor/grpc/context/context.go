// Package context propagates the request context (mctx.AppContext) between
// services through gRPC metadata.
package context

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/ggsrc/gglib/interceptor/grpc/metautils"
	pkgmetadata "github.com/ggsrc/gglib/interceptor/metadata"
	"github.com/ggsrc/gglib/mctx"
	"github.com/ggsrc/gglib/zerolog/log"
)

// ContextUnaryServerInterceptor restores the AppContext from the incoming
// "app-ctx-bin" metadata, binds a logger with request_id and user_id to the
// context, and stores the "metadata" metadata value under
// metadata.ContextKeyMetadata.
func ContextUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		// set request source
		md := metautils.ExtractIncoming(ctx)
		appCtxStr := md.Get(pkgmetadata.CTX_KEY_APP_CTX)
		if appCtxStr != "" {
			appCtx, err := mctx.StringToAppCtx(appCtxStr)
			if err == nil {
				ctx = mctx.ContextWithAppCtx(ctx, appCtx)

				// Bind ctx to the logger so subsequent events carry it. Without
				// this, `log.Ctx(ctx).Info().Msg(...)` emits events whose
				// e.GetCtx() is nil — which means zerolog hooks that need to
				// read the OTel SpanContext from ctx (for correlating logs to
				// traces) cannot fire. With .Ctx(ctx), any downstream event
				// built from this logger has the ctx available to hooks.
				logger := log.Ctx(ctx).With().
					Str("request_id", appCtx.CommonParams.RequestID).
					Str("user_id", appCtx.User.UserID).
					Ctx(ctx)
				ctx = logger.Logger().WithContext(ctx)
			} else {
				log.Ctx(ctx).Error().Err(err).Msg("failed to convert app ctx")
			}
		}

		metadata := md.Get(pkgmetadata.CTX_KEY_METADATA)
		ctx = context.WithValue(ctx, pkgmetadata.ContextKeyMetadata, metadata)

		ret, err := handler(ctx, req)
		return ret, err
	}
}

// ContextUnaryClientInterceptor forwards all incoming metadata to the
// outgoing call. Outgoing metadata set by the caller comes first.
func ContextUnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		md, ok := metadata.FromIncomingContext(ctx)
		if ok {
			md = md.Copy()
		} else {
			md = metadata.MD{}
		}
		outgoingmd, ok := metadata.FromOutgoingContext(ctx)
		if ok {
			// explicitly declared outgoing md take precedence over transitive incoming md
			md = metadata.Join(outgoingmd, md)
		}
		ctx = metadata.NewOutgoingContext(ctx, md)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// NewContextWithGRPCMeta returns a new background context that carries the
// incoming and outgoing gRPC metadata of ctx but not its deadline or values.
func NewContextWithGRPCMeta(ctx context.Context) context.Context {
	newCtx := context.Background()
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		newCtx = metadata.NewIncomingContext(newCtx, md)
	}
	md, ok = metadata.FromOutgoingContext(ctx)
	if ok {
		newCtx = metadata.NewOutgoingContext(newCtx, md)
	}
	return newCtx
}
