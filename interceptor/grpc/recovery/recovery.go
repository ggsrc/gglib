package recovery

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/getsentry/sentry-go"
	"google.golang.org/grpc"

	"github.com/ggsrc/gglib/zerolog/log"
)

type PanicHandler func(ctx context.Context, method string, r any, stack []byte)

func UnaryServerInterceptor(panicHandler PanicHandler) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Ctx(ctx).Error().
					Str("panic.stack", string(debug.Stack())).
					Err(fmt.Errorf("[panic] %v", r)).
					Msgf("%s grpc server panic", strings.Trim(info.FullMethod, "/"))
				err = fmt.Errorf("server Internal Error")
				if panicHandler != nil {
					panicHandler(ctx, info.FullMethod, r, debug.Stack())
				}
			}
		}()

		return handler(ctx, req)
	}
}

func UnaryClientInterceptor(panicHandler PanicHandler) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) (err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Ctx(ctx).Error().
					Str("panic.stack", string(debug.Stack())).
					Err(fmt.Errorf("[panic] %v", r)).
					Msgf("%s grpc client panic", strings.Trim(method, "/"))
				err = fmt.Errorf("server Internal Error")
				if panicHandler != nil {
					panicHandler(ctx, method, r, debug.Stack())
				}
			}
		}()

		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// SentryPanicHandler 返回一个基于 Sentry 的 panic 处理函数
func SentryPanicHandler(ravenDSN string) PanicHandler {
	err := sentry.Init(sentry.ClientOptions{Dsn: ravenDSN})
	if err != nil {
		log.Err(err).Msg("sentry init failed, ignore it and continue...")
	}

	return func(ctx context.Context, method string, r interface{}, stack []byte) {
		hub := sentry.CurrentHub()
		hub.Recover(r)
	}
}
