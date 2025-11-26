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

func ContextUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// set request source
		md := metautils.ExtractIncoming(ctx)
		appCtxStr := md.Get(pkgmetadata.CTX_KEY_APP_CTX)
		if appCtxStr != "" {
			appCtx, err := mctx.StringToAppCtx(appCtxStr)
			if err == nil {
				ctx = mctx.ContextWithAppCtx(ctx, appCtx)
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

func ContextUnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
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
