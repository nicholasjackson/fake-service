package client

import (
	"context"

	"github.com/nicholasjackson/fake-service/grpc/api"
)

// GRPC defines the interface for a GRPC client
type GRPC interface {
	Handle(context.Context, *api.Nil) (*api.Response, error)
}

// GRPCImpl is the concrete implementation of the GRPC client
type GRPCImpl struct {
	client api.FakeServiceClient
}

// Handle calls the upstream client
func (c *GRPCImpl) Handle(ctx context.Context, n *api.Nil) (*api.Response, error) {
	return c.client.Handle(ctx, n)
}
