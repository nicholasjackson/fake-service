package tracing

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/opentracing/opentracing-go"
	zipkinot "github.com/openzipkin-contrib/zipkin-go-opentracing"
	"github.com/openzipkin/zipkin-go"

	"github.com/openzipkin/zipkin-go/reporter"
	zipkinhttp "github.com/openzipkin/zipkin-go/reporter/http"
	logreporter "github.com/openzipkin/zipkin-go/reporter/log"
)

// Client implements a tracing client
type Client interface {
	StartSpanFromContext(context.Context, string) (opentracing.Span, context.Context)
	StartSpan(string, ...opentracing.StartSpanOption) opentracing.Span
}

// OpenTracingClient is an implementation of an open tracing client
type OpenTracingClient struct {
}

// NewOpenTracingClient creates a new open tracing client
func NewOpenTracingClient(uri, name, serviceURI string) Client {
	var reporter reporter.Reporter

	if strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://") {
		reporter = zipkinhttp.NewReporter(fmt.Sprintf("%s/api/v2/spans", uri))
	} else {
		reporter = logreporter.NewReporter(log.New(os.Stderr, "", log.LstdFlags))
	}

	// create our local service endpoint
	endpoint, err := zipkin.NewEndpoint(name, serviceURI)
	if err != nil {
		log.Fatalf("unable to create local endpoint: %s, error: %+v\n", serviceURI, err)
	}

	// initialize our tracer
	nativeTracer, err := zipkin.NewTracer(reporter, zipkin.WithLocalEndpoint(endpoint))
	if err != nil {
		log.Fatalf("unable to create tracer: %+v\n", err)
	}

	// use zipkin-go-opentracing to wrap our tracer
	tracer := zipkinot.Wrap(nativeTracer)

	// optionally set as Global OpenTracing tracer instance
	opentracing.SetGlobalTracer(tracer)

	return &OpenTracingClient{}
}

// StartSpanFromContext creates a new span from the given context
func (otc *OpenTracingClient) StartSpanFromContext(ctx context.Context, operation string) (opentracing.Span, context.Context) {
	return opentracing.StartSpanFromContext(ctx, operation)
}

// StartSpan creates a new root span
func (otc *OpenTracingClient) StartSpan(operation string, opts ...opentracing.StartSpanOption) opentracing.Span {
	return opentracing.StartSpan(operation, opts...)
}

func GetZipkinSpanDetails(ctx opentracing.SpanContext) *SpanDetails {
	if s, ok := ctx.(zipkinot.SpanContext); ok {
		return &SpanDetails{
			SpanID:  s.ID.String(),
			TraceID: s.TraceID.String(),
		}
	}

	return nil
}
