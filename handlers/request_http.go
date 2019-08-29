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
	"github.com/opentracing/opentracing-go/log"
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

	// start timing the service this is used later for the total request time
	ts := time.Now()

	var serverSpan opentracing.Span
	wireContext, err := opentracing.GlobalTracer().Extract(
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(r.Header),
	)
	if err != nil {
		// Optionally record something about err here
		rq.logger.Error("Error obtaining context, creating new span", "error", err)
	}

	// Create the span referring to the RPC client if available.
	// If wireContext == nil, a root span will be created.
	serverSpan = opentracing.StartSpan(
		"handle_request",
		ext.RPCServerOption(wireContext))
	serverSpan.LogFields(log.String("service.type", "http"))

	defer serverSpan.Finish()

	data := []byte(fmt.Sprintf("# Reponse from: %s #\n%s\n", rq.name, rq.message))
	// if we need to create upstream requests create a worker pool
	if len(rq.upstreamURIs) > 0 {
		wp := worker.New(rq.workerCount, rq.logger, func(uri string) (string, error) {
			if strings.HasPrefix(uri, "http://") {
				rq.logger.Info("Calling upstream HTTP service", "uri", uri)
				return workerHTTP(serverSpan.Context(), uri, rq.defaultClient, r)
			}

			rq.logger.Info("Calling upstream HTTP service", "uri", uri)
			return workerGRPC(serverSpan.Context(), uri, rq.grpcClients)
		})

		err := wp.Do(rq.upstreamURIs)

		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		data = append(data, processResponses(wp.Responses())...)
	}

	// randomize the time the request takes
	d := rq.duration.Calculate()
	sp := serverSpan.Tracer().StartSpan(
		"service_delay",
		opentracing.ChildOf(serverSpan.Context()),
	)
	defer sp.Finish()

	rw.Write(data)

	// service time is equal to the randomised time - the current time take
	et := time.Now().Sub(ts)
	rd := d - et

	rq.logger.Info("Service Duration", "elapsed_time", et.String(), "calculated_duration", d.String(), "sleep_time", rd.String())
	sp.LogFields(log.String("randomized_duration", d.String()))

	if rd > 0 {
		rq.logger.Info("Sleeping for", "duration", rd.String())
		time.Sleep(rd)
	}
}
