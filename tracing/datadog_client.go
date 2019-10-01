package tracing

import (
	"fmt"

	"github.com/opentracing/opentracing-go"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/opentracer"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

func NewDataDogClient(uri, name string) {
	t := opentracer.New(tracer.WithAgentAddr(uri), tracer.WithServiceName(name), tracer.WithAnalytics(true))

	opentracing.SetGlobalTracer(t)
}

func GetDataDogSpanDetails(ctx opentracing.SpanContext) *SpanDetails {
	if s, ok := ctx.(ddtrace.SpanContext); ok {
		return &SpanDetails{
			SpanID:  fmt.Sprint(s.SpanID()),
			TraceID: fmt.Sprint(s.TraceID()),
		}
	}

	return nil
}
