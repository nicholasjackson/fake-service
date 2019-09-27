package logging

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/log"
	"google.golang.org/grpc/metadata"
)

type Logger struct {
	metrics Metrics
	log     hclog.Logger
}

func NewLogger(m Metrics, l hclog.Logger) *Logger {
	return &Logger{
		metrics: m,
		log:     l,
	}
}

// LogProcess is returned from a logging function
type LogProcess struct {
	finished func(err error, meta map[string]string)
	err      error
	metadata map[string]string
	Span     opentracing.Span
}

// SetError for the current operation
func (l *LogProcess) SetError(err error) {
	l.err = err
}

// SetMetadata for the process
func (l *LogProcess) SetMetadata(key, value string) {
	if l.metadata == nil {
		l.metadata = map[string]string{}
	}

	l.metadata[key] = value
}

// Finished operation
func (l *LogProcess) Finished() {
	l.finished(l.err, l.metadata)
}

func (l *Logger) Log() hclog.Logger {
	return l.log
}

// HandleHTTPRequest creates the request span and timing metrics for the handler
func (l *Logger) HandleHTTPRequest(r *http.Request) *LogProcess {
	l.log.Info("Handle inbound request", "request", formatRequest(r))
	// create the start time
	st := time.Now()

	// attempt to create a span using a parent span defined in http headers
	var serverSpan opentracing.Span
	wireContext, err := opentracing.GlobalTracer().Extract(
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(r.Header),
	)

	if err != nil {
		// if there is no span in the headers an error will be raised, log
		// this error
		l.log.Debug("Error obtaining context, creating new span", "error", err)
	}

	// Create the span referring to the RPC client if available.
	// If wireContext == nil, a root span will be created.
	serverSpan = opentracing.StartSpan(
		"handle_request",
		ext.RPCServerOption(wireContext))
	serverSpan.LogFields(log.String("service.type", "http"))

	// return an object which can be used to set metadata onto the trace and
	// complete
	return &LogProcess{
		finished: func(err error, meta map[string]string) {
			te := time.Now()

			// if there was an error add this to the trace
			// and log
			if err != nil {
				serverSpan.LogFields(log.Error(err))
				l.log.Error("Error handling request", "error", err)
			}

			// add metadata to the trace and stats
			for k, v := range meta {
				serverSpan.SetTag(k, v)
			}

			serverSpan.Finish()
			l.metrics.Timing("handle.request.http", te.Sub(st), getTags(err, meta))
		},
		Span: serverSpan,
	}
}

func (l *Logger) HandleGRCPRequest(ctx context.Context) *LogProcess {
	l.log.Info("Handling request gRPC request", "context", printContext(ctx))
	st := time.Now()

	// we need to convert the metadata to a httpRequest to extract the span
	md, _ := metadata.FromIncomingContext(ctx)
	r := grpcMetaDataToHTTPRequest(md)

	var serverSpan opentracing.Span
	wireContext, err := opentracing.GlobalTracer().Extract(
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(r.Header),
	)

	if err != nil {
		// Optionally record something about err here
		l.log.Debug("Error obtaining context, creating new span", "error", err)
	}

	// Create the span referring to the RPC client if available.
	// If wireContext == nil, a root span will be created.
	serverSpan = opentracing.StartSpan(
		"handle_request",
		ext.RPCServerOption(wireContext))

	serverSpan.LogFields(log.String("service.type", "grpc"))

	return &LogProcess{
		finished: func(err error, meta map[string]string) {
			te := time.Now()

			// if there was an error add this to the trace
			// and log
			if err != nil {
				serverSpan.LogFields(log.Error(err))
				l.log.Error("Error handling request", "error", err)
			}

			// add metadata to the trace and stats
			for k, v := range meta {
				serverSpan.SetTag(k, v)
			}

			serverSpan.Finish()
			l.metrics.Timing("handle.request.grpc", te.Sub(st), getTags(err, meta))
		},
		Span: serverSpan,
	}
}

// Logs data about service duration simulation
func (l *Logger) SleepService(parentSpan opentracing.Span, d time.Duration) *LogProcess {
	sp := parentSpan.Tracer().StartSpan(
		"service_delay",
		opentracing.ChildOf(parentSpan.Context()),
	)

	l.log.Info("Sleeping for", "duration", d.String())
	sp.LogFields(log.String("randomized_duration", d.String()))

	return &LogProcess{
		finished: func(err error, meta map[string]string) {
			sp.Finish()
		},
		Span: sp,
	}
}

