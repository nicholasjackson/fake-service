package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/nicholasjackson/fake-service/client"
	"github.com/nicholasjackson/fake-service/timing"
	"github.com/nicholasjackson/fake-service/tracing"
	"github.com/nicholasjackson/fake-service/worker"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

// done is a message sent when an upstream worker has completed
type done struct {
	uri  string
	data []byte
}

// Request handles inbound requests and makes any necessary upstream calls
type Request struct {
	// name of the service
	name string
	// message to return to caller
	message       string
	logger        hclog.Logger
	duration      *timing.RequestDuration
	upstreamURIs  []string
	workerCount   int
	defaultClient client.HTTP
	tracingClient tracing.Client
}

// NewRequest creates a new request handler
func NewRequest(
	name, message string,
	logger hclog.Logger,
	duration *timing.RequestDuration,
	upstreamURIs []string,
	workerCount int,
	defaultClient client.HTTP,
	tracingClient tracing.Client,
) *Request {

	return &Request{
		name:          name,
		message:       message,
		logger:        logger,
		duration:      duration,
		upstreamURIs:  upstreamURIs,
		workerCount:   workerCount,
		defaultClient: defaultClient,
		tracingClient: tracingClient,
	}
}

// Handle the request and call the upstream servers
func (rq *Request) Handle(rw http.ResponseWriter, r *http.Request) {
	rq.logger.Info("Handling request", "request", formatRequest(r))

	var serverSpan opentracing.Span
	wireContext, err := opentracing.GlobalTracer().Extract(
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(r.Header),
	)
	if err != nil {
		// Optionally record something about err here
		rq.logger.Error("Error obtaining context", "error", err)
	}

	// Create the span referring to the RPC client if available.
	// If wireContext == nil, a root span will be created.
	serverSpan = opentracing.StartSpan(
		"handle_request",
		ext.RPCServerOption(wireContext))

	defer serverSpan.Finish()

	// randomize the time the request takes
	d := rq.duration.Calculate()
	sp := serverSpan.Tracer().StartSpan(
		"service_delay",
		opentracing.ChildOf(serverSpan.Context()),
	)

	// wait for a predetermined time
	time.Sleep(d)

	sp.Finish()

	data := []byte(fmt.Sprintf("# Reponse from: %s #\n%s\n", rq.name, rq.message))
	// if we need to create upstream requests create a worker pool
	if len(rq.upstreamURIs) > 0 {
		wp := worker.New(rq.workerCount, rq.logger, func(uri string) (string, error) {
			return rq.workerHTTP(serverSpan.Context(), uri)
		})

		err := wp.Do(rq.upstreamURIs)

		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		data = append(data, rq.processResponses(wp.Responses())...)
	}

	rw.Write(data)
}

func (rq *Request) workerHTTP(ctx opentracing.SpanContext, uri string) (string, error) {
	httpReq, _ := http.NewRequest("GET", uri, nil)

	clientSpan := opentracing.StartSpan(
		"call_upstream",
		opentracing.ChildOf(ctx),
	)

	ext.SpanKindRPCClient.Set(clientSpan)
	ext.HTTPUrl.Set(clientSpan, uri)
	ext.HTTPMethod.Set(clientSpan, "GET")

	// Transmit the span's TraceContext as HTTP headers on our
	// outbound request.
	opentracing.GlobalTracer().Inject(
		clientSpan.Context(),
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(httpReq.Header))

	resp, err := rq.defaultClient.Do(httpReq)
	clientSpan.Finish()

	if err != nil {
		return "", err
	}

	return string(resp), nil
}

func (rq Request) processResponses(responses []worker.Done) []byte {
	respLines := []string{}

	// append the output from the upstreams
	for _, r := range responses {
		respLines = append(respLines, fmt.Sprintf("## Called upstream uri: %s", r.URI))
		// indent the reposne from the upstream
		lines := strings.Split(r.Message, "\n")
		for _, l := range lines {
			respLines = append(respLines, fmt.Sprintf("  %s", l))
		}
	}

	return []byte(strings.Join(respLines, "\n"))
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
