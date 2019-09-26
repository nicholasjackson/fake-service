package tracing

import (
	"github.com/opentracing/opentracing-go"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/opentracer"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

func NewDataDogClient(uri string) {
	t := opentracer.New(tracer.WithAgentAddr(uri))

	opentracing.SetGlobalTracer(t)
}
