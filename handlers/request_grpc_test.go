package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/nicholasjackson/fake-service/client"
	"github.com/nicholasjackson/fake-service/errors"
	"github.com/nicholasjackson/fake-service/grpc/api"
	"github.com/nicholasjackson/fake-service/response"
	"github.com/nicholasjackson/fake-service/timing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func setupFakeServer(t *testing.T, uris []string, errorRate float64) (*FakeServer, *client.MockHTTP, map[string]client.GRPC) {
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

	// setup the error injector
	i := errors.NewInjector(l, errorRate, int(codes.Internal), "http_error", 0, 0, 0)

	return NewFakeServer("test", "hello world", d, uris, 1, c, grpcClients, l, i), c, grpcClients
}

func TestGRPCServiceHandlesRequestWithNoUpstream(t *testing.T) {
	fs, _, _ := setupFakeServer(t, nil, 0)

	resp, err := fs.Handle(context.Background(), nil)
	mr := response.Response{}
	mr.FromJSON([]byte(resp.Message))

	assert.Nil(t, err)
	assert.Equal(t, "test", mr.Name)
	assert.Equal(t, "hello world", mr.Body)
	assert.Len(t, mr.UpstreamCalls, 0)
}

func TestGRPCServiceHandlesErrorInjection(t *testing.T) {
	fs, _, _ := setupFakeServer(t, nil, 1)

	resp, err := fs.Handle(context.Background(), nil)
	status, ok := status.FromError(err)

	mr := response.Response{}
	mr.FromJSON([]byte(resp.Message))

	assert.Error(t, err)
	assert.True(t, ok)
	assert.Equal(t, "test", mr.Name)
	assert.Equal(t, "hello world", mr.Body)
	assert.Equal(t, codes.Internal, status.Code())
	assert.Len(t, mr.UpstreamCalls, 0)
}

func TestGRPCServiceHandlesRequestWithHTTPUpstream(t *testing.T) {
	uris := []string{"http://test.com"}
	fs, mc, _ := setupFakeServer(t, uris, 0)
	mc.On("Do", mock.Anything, mock.Anything).Return(http.StatusInternalServerError, []byte(`{"name": "upstream", "error": "boom", "code": 500}`), fmt.Errorf("It went bang"))

	resp, err := fs.Handle(context.Background(), nil)

	assert.Error(t, err)
	mc.AssertCalled(t, "Do", mock.Anything, mock.Anything)
	mr := response.Response{}
	mr.FromJSON([]byte(resp.Message))

	assert.Equal(t, "test", mr.Name)
	assert.Equal(t, "hello world", mr.Body)
	assert.Equal(t, 13, mr.Code)
	assert.Len(t, mr.UpstreamCalls, 1)
	assert.Equal(t, "upstream", mr.UpstreamCalls[0].Name)
	assert.Equal(t, 500, mr.UpstreamCalls[0].Code)
}

func TestGRPCServiceHandlesRequestWithHTTPUpstreamError(t *testing.T) {
	uris := []string{"http://test.com"}
	fs, mc, _ := setupFakeServer(t, uris, 0)
	mc.On("Do", mock.Anything, mock.Anything).Return(http.StatusOK, []byte(`{"name": "upstream", "body": "OK"}`), nil)

	resp, err := fs.Handle(context.Background(), nil)

	assert.Nil(t, err)
	mc.AssertCalled(t, "Do", mock.Anything, mock.Anything)
	mr := response.Response{}
	mr.FromJSON([]byte(resp.Message))

	assert.Equal(t, "test", mr.Name)
	assert.Equal(t, "hello world", mr.Body)
	assert.Len(t, mr.UpstreamCalls, 1)
	assert.Equal(t, "upstream", mr.UpstreamCalls[0].Name)
	assert.Equal(t, "http://test.com", mr.UpstreamCalls[0].URI)
}

func TestGRPCServiceHandlesRequestWithGRPCUpstream(t *testing.T) {
	uris := []string{"grpc://test.com"}
	fs, _, gc := setupFakeServer(t, uris, 0)

	gcMock := gc["grpc://test.com"].(*client.MockGRPC)
	gcMock.On("Handle", mock.Anything, mock.Anything).Return(&api.Response{Message: `{"name": "upstream", "body": "OK"}`}, nil)

	resp, err := fs.Handle(context.Background(), nil)
	mr := response.Response{}
	mr.FromJSON([]byte(resp.Message))

	assert.Nil(t, err)
	gcMock.AssertCalled(t, "Handle", mock.Anything, mock.Anything)

	assert.Equal(t, "test", mr.Name)
	assert.Equal(t, "hello world", mr.Body)
	assert.Len(t, mr.UpstreamCalls, 1)
	assert.Equal(t, "upstream", mr.UpstreamCalls[0].Name)
	assert.Equal(t, "grpc://test.com", mr.UpstreamCalls[0].URI)
}
