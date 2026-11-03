// Package recovery turns panics in gRPC unary calls into errors.
package recovery

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/getsentry/sentry-go"
	"google.golang.org/grpc"

	"github.com/ggsrc/gglib/zerolog/log"
)

// PanicHandler is called after a panic has been recovered, with the
// recovered value and the stack.
type PanicHandler func(ctx context.Context, method string, r any, stack []byte)

// UnaryServerInterceptor recovers a panic in the handler, logs it with its
// stack, calls panicHandler if it is not nil, and returns an error.
func UnaryServerInterceptor(panicHandler PanicHandler) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Ctx(ctx).Error().
					Str("panic.stack", string(debug.Stack())).
					Err(fmt.Errorf("[panic] %v", r)).
					Msgf("%s grpc server panic", strings.Trim(info.FullMethod, "/"))
				err = errors.New("server Internal Error")
				if panicHandler != nil {
					panicHandler(ctx, info.FullMethod, r, debug.Stack())
				}
			}
		}()

		return handler(ctx, req)
	}
}

// UnaryClientInterceptor recovers a panic in the client call chain, logs it
// with its stack, calls panicHandler if it is not nil, and returns an error.
func UnaryClientInterceptor(panicHandler PanicHandler) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) (err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Ctx(ctx).Error().
					Str("panic.stack", string(debug.Stack())).
					Err(fmt.Errorf("[panic] %v", r)).
					Msgf("%s grpc client panic", strings.Trim(method, "/"))
				err = errors.New("server Internal Error")
				if panicHandler != nil {
					panicHandler(ctx, method, r, debug.Stack())
				}
			}
		}()

		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// SentryPanicHandler initializes Sentry with ravenDSN and returns a
// PanicHandler that reports recovered panics to it.
func SentryPanicHandler(ravenDSN string) PanicHandler {
	err := sentry.Init(sentry.ClientOptions{Dsn: ravenDSN})
	if err != nil {
		log.Err(err).Msg("sentry init failed, ignore it and continue...")
	}

	return func(ctx context.Context, method string, r any, stack []byte) {
		hub := sentry.CurrentHub()
		hub.Recover(r)
	}
}
