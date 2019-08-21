package handlers

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-hclog"
	"github.com/nicholasjackson/fake-service/client"
	"github.com/nicholasjackson/fake-service/grpc/api"
	"github.com/nicholasjackson/fake-service/timing"
	"github.com/nicholasjackson/fake-service/worker"
	"google.golang.org/grpc"
)

// FakeServer implements the gRPC interface
type FakeServer struct {
	name          string
	message       string
	duration      *timing.RequestDuration
	upstreamURIs  []string
	workerCount   int
	defaultClient client.HTTP
	logger        hclog.Logger
}

// NewFakeServer creates a new instance of FakeServer
func NewFakeServer(
	name, message string,
	duration *timing.RequestDuration,
	upstreamURIs []string,
	workerCount int,
	defaultClient client.HTTP,
	l hclog.Logger) *FakeServer {

	return &FakeServer{
		name:          name,
		message:       message,
		duration:      duration,
		upstreamURIs:  upstreamURIs,
		workerCount:   workerCount,
		defaultClient: defaultClient,
		logger:        l,
	}
}

// Handle implmements the FakeServer Handle interface method
func (f *FakeServer) Handle(ctx context.Context, in *api.Nil, opts ...grpc.CallOption) (*api.Response, error) {
	f.logger.Info("Handling request", "request", in.String())

	data := []byte(fmt.Sprintf("# Reponse from: %s #\n%s\n", f.name, f.message))
	// if we need to create upstream requests create a worker pool
	if len(f.upstreamURIs) > 0 {
		wp := worker.New(f.workerCount, f.logger, func(uri string) (string, error) {
			return workerHTTP(nil, uri, f.defaultClient)
		})

		err := wp.Do(f.upstreamURIs)

		if err != nil {
			return nil, err
		}

		data = append(data, processResponses(wp.Responses())...)
	}

	return &api.Response{Message: string(data)}, nil
}
