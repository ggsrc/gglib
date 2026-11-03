// Package errlog logs gRPC errors at a level that matches their status code.
package errlog

import (
	"context"
	"encoding/json"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/ggsrc/gglib/zerolog/log"
)

// UnaryServerInterceptor logs gRPC handler errors at a level based on the
// status code: client errors (NotFound, AlreadyExists, …) at Warn, server
// errors (Internal, Unknown, …) at Error.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		if err != nil {
			logGRPCError(ctx, err, info.FullMethod, "grpc server error")
		}
		return resp, err
	}
}

// StreamServerInterceptor logs errors returned by streaming handlers, like
// UnaryServerInterceptor.
func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		err := handler(srv, ss)
		if err != nil {
			logGRPCError(ss.Context(), err, info.FullMethod, "grpc server error")
		}
		return err
	}
}

// UnaryClientInterceptor logs gRPC client call errors at a level based on the
// status code.
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
	st, ok := status.FromError(err)
	if !ok {
		// context.Canceled and DeadlineExceeded map to their codes, as
		// grpc-go does on the wire; other plain errors map to Unknown.
		st = status.FromContextError(err)
	}
	details := detailsJSON(st)
	log.Ctx(ctx).
		WithLevel(levelForCode(st.Code())).
		Str("grpc_method", method).
		Str("grpc_code", st.Code().String()).
		Func(func(e *zerolog.Event) {
			if details != "" {
				e.Str("sub_errors", details)
			}
		}).
		Err(err).
		Msg(msg)
}

// detailsJSON encodes the status details as a JSON array of protojson
// objects with their "@type", or returns "" when there are none.
func detailsJSON(st *status.Status) string {
	raw := make([]json.RawMessage, 0, len(st.Details()))
	for _, d := range st.Details() {
		msg, ok := d.(proto.Message)
		if !ok {
			continue
		}
		a, err := anypb.New(msg)
		if err != nil {
			continue
		}
		b, err := protojson.Marshal(a)
		if err != nil {
			continue
		}
		raw = append(raw, b)
	}
	if len(raw) == 0 {
		return ""
	}
	out, err := json.Marshal(raw)
	if err != nil {
		return ""
	}
	return string(out)
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
