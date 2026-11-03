// Package log records gRPC request and response bodies as span attributes.
package log

import (
	"context"

	"github.com/bytedance/sonic"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"

	"github.com/ggsrc/gglib/zerolog/log"
)

// LogUnaryServerInterceptor records the JSON-encoded request and response as
// the "grpc.request" and "grpc.response" attributes of the current span, when
// the span is recording.
func LogUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		span := trace.SpanFromContext(ctx)
		if span.IsRecording() {
			reqStr, err := sonic.MarshalString(req)
			if err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("failed to marshal request")
			}
			span.SetAttributes(attribute.String("grpc.request", reqStr))
		}
		resp, err = handler(ctx, req)
		if span.IsRecording() {
			respStr, err := sonic.MarshalString(resp)
			if err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("failed to marshal response")
			}
			span.SetAttributes(attribute.String("grpc.response", respStr))
		}
		return resp, err
	}
}
