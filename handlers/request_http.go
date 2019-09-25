package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/nicholasjackson/fake-service/client"
	"github.com/nicholasjackson/fake-service/errors"
	"github.com/nicholasjackson/fake-service/response"
	"github.com/nicholasjackson/fake-service/timing"
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
	errorInjector *errors.Injector
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
	errorInjector *errors.Injector,
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
		errorInjector: errorInjector,
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

	resp := &response.Response{}
	resp.Name = rq.name
	resp.Type = "HTTP"

	// are we injecting errors, if so return the error
	if er := rq.errorInjector.Do(); er != nil {
		resp.Code = er.Code
		resp.Error = er.Error.Error()
		serverSpan.LogFields(log.Error(er.Error))

		rw.WriteHeader(er.Code)
		rw.Write([]byte(resp.ToJSON()))
		return
	}

	// if we need to create upstream requests create a worker pool
	var upstreamError error
	if len(rq.upstreamURIs) > 0 {
		wp := worker.New(rq.workerCount, rq.logger, func(uri string) (*response.Response, error) {
			if strings.HasPrefix(uri, "http://") {
				rq.logger.Info("Calling upstream HTTP service", "uri", uri)
				return workerHTTP(serverSpan.Context(), uri, rq.defaultClient, r)
			}

			rq.logger.Info("Calling upstream GRPC service", "uri", uri)
			return workerGRPC(serverSpan.Context(), uri, rq.grpcClients)
		})

		err := wp.Do(rq.upstreamURIs)

		if err != nil {
			upstreamError = err
		}

		for _, v := range wp.Responses() {
			resp.AppendUpstream(v.Response)
		}
	}

	d := rq.duration.Calculate()
	// service time is equal to the randomised time - the current time take
	et := time.Now().Sub(ts)
	rd := d - et

	// randomize the time the request takes if no error
	if upstreamError == nil {
		rq.logger.Info("Upstreams processed correctly")
		sp := serverSpan.Tracer().StartSpan(
			"service_delay",
			opentracing.ChildOf(serverSpan.Context()),
		)
		defer sp.Finish()

		if rd > 0 {
			rq.logger.Info("Sleeping for", "duration", rd.String())
			time.Sleep(rd)
		}
		sp.LogFields(log.String("randomized_duration", d.String()))
		resp.Code = http.StatusOK
	} else {
		rq.logger.Error("Error processing upstreams", "error", upstreamError)
		rw.WriteHeader(http.StatusInternalServerError)
		resp.Code = http.StatusInternalServerError
	}

	et = time.Now().Sub(ts)
	resp.Duration = et.String()

	// add the response body
	resp.Body = rq.message

	rw.Write([]byte(resp.ToJSON()))

	rq.logger.Info("Service Duration", "elapsed_time", et.String(), "calculated_duration", d.String(), "sleep_time", rd.String())
}
