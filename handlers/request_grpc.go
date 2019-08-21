package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/nicholasjackson/fake-service/client"
	"github.com/nicholasjackson/fake-service/grpc/api"
	"github.com/nicholasjackson/fake-service/timing"
	"github.com/nicholasjackson/fake-service/worker"
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
	f.logger.Info("Handling request gRPC request")

	data := []byte(fmt.Sprintf("# Reponse from: %s #\n%s\n", f.name, f.message))

	// if we need to create upstream requests create a worker pool
	if len(f.upstreamURIs) > 0 {
		wp := worker.New(f.workerCount, f.logger, func(uri string) (string, error) {
			if strings.HasPrefix(uri, "http://") {
				return workerHTTP(nil, uri, f.defaultClient)
			}

			return workerGRPC(nil, uri, f.grpcClients)
		})

		err := wp.Do(f.upstreamURIs)

		if err != nil {
			f.logger.Error("Error making upstream call", "error", err)
			return nil, err
		}

		data = append(data, processResponses(wp.Responses())...)
	}

	return &api.Response{Message: string(data)}, nil
}
