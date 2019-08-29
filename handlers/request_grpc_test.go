package handlers

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/nicholasjackson/fake-service/client"
	"github.com/nicholasjackson/fake-service/grpc/api"
	"github.com/nicholasjackson/fake-service/timing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupFakeServer(t *testing.T, uris []string) (*FakeServer, *client.MockHTTP, map[string]client.GRPC) {
	l := hclog.Default()
	c := &client.MockHTTP{}
	d := timing.NewRequestDuration(
		1*time.Nanosecond,
		1*time.Nanosecond,
		1*time.Nanosecond,
		0)

	// if we have any grpc apis create the clients
	grpcClients := make(map[string]client.GRPC)
	for _, u := range uris {
		if strings.HasPrefix(u, "grpc://") {
			c := &client.MockGRPC{}
			grpcClients[u] = c
		}
	}

	return NewFakeServer("test", "hello world", d, uris, 1, c, grpcClients, l), c, grpcClients
}

func TestGRPCServiceHandlesRequestWithNoUpstream(t *testing.T) {
	fs, _, _ := setupFakeServer(t, nil)

	resp, err := fs.Handle(context.Background(), nil)

	assert.Nil(t, err)
	assert.Equal(t, "# Reponse from: test #\nhello world\n", resp.Message)
}

func TestGRPCServiceHandlesRequestWithHTTPUpstream(t *testing.T) {
	uris := []string{"http://test.com"}
	fs, mc, _ := setupFakeServer(t, uris)
	mc.On("Do", mock.Anything, mock.Anything).Return([]byte("# Response from: upstream #\nOK\n"), nil)

	resp, err := fs.Handle(context.Background(), nil)

	assert.Nil(t, err)
	mc.AssertCalled(t, "Do", mock.Anything, mock.Anything)
	assert.Equal(t, "# Reponse from: test #\nhello world\n## Called upstream uri: http://test.com\n  # Response from: upstream #\n  OK\n  ", resp.Message)
}

func TestGRPCServiceHandlesRequestWithGRPCUpstream(t *testing.T) {
	uris := []string{"grpc://test.com"}
	fs, _, gc := setupFakeServer(t, uris)

	gcMock := gc["grpc://test.com"].(*client.MockGRPC)
	gcMock.On("Handle", mock.Anything, mock.Anything).Return(&api.Response{Message: "# Response from: upstream #\nOK\n"}, nil)

	resp, err := fs.Handle(context.Background(), nil)

	assert.Nil(t, err)
	gcMock.AssertCalled(t, "Handle", mock.Anything, mock.Anything)
	assert.Equal(t, "# Reponse from: test #\nhello world\n## Called upstream uri: grpc://test.com\n  # Response from: upstream #\n  OK\n  ", resp.Message)
}
