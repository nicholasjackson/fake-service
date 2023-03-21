package client

import (
	"context"
	"strings"
	"time"

	"github.com/nicholasjackson/fake-service/grpc/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// GRPC defines the interface for a GRPC client
type GRPC interface {
	Handle(context.Context, *api.Request) (*api.Response, map[string]string, error)
}

// NewGRPC creates a new GRPC client
func NewGRPC(uri string, timeout time.Duration) (GRPC, error) {

	conn, err := grpc.Dial(
		uri,
		grpc.WithInsecure(),
		grpc.WithTimeout(timeout),
	)

	if err != nil {
		return nil, err
	}

	return &GRPCImpl{api.NewFakeServiceClient(conn)}, nil
}

// GRPCImpl is the concrete implementation of the GRPC client
type GRPCImpl struct {
	client api.FakeServiceClient
}

// Handle calls the upstream client
func (c *GRPCImpl) Handle(ctx context.Context, n *api.Request) (*api.Response, map[string]string, error) {
	var header, trailer metadata.MD

	resp, err := c.client.Handle(
		ctx, n,
		grpc.Header(&header),
		grpc.Trailer(&trailer),
	)

	// get the metadata an add it to the map
	headers := map[string]string{}
	for k, v := range header {
		headers[k] = strings.Join(v, ",")
	}

	for k, v := range trailer {
		headers[k] = strings.Join(v, ",")
	}

	return resp, headers, err
}
