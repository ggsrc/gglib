// Package log records gRPC request and response bodies as span attributes,
// for debugging. Bodies can contain secrets; enable it only where that is
// acceptable.
package log

import (
	"context"
	"encoding/json"
	"unicode/utf8"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// MaxBodySize is the number of bytes of each body kept in a span attribute;
// longer bodies are cut and marked with a trailing "…(truncated)".
const MaxBodySize = 4 << 10

// LogUnaryServerInterceptor records the JSON-encoded request and response as
// the "grpc.request" and "grpc.response" attributes of the current span, when
// the span is recording.
func LogUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		span := trace.SpanFromContext(ctx)
		if span.IsRecording() {
			span.SetAttributes(attribute.String("grpc.request", encode(req)))
		}
		resp, err := handler(ctx, req)
		if span.IsRecording() && resp != nil {
			span.SetAttributes(attribute.String("grpc.response", encode(resp)))
		}
		return resp, err
	}
}

func encode(v any) string {
	var (
		b   []byte
		err error
	)
	if m, ok := v.(proto.Message); ok {
		b, err = protojson.Marshal(m)
	} else {
		b, err = json.Marshal(v)
	}
	if err != nil {
		return "<unencodable: " + err.Error() + ">"
	}
	return truncate(string(b))
}

func truncate(s string) string {
	if len(s) <= MaxBodySize {
		return s
	}
	cut := MaxBodySize
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "…(truncated)"
}
