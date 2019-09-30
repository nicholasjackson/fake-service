package tracing

import (
	"github.com/opentracing/opentracing-go"
)

type SpanDetails struct {
	SpanID  string
	TraceID string
}

type SpanDetailsFunc func(ctx opentracing.SpanContext) *SpanDetails
