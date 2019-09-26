package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/nicholasjackson/fake-service/client"
	"github.com/nicholasjackson/fake-service/errors"
	"github.com/nicholasjackson/fake-service/grpc/api"
	"github.com/nicholasjackson/fake-service/response"
	"github.com/nicholasjackson/fake-service/timing"
	"github.com/nicholasjackson/fake-service/worker"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// FakeServer implements the gRPC interface
type FakeServer struct {
	name          string
	message       string
	duration      *timing.RequestDuration
	upstreamURIs  []string
	workerCount   int
	defaultClient client.HTTP
	grpcClients   map[string]client.GRPC
	logger        hclog.Logger
	errorInjector *errors.Injector
}

// NewFakeServer creates a new instance of FakeServer
func NewFakeServer(
	name, message string,
	duration *timing.RequestDuration,
	upstreamURIs []string,
	workerCount int,
	defaultClient client.HTTP,
	grpcClients map[string]client.GRPC,
	l hclog.Logger,
	i *errors.Injector,
) *FakeServer {

	return &FakeServer{
		name:          name,
		message:       message,
		duration:      duration,
		upstreamURIs:  upstreamURIs,
		workerCount:   workerCount,
		defaultClient: defaultClient,
		grpcClients:   grpcClients,
		logger:        l,
		errorInjector: i,
	}
}

// Handle implmements the FakeServer Handle interface method
func (f *FakeServer) Handle(ctx context.Context, in *api.Nil) (*api.Response, error) {
	f.logger.Info("Handling request gRPC request", "context", printContext(ctx))

	// start timing the service this is used later for the total request time
	ts := time.Now()

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
		f.logger.Error("Error obtaining context, creating new span", "error", err)
	}

	// Create the span referring to the RPC client if available.
	// If wireContext == nil, a root span will be created.
	serverSpan = opentracing.StartSpan(
		"handle_request",
		ext.RPCServerOption(wireContext))

	serverSpan.LogFields(log.String("service.type", "grpc"))

	defer serverSpan.Finish()

	resp := &response.Response{}
	resp.Name = f.name
	resp.Type = "gRPC"

	// are we injecting errors, if so return the error
	if er := f.errorInjector.Do(); er != nil {
		resp.Code = er.Code
		resp.Error = er.Error.Error()
		serverSpan.LogFields(log.Error(er.Error))

		// return the error
		return &api.Response{Message: resp.ToJSON()}, status.New(codes.Code(resp.Code), er.Error.Error()).Err()
	}

	// if we need to create upstream requests create a worker pool
	var upstreamError error
	if len(f.upstreamURIs) > 0 {
		wp := worker.New(f.workerCount, f.logger, func(uri string) (*response.Response, error) {
			if strings.HasPrefix(uri, "http://") {
				return workerHTTP(serverSpan.Context(), uri, f.defaultClient, nil)
			}

			return workerGRPC(serverSpan.Context(), uri, f.grpcClients)
		})

		err := wp.Do(f.upstreamURIs)

		for _, v := range wp.Responses() {
			resp.AppendUpstream(v.Response)
		}

		if err != nil {
			f.logger.Error("Error making upstream call", "error", err)
			upstreamError = err
		}
	}

	et := time.Now().Sub(ts)
	resp.Duration = et.String()

	if upstreamError != nil {
		resp.Code = int(codes.Internal)
		resp.Error = upstreamError.Error()

		f.logger.Error("Service resulted in error, returning response", "response", resp)

		return &api.Response{Message: resp.ToJSON()}, status.New(codes.Internal, upstreamError.Error()).Err()
	}

	// randomize the time the request takes
	d := f.duration.Calculate()
	sp := serverSpan.Tracer().StartSpan(
		"service_delay",
		opentracing.ChildOf(serverSpan.Context()),
	)
	defer sp.Finish()

	// service time is equal to the randomised time - the current time take
	et = time.Now().Sub(ts)
	rd := d - et

	f.logger.Info("Service Duration", "elapsed_time", et.String(), "calculated_duration", d.String(), "sleep_time", rd.String())
	sp.LogFields(log.String("randomized_duration", d.String()))

	if rd > 0 {
		f.logger.Info("Sleeping for", "duration", rd.String())
		time.Sleep(rd)
	}

	// add the response body if there is no upstream error
	if upstreamError == nil {
		resp.Body = f.message
	}

	return &api.Response{Message: resp.ToJSON()}, nil
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
