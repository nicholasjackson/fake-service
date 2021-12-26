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

func setupRequest(t *testing.T, uris []string, errorRate float64) (*Request, *client.MockHTTP, map[string]client.GRPC) {
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

	i := errors.NewInjector(hclog.Default(), errorRate, http.StatusInternalServerError, "http_error", 0, 0, 0)
	lg := load.NewGenerator(0, 0, 0, 0, hclog.Default())

	rh := NewReady(l, 200, 501, 10*time.Millisecond)

	return &Request{
		name:             "test",
		message:          "hello world",
		duration:         d,
		upstreamURIs:     uris,
		workerCount:      1,
		defaultClient:    c,
		grpcClients:      grpcClients,
		errorInjector:    i,
		loadGenerator:    lg,
		log:              l,
		requestGenerator: load.NoopRequestGenerator,
		waitTillReady:    false,
		readinessHandler: rh,
	}, c, grpcClients
}

func TestRequestWaitsUntilReadinessCompletes(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", bytes.NewReader([]byte("")))
	h, _, _ := setupRequest(t, nil, 0)
	h.waitTillReady = true

	failCount := 0

	assert.Eventually(t, func() bool {
		rr := httptest.NewRecorder()

		h.ServeHTTP(rr, r)

		if rr.Code != http.StatusOK {
			failCount++
			return false
		}

		return true
	}, 100*time.Millisecond, 1*time.Millisecond)

	assert.Greater(t, failCount, 1)
}

func TestRequestCompletesWithNoUpstreams(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", bytes.NewReader([]byte("")))
	rr := httptest.NewRecorder()
	h, c, _ := setupRequest(t, nil, 0)

	h.ServeHTTP(rr, r)
	mr := response.Response{}
	mr.FromJSON([]byte(rr.Body.String()))

	c.AssertNotCalled(t, "Do", mock.Anything)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "test", mr.Name)

	// check the body
	d, err := mr.Body.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, "\"hello world\"", string(d))

	assert.Equal(t, http.StatusOK, mr.Code)
	assert.Len(t, mr.UpstreamCalls, 0)
}

func TestRequestCompletesWithNoUpstreamsJSONBody(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", bytes.NewReader([]byte("")))
	rr := httptest.NewRecorder()
	h, c, _ := setupRequest(t, nil, 0)
	h.message = "{\"hello\": \"world\"}"

	h.ServeHTTP(rr, r)
	mr := response.Response{}
	mr.FromJSON([]byte(rr.Body.String()))

	c.AssertNotCalled(t, "Do", mock.Anything)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "test", mr.Name)

	// check the body
	d, err := mr.Body.MarshalJSON()
	assert.NoError(t, err)
	assert.JSONEq(t, h.message, string(d))

	assert.Equal(t, http.StatusOK, mr.Code)
	assert.Len(t, mr.UpstreamCalls, 0)
}

func TestRequestCompletesWithInjectedError(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", bytes.NewReader([]byte("")))
	rr := httptest.NewRecorder()
	h, c, _ := setupRequest(t, nil, 1)

	h.ServeHTTP(rr, r)
	mr := response.Response{}
	mr.FromJSON([]byte(rr.Body.String()))

	c.AssertNotCalled(t, "Do", mock.Anything)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Equal(t, "test", mr.Name)
	assert.Equal(t, http.StatusInternalServerError, mr.Code)
	assert.Equal(t, errors.ErrorInjection.Error(), mr.Error)
	assert.Len(t, mr.UpstreamCalls, 0)
}

func TestRequestCompletesWithHTTPUpstreams(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", bytes.NewReader([]byte("")))
	rr := httptest.NewRecorder()
	h, c, _ := setupRequest(t, []string{"http://test.com"}, 0)

	// setup the upstream response
	c.On("Do", mock.Anything, mock.Anything).Return(http.StatusOK, []byte(`{"name": "upstream", "body": "OK"}`), nil)

	h.ServeHTTP(rr, r)

	c.AssertCalled(t, "Do", mock.Anything, mock.Anything)
	assert.Equal(t, http.StatusOK, rr.Code)
	mr := response.Response{}
	mr.FromJSON([]byte(rr.Body.String()))

	c.AssertNotCalled(t, "Do", mock.Anything)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "test", mr.Name)

	d, err := mr.Body.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, "\"hello world\"", string(d))

	assert.Len(t, mr.UpstreamCalls, 1)
	assert.Equal(t, "upstream", mr.UpstreamCalls["http://test.com"].Name)
	assert.Equal(t, "http://test.com", mr.UpstreamCalls["http://test.com"].URI)
}

