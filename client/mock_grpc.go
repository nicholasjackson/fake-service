package client

import (
	"context"

	"github.com/nicholasjackson/fake-service/grpc/api"
	"github.com/stretchr/testify/mock"
)

// MockGRPC is a mock implementation of the GRPC client
type MockGRPC struct {
	mock.Mock
}

// Handle calls the upstream client
func (m *MockGRPC) Handle(ctx context.Context, n *api.Nil) (*api.Response, error) {
	args := m.Called(ctx, n)

	if a := args.Get(0); a != nil {
		return a.(*api.Response), nil
	}

	return nil, args.Error(1)
}
