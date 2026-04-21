package errlog

import (
	"context"
	"encoding/json"

	"github.com/jinzhu/copier"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ggsrc/gglib/interceptor/grpc/errors"
	"github.com/ggsrc/gglib/zerolog/log"
)

// UnaryServerInterceptor logs gRPC handler errors at an appropriate level
// based on the status code: client errors (NotFound, AlreadyExists, …) at
// Warn, server errors (Internal, Unknown, …) at Error.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		resp, err := handler(ctx, req)
		if err != nil {
			logGRPCError(ctx, err, info.FullMethod, "grpc server error")
		}
		return resp, err
	}
}

// UnaryClientInterceptor logs gRPC client call errors at an appropriate level.
func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		err := invoker(ctx, method, req, reply, cc, opts...)
		if err != nil {
			logGRPCError(ctx, err, method, "grpc client error")
		}
		return err
	}
}

func logGRPCError(ctx context.Context, err error, method, msg string) {
	grpcStatus, ok := status.FromError(err)
	if !ok {
		log.Ctx(ctx).Error().Err(err).Msg(msg)
		return
	}

	var subErrs []*errors.ErrorInfo
	for _, detail := range grpcStatus.Details() {
		if detail == nil {
			continue
		}
		var subErr errors.ErrorInfo
		if copyErr := copier.Copy(&subErr, detail); copyErr == nil {
			subErrs = append(subErrs, &subErr)
		}
	}

	subErrStr, _ := json.Marshal(subErrs)
	log.Ctx(ctx).
		WithLevel(levelForCode(grpcStatus.Code())).
		Str("grpc_method", method).
		Str("sub_errors", string(subErrStr)).
		Err(err).
		Msg(msg)
}

// levelForCode maps gRPC status codes to zerolog levels.
// Client-facing codes get Warn; server-side failures get Error.
func levelForCode(c codes.Code) zerolog.Level {
	switch c {
	case codes.OK:
		return zerolog.DebugLevel
	case codes.Canceled,
		codes.InvalidArgument,
		codes.NotFound,
		codes.AlreadyExists,
		codes.PermissionDenied,
		codes.FailedPrecondition,
		codes.Aborted,
		codes.OutOfRange,
		codes.Unauthenticated:
		return zerolog.WarnLevel
	default:
		return zerolog.ErrorLevel
	}
}
