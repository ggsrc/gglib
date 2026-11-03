package zerolog

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/trace"
)

const scopeName = "github.com/ggsrc/gglib/zerolog"

// otlpWriter sends every log line to the global OpenTelemetry logger
// provider. zerolog hands each writer the event as one JSON object; the
// writer turns it into a record: the message is the body, the level the
// severity, the time field the timestamp, trace_id and span_id the record's
// trace context, and every other field an attribute. The provider is looked
// up through the global delegate, so gglib/otel may install it before or
// after Setup.
//
// The trace flags of the span are not in the line, so records carry the
// trace and span ids without flags.
type otlpWriter struct {
	logger otellog.Logger
}

func newOTLPWriter() *otlpWriter {
	return &otlpWriter{logger: otel.GetLoggerProvider().Logger(scopeName)}
}

// Write sends a line whose level is read from its level field.
func (w *otlpWriter) Write(p []byte) (int, error) {
	return w.WriteLevel(zerolog.NoLevel, p)
}

// WriteLevel sends a line. It never fails: a line that is not a JSON object
// is dropped, since stdout still has it.
func (w *otlpWriter) WriteLevel(level zerolog.Level, p []byte) (int, error) {
	ld, _ := decoders.Get().(*lineDecoder)
	ld.r.Reset(p)
	dec := ld.dec
	dec.Reset(&ld.r, decodeOptions...)
	defer func() {
		ld.r.Reset(nil)
		decoders.Put(ld)
	}()

	now := time.Now()
	var (
		r       otellog.Record
		ts      = now
		attrs   []attribute.KeyValue
		traceID trace.TraceID
		spanID  trace.SpanID
	)
	if tok, err := dec.ReadToken(); err != nil || tok.Kind() != jsontext.KindBeginObject {
		return len(p), nil
	}
	for dec.PeekKind() != jsontext.KindEndObject {
		name, err := dec.ReadToken()
		if err != nil {
			return len(p), nil
		}
		k := name.String() // a token is only valid until the next read
		v, err := readValue(dec)
		if err != nil {
			return len(p), nil
		}
		switch k {
		case zerolog.MessageFieldName:
			r.SetBody(v)
		case zerolog.LevelFieldName:
			if l, err := zerolog.ParseLevel(v.AsString()); err == nil && level == zerolog.NoLevel {
				level = l
			}
		case zerolog.TimestampFieldName:
			if t, err := time.Parse(zerolog.TimeFieldFormat, v.AsString()); err == nil {
				ts = t
			}
		case traceIDField:
			traceID, _ = trace.TraceIDFromHex(v.AsString())
		case spanIDField:
			spanID, _ = trace.SpanIDFromHex(v.AsString())
		default:
			attrs = append(attrs, attribute.KeyValue{Key: attribute.Key(k), Value: v})
		}
	}

	r.SetObservedTimestamp(now)
	r.SetTimestamp(ts)
	r.SetSeverity(severity(level))
	r.SetSeverityText(level.String())
	r.AddAttributes(attrs...)
	ctx := context.Background()
	if traceID.IsValid() && spanID.IsValid() {
		ctx = trace.ContextWithSpanContext(ctx, trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: spanID}))
	}
	w.logger.Emit(ctx, r)
	return len(p), nil
}

// zerolog may write a field twice; the last value wins for the record's own
// fields, and both are kept as attributes.
var (
	decodeOptions = []jsontext.Options{jsontext.AllowDuplicateNames(true), jsontext.AllowInvalidUTF8(true)}
	decoders      = sync.Pool{New: func() any {
		ld := &lineDecoder{}
		ld.dec = jsontext.NewDecoder(&ld.r, decodeOptions...)
		return ld
	}}
)

// lineDecoder is a reusable decoder over one log line.
type lineDecoder struct {
	r   bytes.Reader
	dec *jsontext.Decoder
}

// readValue reads the next JSON value. Integers that fit in int64 stay
// integers, other numbers become floats, except integers too large for
// int64, which are kept exact as strings. Objects become maps, in their
// order; null is the empty value.
func readValue(dec *jsontext.Decoder) (attribute.Value, error) {
	tok, err := dec.ReadToken()
	if err != nil {
		return attribute.Value{}, err
	}
	switch tok.Kind() {
	case jsontext.KindString:
		return attribute.StringValue(tok.String()), nil
	case jsontext.KindNumber:
		return number(tok.String()), nil
	case jsontext.KindTrue, jsontext.KindFalse:
		return attribute.BoolValue(tok.Bool()), nil
	case jsontext.KindBeginArray:
		var vs []attribute.Value
		for dec.PeekKind() != jsontext.KindEndArray {
			v, err := readValue(dec)
			if err != nil {
				return attribute.Value{}, err
			}
			vs = append(vs, v)
		}
		if _, err := dec.ReadToken(); err != nil {
			return attribute.Value{}, err
		}
		return attribute.SliceValue(vs...), nil
	case jsontext.KindBeginObject:
		var kvs []attribute.KeyValue
		for dec.PeekKind() != jsontext.KindEndObject {
			name, err := dec.ReadToken()
			if err != nil {
				return attribute.Value{}, err
			}
			k := name.String()
			v, err := readValue(dec)
			if err != nil {
				return attribute.Value{}, err
			}
			kvs = append(kvs, attribute.KeyValue{Key: attribute.Key(k), Value: v})
		}
		if _, err := dec.ReadToken(); err != nil {
			return attribute.Value{}, err
		}
		return attribute.MapValue(kvs...), nil
	default: // null
		return attribute.Value{}, nil
	}
}

func number(s string) attribute.Value {
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return attribute.Int64Value(i)
	}
	if strings.ContainsAny(s, ".eE") {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return attribute.Float64Value(f)
		}
	}
	return attribute.StringValue(s)
}
