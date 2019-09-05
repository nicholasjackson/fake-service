package handlers

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/nicholasjackson/fake-service/client"
	"github.com/nicholasjackson/fake-service/grpc/api"
	"github.com/nicholasjackson/fake-service/response"
	"github.com/nicholasjackson/fake-service/timing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupRequest(t *testing.T, uris []string) (*Request, *client.MockHTTP, map[string]client.GRPC) {
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

	return &Request{
		name:          "test",
		message:       "hello world",
		logger:        hclog.Default(),
		duration:      d,
		upstreamURIs:  uris,
		workerCount:   1,
		defaultClient: c,
		grpcClients:   grpcClients,
	}, c, grpcClients
}

func TestRequestCompletesWithNoUpstreams(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", bytes.NewReader([]byte("")))
	rr := httptest.NewRecorder()
	h, c, _ := setupRequest(t, nil)

	h.Handle(rr, r)
	mr := response.Response{}
	mr.FromJSON([]byte(rr.Body.String()))

	c.AssertNotCalled(t, "Do", mock.Anything)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "test", mr.Name)
	assert.Equal(t, "hello world", mr.Body)
	assert.Len(t, mr.UpstreamCalls, 0)
}

func TestRequestCompletesWithHTTPUpstreams(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", bytes.NewReader([]byte("")))
	rr := httptest.NewRecorder()
	h, c, _ := setupRequest(t, []string{"http://test.com"})

	// setup the upstream response
	c.On("Do", mock.Anything, mock.Anything).Return([]byte(`{"name": "upstream", "body": "OK"}`), nil)

	h.Handle(rr, r)

	c.AssertCalled(t, "Do", mock.Anything, mock.Anything)
	assert.Equal(t, http.StatusOK, rr.Code)
	mr := response.Response{}
	mr.FromJSON([]byte(rr.Body.String()))

	c.AssertNotCalled(t, "Do", mock.Anything)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "test", mr.Name)
	assert.Equal(t, "hello world", mr.Body)
	assert.Len(t, mr.UpstreamCalls, 1)
	assert.Equal(t, "upstream", mr.UpstreamCalls[0].Name)
	assert.Equal(t, "http://test.com", mr.UpstreamCalls[0].URI)
}

func TestReturnsErrorWithHTTPUpstreamError(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", bytes.NewReader([]byte("")))
	rr := httptest.NewRecorder()
	h, c, _ := setupRequest(t, []string{"http://something.com"})

	// setup the error
	c.On("Do", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("Boom"))

	h.Handle(rr, r)

	c.AssertCalled(t, "Do", mock.Anything, mock.Anything)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Equal(t, "Boom\n", rr.Body.String())
}

func TestRequestCompletesWithGRPCUpstreams(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", bytes.NewReader([]byte("")))
	rr := httptest.NewRecorder()
	h, _, gc := setupRequest(t, []string{"grpc://test.com"})

	// setup the upstream response
	gcMock := gc["grpc://test.com"].(*client.MockGRPC)
	gcMock.On("Handle", mock.Anything, mock.Anything).Return(&api.Response{Message: `{"name": "upstream", "body": "OK"}`}, nil)

	h.Handle(rr, r)
	mr := response.Response{}
	mr.FromJSON([]byte(rr.Body.String()))

	gcMock.AssertCalled(t, "Handle", mock.Anything, mock.Anything)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "test", mr.Name)
	assert.Equal(t, "hello world", mr.Body)
	assert.Len(t, mr.UpstreamCalls, 1)
	assert.Equal(t, "upstream", mr.UpstreamCalls[0].Name)
	assert.Equal(t, "grpc://test.com", mr.UpstreamCalls[0].URI)
}

func TestRequestCompletesWithGRPCUpstreamsError(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", bytes.NewReader([]byte("")))
	rr := httptest.NewRecorder()
	h, _, gc := setupRequest(t, []string{"grpc://something.com"})

	// setup the upstream response
	gcMock := gc["grpc://something.com"].(*client.MockGRPC)
	gcMock.On("Handle", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("Boom"))

	h.Handle(rr, r)

	gcMock.AssertCalled(t, "Handle", mock.Anything, mock.Anything)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Equal(t, "Boom\n", rr.Body.String())
}
