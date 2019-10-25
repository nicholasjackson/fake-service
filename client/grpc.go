package client

import (
	"context"
	"time"

	"github.com/nicholasjackson/fake-service/grpc/api"
	"google.golang.org/grpc"
)

// GRPC defines the interface for a GRPC client
type GRPC interface {
	Handle(context.Context, *api.Nil) (*api.Response, error)
}

// NewGRPC creates a new GRPC client
func NewGRPC(uri string, timeout time.Duration) (GRPC, error) {
	conn, err := grpc.Dial(uri, grpc.WithInsecure(), grpc.WithTimeout(timeout))
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
func (c *GRPCImpl) Handle(ctx context.Context, n *api.Nil) (*api.Response, error) {
	return c.client.Handle(ctx, n)
}
