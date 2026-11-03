// Package recovery turns panics in gRPC calls into codes.Internal errors.
package recovery

import (
	"context"
	"fmt"
	"runtime/debug"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ggsrc/gglib/zerolog/log"
)

// errInternal is what the caller sees after a panic: the panic value and
// stack are logged, never sent.
var errInternal = status.Error(codes.Internal, "internal error")

// UnaryServerInterceptor recovers a panic in the rest of the chain or the
// handler, logs it with its stack and returns codes.Internal.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				resp, err = nil, logPanic(ctx, "server", info.FullMethod, r)
			}
		}()
		return handler(ctx, req)
	}
}

// StreamServerInterceptor recovers a panic in a streaming handler, logs it
// with its stack and returns codes.Internal.
func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = logPanic(ss.Context(), "server", info.FullMethod, r)
			}
		}()
		return handler(srv, ss)
	}
}

// UnaryClientInterceptor recovers a panic in the rest of the client chain,
// logs it with its stack and returns codes.Internal.
func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = logPanic(ctx, "client", method, r)
			}
		}()
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func logPanic(ctx context.Context, side, method string, r any) error {
	log.Ctx(ctx).Error().
		Str("grpc_method", method).
		Str("panic.stack", string(debug.Stack())).
		Err(fmt.Errorf("panic: %v", r)).
		Msgf("grpc %s panic", side)
	return errInternal
}
