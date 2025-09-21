package ratelimit

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const rateLimitErrMsg = "rate limit exceeded"

func RateLimitUnaryServerInterceptor(manager *RateLimitManager) grpc.UnaryServerInterceptor {
	if manager == nil {
		panic("RateLimitManager cannot be nil")
	}

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if !manager.Allow(ctx, info.FullMethod) {
			return nil, status.Error(codes.ResourceExhausted, rateLimitErrMsg)
		}

		return handler(ctx, req)
	}
}

func RateLimitUnaryClientInterceptor(manager *RateLimitManager) grpc.UnaryClientInterceptor {
	if manager == nil {
		panic("RateLimitManager cannot be nil")
	}

	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if !manager.Allow(ctx, method) {
			return status.Error(codes.ResourceExhausted, rateLimitErrMsg)
		}

		err := invoker(ctx, method, req, reply, cc, opts...)
		if err != nil {
			return err
		}
		return err
	}
}
