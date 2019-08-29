package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/nicholasjackson/fake-service/client"
	"github.com/nicholasjackson/fake-service/grpc/api"
	"github.com/nicholasjackson/fake-service/timing"
	"github.com/nicholasjackson/fake-service/worker"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/log"
	"google.golang.org/grpc/metadata"
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
}

// NewFakeServer creates a new instance of FakeServer
func NewFakeServer(
	name, message string,
	duration *timing.RequestDuration,
	upstreamURIs []string,
	workerCount int,
	defaultClient client.HTTP,
	grpcClients map[string]client.GRPC,
	l hclog.Logger) *FakeServer {

	return &FakeServer{
		name:          name,
		message:       message,
		duration:      duration,
		upstreamURIs:  upstreamURIs,
		workerCount:   workerCount,
		defaultClient: defaultClient,
		grpcClients:   grpcClients,
		logger:        l,
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

	data := []byte(fmt.Sprintf("# Reponse from: %s #\n%s\n", f.name, f.message))

	// if we need to create upstream requests create a worker pool
	if len(f.upstreamURIs) > 0 {
		wp := worker.New(f.workerCount, f.logger, func(uri string) (string, error) {
			if strings.HasPrefix(uri, "http://") {
				return workerHTTP(serverSpan.Context(), uri, f.defaultClient, nil)
			}

			return workerGRPC(serverSpan.Context(), uri, f.grpcClients)
		})

		err := wp.Do(f.upstreamURIs)

		if err != nil {
			f.logger.Error("Error making upstream call", "error", err)
			return nil, err
		}

		data = append(data, processResponses(wp.Responses())...)
	}

	// randomize the time the request takes
	d := f.duration.Calculate()
	sp := serverSpan.Tracer().StartSpan(
		"service_delay",
		opentracing.ChildOf(serverSpan.Context()),
	)
	defer sp.Finish()

	// service time is equal to the randomised time - the current time take
	et := time.Now().Sub(ts)
	rd := d - et

	f.logger.Info("Service Duration", "elapsed_time", et.String(), "calculated_duration", d.String(), "sleep_time", rd.String())
	sp.LogFields(log.String("randomized_duration", d.String()))

	if rd > 0 {
		f.logger.Info("Sleeping for", "duration", rd.String())
		time.Sleep(rd)
	}

	return &api.Response{Message: string(data)}, nil
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
