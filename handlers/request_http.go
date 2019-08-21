package handlers

import (
	"fmt"
	"net/http"
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
	grpcClients   map[string]client.GRPC
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
	grpcClients map[string]client.GRPC,
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
		grpcClients:   grpcClients,
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
			return workerHTTP(serverSpan.Context(), uri, rq.defaultClient)
		})

		err := wp.Do(rq.upstreamURIs)

		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		data = append(data, processResponses(wp.Responses())...)
	}

	rw.Write(data)
}
