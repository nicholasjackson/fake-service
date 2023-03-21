package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/nicholasjackson/fake-service/client"
	"github.com/nicholasjackson/fake-service/errors"
	"github.com/nicholasjackson/fake-service/grpc/api"
	"github.com/nicholasjackson/fake-service/load"
	"github.com/nicholasjackson/fake-service/logging"
	"github.com/nicholasjackson/fake-service/response"
	"github.com/nicholasjackson/fake-service/timing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func setupFakeServer(t *testing.T, uris []string, errorRate float64) (*FakeServer, *client.MockHTTP, map[string]client.GRPC) {
	l := logging.NewLogger(&logging.NullMetrics{}, hclog.Default(), nil)
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

	rh := NewReady(l, 200, 501, 10*time.Millisecond)

	// setup the error injector and load simulation
	i := errors.NewInjector(l.Log(), errorRate, int(codes.Internal), "http_error", 0, 0, 0)
	lg := load.NewGenerator(0, 0, 0, 0, hclog.Default())

	return NewFakeServer("test", "hello world", d, uris, 1, c, grpcClients, i, lg, l, load.NoopRequestGenerator, false, rh), c, grpcClients
}

func TestGRPCWaitsUntilReadinessCompletes(t *testing.T) {
	fs, _, _ := setupFakeServer(t, nil, 0)
	fs.waitTillReady = true

	failCount := 0

	assert.Eventually(t, func() bool {
		_, err := fs.Handle(context.Background(), nil)

		s, ok := status.FromError(err)
		if ok && s.Code() == codes.Unavailable {
			failCount++
			return false
		}

		return true
	}, 100*time.Millisecond, 1*time.Millisecond)

	assert.Greater(t, failCount, 1)
}

func TestGRPCServiceHandlesRequestWithNoUpstream(t *testing.T) {
	fs, _, _ := setupFakeServer(t, nil, 0)

	resp, err := fs.Handle(context.Background(), nil)
	mr := response.Response{}
	mr.FromJSON([]byte(resp.Message))

	assert.Nil(t, err)
	assert.Equal(t, "test", mr.Name)

	d, err := mr.Body.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, "\"hello world\"", string(d))

	assert.Len(t, mr.UpstreamCalls, 0)
}

func TestGRPCServiceHandlesErrorInjection(t *testing.T) {
	fs, _, _ := setupFakeServer(t, nil, 1)

	resp, err := fs.Handle(context.Background(), nil)
	status, ok := status.FromError(err)

	assert.Error(t, err)
	assert.True(t, ok)
	assert.Equal(t, codes.Internal, status.Code())
	assert.Nil(t, resp)

	// test the response is returned in the error body
	assert.Len(t, status.Details(), 1)
	d, ok := status.Details()[0].(*api.Response)
	assert.True(t, ok)

	mr := response.Response{}
	mr.FromJSON([]byte(d.Message))
	assert.Equal(t, "test", mr.Name)
	assert.Equal(t, json.RawMessage(json.RawMessage(nil)), mr.Body, "No body should be returned when the service has an exception")
	assert.Equal(t, int(codes.Internal), mr.Code)
	assert.Equal(t, "Service error automatically injected", mr.Error)
}

func TestGRPCServiceHandlesRequestWithHTTPUpstreamError(t *testing.T) {
	uris := []string{"http://test.com"}
	fs, mc, _ := setupFakeServer(t, uris, 0)
	mc.On("Do", mock.Anything, mock.Anything).Return(http.StatusInternalServerError, []byte(`{"name": "upstream", "error": "boom", "code": 500}`), fmt.Errorf("It went bang"))

	resp, err := fs.Handle(context.Background(), nil)
	status, ok := status.FromError(err)

	assert.Error(t, err)
	assert.Nil(t, resp)
	mc.AssertCalled(t, "Do", mock.Anything, mock.Anything)

	// test the response is returned in the error body
	assert.Len(t, status.Details(), 1)
	d, ok := status.Details()[0].(*api.Response)
	assert.True(t, ok)

	mr := response.Response{}
	mr.FromJSON([]byte(d.Message))
	assert.Equal(t, "test", mr.Name)
	assert.Equal(t, json.RawMessage(json.RawMessage(nil)), mr.Body, "No body should be returned when the service has an exception")
	assert.Equal(t, int(codes.Internal), mr.Code)
	assert.Equal(t, "It went bang", mr.Error)
}

func TestGRPCServiceHandlesRequestWithHTTPUpstream(t *testing.T) {
	uris := []string{"http://test.com"}
	fs, mc, _ := setupFakeServer(t, uris, 0)
	mc.On("Do", mock.Anything, mock.Anything).Return(http.StatusOK, []byte(`{"name": "upstream", "body": "OK"}`), nil)

	resp, err := fs.Handle(context.Background(), nil)

	assert.Nil(t, err)
	mc.AssertCalled(t, "Do", mock.Anything, mock.Anything)
	mr := response.Response{}
	mr.FromJSON([]byte(resp.Message))

	assert.Equal(t, "test", mr.Name)

	d, err := mr.Body.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, "\"hello world\"", string(d))

	assert.Len(t, mr.UpstreamCalls, 1)
	assert.Equal(t, "upstream", mr.UpstreamCalls["http://test.com"].Name)
	assert.Equal(t, "http://test.com", mr.UpstreamCalls["http://test.com"].URI)
}

func TestGRPCServiceHandlesRequestWithGRPCUpstream(t *testing.T) {
	uris := []string{"grpc://test.com"}
	fs, _, gc := setupFakeServer(t, uris, 0)

	gcMock := gc["grpc://test.com"].(*client.MockGRPC)
	gcMock.On("Handle", mock.Anything, mock.Anything).Return(&api.Response{Message: `{"name": "upstream", "body": "OK"}`}, map[string]string{"test": "abc"}, nil)

	resp, err := fs.Handle(context.Background(), nil)
	mr := response.Response{}
	mr.FromJSON([]byte(resp.Message))

	assert.Nil(t, err)
	gcMock.AssertCalled(t, "Handle", mock.Anything, mock.Anything)

	assert.Equal(t, "test", mr.Name)

	d, err := mr.Body.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, "\"hello world\"", string(d))

	assert.Len(t, mr.UpstreamCalls, 1)
	assert.Equal(t, "upstream", mr.UpstreamCalls["grpc://test.com"].Name)
	assert.Equal(t, "grpc://test.com", mr.UpstreamCalls["grpc://test.com"].URI)
	assert.Equal(t, "abc", mr.UpstreamCalls["grpc://test.com"].Headers["test"])
}