// Logs data regarding upstream http requests
func (l *Logger) CallHTTPUpstream(parentRequest *http.Request, upstreamRequest *http.Request, ctx opentracing.SpanContext) *LogProcess {
	l.log.Info("Calling upstream service", "uri", upstreamRequest.URL.String(), "type", "HTTP", "request", formatRequest(upstreamRequest))

	st := time.Now()

	clientSpan := opentracing.StartSpan(
		"call_upstream",
		opentracing.ChildOf(ctx),
	)

	clientSpan.LogFields(log.String("upstream.type", "http"))

	ext.SpanKindRPCClient.Set(clientSpan)
	ext.HTTPUrl.Set(clientSpan, upstreamRequest.URL.String())
	ext.HTTPMethod.Set(clientSpan, upstreamRequest.Method)

	// Transmit the span's TraceContext as HTTP headers on our
	// outbound request.
	opentracing.GlobalTracer().Inject(
		clientSpan.Context(),
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(upstreamRequest.Header))

	return &LogProcess{
		finished: func(err error, meta map[string]string) {
			te := time.Now()

			// if there was an error add this to the trace
			// and log
			if err != nil {
				clientSpan.LogFields(log.Error(err))
				l.log.Error("Error processing upstream request", "error", err)
			}

			// add metadata to the trace and stats
			for k, v := range meta {
				clientSpan.SetTag(k, v)
			}

			l.metrics.Timing("upstream.request.http", te.Sub(st), getTags(err, meta))
			clientSpan.Finish()
		},
	}
}

// Logs data regarding upstream grpc requests
// returns a context containing span context for tracing
func (l *Logger) CallGRCPUpstream(uri string, ctx opentracing.SpanContext) (*LogProcess, context.Context) {
	l.log.Info("Calling upstream service", "uri", uri, "type", "gRPC")

	st := time.Now()

	clientSpan := opentracing.StartSpan(
		"call_upstream",
		opentracing.ChildOf(ctx),
	)
	ext.SpanKindRPCClient.Set(clientSpan)

	// add the upstream type
	clientSpan.LogFields(log.String("upstream.type", "grpc"))

	req := &http.Request{Header: http.Header{}}
	opentracing.GlobalTracer().Inject(
		clientSpan.Context(),
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(req.Header))

	// create the grpc metadata and inject the client span
	md := httpRequestToGrpcMetadata(req)

	outCtx := metadata.NewOutgoingContext(context.Background(), md)

	return &LogProcess{
		finished: func(err error, meta map[string]string) {

			te := time.Now()

			// if there was an error add this to the trace
			// and log
			if err != nil {
				clientSpan.LogFields(log.Error(err))
				l.log.Error("Error processing upstream request", "error", err)
			}

			// add metadata to the trace and stats
			for k, v := range meta {
				clientSpan.SetTag(k, v)
			}

			l.metrics.Timing("upstream.request.grpc", te.Sub(st), getTags(err, meta))
		},
	}, outCtx
}

func (l *Logger) CallHealthHTTP() *LogProcess {
	st := time.Now()
	l.log.Info("Handling health request")

	return &LogProcess{
		finished: func(err error, meta map[string]string) {
			te := time.Now()
			l.metrics.Timing("handle.health.http", te.Sub(st), getTags(err, meta))
		},
	}
}

func getTags(err error, meta map[string]string) []string {
	tags := []string{}

	for k, v := range meta {
		tags = append(tags, fmt.Sprintf("%s:%s", k, v))
	}

	if err != nil {
		tags = append(tags, "error:true")
	}

	return tags
}

// formatRequest generates ascii representation of a request
func formatRequest(r *http.Request) string {
	// Create return string
	var request []string
	// Add the request string
	url := fmt.Sprintf("%v %v %v", r.Method, r.URL, r.Proto)
	request = append(request, url)
	// Add the host
	request = append(request, fmt.Sprintf("Host: %v", r.Host))
	// Loop through headers
	for name, headers := range r.Header {
		name = strings.ToLower(name)
		for _, h := range headers {
			request = append(request, fmt.Sprintf("%v: %v", name, h))
		}
	}

	// If this is a POST, add post data
	if r.Method == "POST" {
		r.ParseForm()
		request = append(request, "\n")
		request = append(request, r.Form.Encode())
	}
	// Return the request as a string
	return strings.Join(request, "\n")
}

// the following two functions are a hack to get round that
// opentracing zipkin can not deal with grpc metadata for
// Inject and extract
func grpcMetaDataToHTTPRequest(md metadata.MD) *http.Request {
	h := http.Header{}
	for k, v := range md {
		for _, vv := range v {
			h.Add(k, vv)
		}
	}
	return &http.Request{Header: h}
}

func httpRequestToGrpcMetadata(r *http.Request) metadata.MD {
	md := metadata.MD{}

	for k, v := range r.Header {
		md.Set(k, v...)
	}

	return md
}

func printContext(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "No metadata in context"
	}

	ret := ""
	for k, v := range md {
		ret += fmt.Sprintf("key: %s value: %s\n", k, v)
	}

	return ret
}