func TestReturnsErrorWithHTTPUpstreamConnectionError(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", bytes.NewReader([]byte("")))
	rr := httptest.NewRecorder()
	h, c, _ := setupRequest(t, []string{"http://something.com"}, 0)

	// setup the error
	c.On("Do", mock.Anything, mock.Anything).Return(-1, nil, fmt.Errorf("Boom"))

	h.ServeHTTP(rr, r)
	mr := response.Response{}
	mr.FromJSON(rr.Body.Bytes())

	c.AssertCalled(t, "Do", mock.Anything, mock.Anything)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	assert.Equal(t, "test", mr.Name)
	assert.Len(t, mr.UpstreamCalls, 1)
	assert.Equal(t, -1, mr.UpstreamCalls["http://something.com"].Code)

}

func TestReturnsErrorWithHTTPUpstreamHandleError(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", bytes.NewReader([]byte("")))
	rr := httptest.NewRecorder()
	h, c, _ := setupRequest(t, []string{"http://something.com"}, 0)

	// setup the error
	c.On("Do", mock.Anything, mock.Anything).Return(http.StatusInternalServerError, []byte(`{"name": "upstream", "code": 503, "error": "boom"}`), fmt.Errorf("Error processing upstream"))

	h.ServeHTTP(rr, r)
	mr := response.Response{}
	mr.FromJSON([]byte(rr.Body.String()))

	c.AssertCalled(t, "Do", mock.Anything, mock.Anything)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Equal(t, "test", mr.Name)
	assert.Equal(t, http.StatusInternalServerError, mr.Code)
	assert.Len(t, mr.UpstreamCalls, 1)
	assert.Equal(t, http.StatusInternalServerError, mr.UpstreamCalls["http://something.com"].Code)
}

func TestRequestCompletesWithHTTPSUpstreams(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", bytes.NewReader([]byte("")))
	rr := httptest.NewRecorder()
	h, c, _ := setupRequest(t, []string{"https://test.com"}, 0)

	// setup the upstream response
	c.On("Do", mock.Anything, mock.Anything).Return(http.StatusOK, []byte(`{"name": "upstream", "body": "OK"}`), nil)

	h.ServeHTTP(rr, r)

	c.AssertCalled(t, "Do", mock.Anything, mock.Anything)
	assert.Equal(t, http.StatusOK, rr.Code)
	mr := response.Response{}
	mr.FromJSON([]byte(rr.Body.String()))

	c.AssertNotCalled(t, "Do", mock.Anything)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "test", mr.Name)

	d, err := mr.Body.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, "\"hello world\"", string(d))

	assert.Len(t, mr.UpstreamCalls, 1)
	assert.Equal(t, "upstream", mr.UpstreamCalls["https://test.com"].Name)
	assert.Equal(t, "https://test.com", mr.UpstreamCalls["https://test.com"].URI)
}

func TestRequestCompletesWithGRPCUpstreams(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", bytes.NewReader([]byte("")))
	rr := httptest.NewRecorder()
	h, _, gc := setupRequest(t, []string{"grpc://test.com"}, 0)

	// setup the upstream response
	gcMock := gc["grpc://test.com"].(*client.MockGRPC)
	gcMock.On("Handle", mock.Anything, mock.Anything).Return(&api.Response{Message: `{"name": "upstream", "body": "OK"}`}, map[string]string{"test": "abc"}, nil)

	h.ServeHTTP(rr, r)
	mr := response.Response{}
	mr.FromJSON([]byte(rr.Body.String()))

	gcMock.AssertCalled(t, "Handle", mock.Anything, mock.Anything)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "test", mr.Name)

	d, err := mr.Body.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, "\"hello world\"", string(d))

	assert.Len(t, mr.UpstreamCalls, 1)
	assert.Equal(t, "upstream", mr.UpstreamCalls["grpc://test.com"].Name)
	assert.Equal(t, "grpc://test.com", mr.UpstreamCalls["grpc://test.com"].URI)
	assert.Equal(t, "abc", mr.UpstreamCalls["grpc://test.com"].Headers["test"])
}

func TestRequestCompletesWithGRPCUpstreamsError(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", bytes.NewReader([]byte("")))
	rr := httptest.NewRecorder()
	h, _, gc := setupRequest(t, []string{"grpc://something.com"}, 0)

	// setup the upstream response
	gcMock := gc["grpc://something.com"].(*client.MockGRPC)
	gcMock.On("Handle", mock.Anything, mock.Anything).Return(nil, map[string]string{}, status.Error(codes.Internal, "Boom"))

	h.ServeHTTP(rr, r)
	mr := response.Response{}
	mr.FromJSON([]byte(rr.Body.String()))
	//pretty.Print(mr)

	gcMock.AssertCalled(t, "Handle", mock.Anything, mock.Anything)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Equal(t, "test", mr.Name)
}
