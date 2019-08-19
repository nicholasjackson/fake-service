package handlers

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/nicholasjackson/fake-service/client"
	"github.com/nicholasjackson/fake-service/timing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupRequest(t *testing.T, uris []string) (*Request, *client.MockHTTP) {
	c := &client.MockHTTP{}
	d := timing.NewRequestDuration(
		1*time.Nanosecond,
		1*time.Nanosecond,
		1*time.Nanosecond,
		0)

	return &Request{
		name:          "test",
		message:       "test message",
		logger:        hclog.Default(),
		duration:      d,
		upstreamURIs:  uris,
		workerCount:   1,
		defaultClient: c,
	}, c
}

func TestRequestCompletesWithNoUpstreams(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", bytes.NewReader([]byte("")))
	rr := httptest.NewRecorder()
	h, c := setupRequest(t, nil)

	h.Handle(rr, r)

	c.AssertNotCalled(t, "Do", mock.Anything)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "# Reponse from: test #\ntest message\n", rr.Body.String())
}

func TestRequestCompletesWithUpstreams(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", bytes.NewReader([]byte("")))
	rr := httptest.NewRecorder()
	h, c := setupRequest(t, []string{"http://something.com"})

	// setup the upstream response
	c.On("Do", mock.Anything).Return([]byte("# Response from: upstream #\nOK\n"), nil)

	h.Handle(rr, r)

	c.AssertCalled(t, "Do", mock.Anything)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(
		t,
		"# Reponse from: test #\ntest message\n## Called upstream uri: http://something.com\n  # Response from: upstream #\n  OK\n  ",
		rr.Body.String())
}

func TestReturnsErrorWithUpstreamError(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", bytes.NewReader([]byte("")))
	rr := httptest.NewRecorder()
	h, c := setupRequest(t, []string{"http://something.com"})

	// setup the error
	c.On("Do", mock.Anything).Return(nil, fmt.Errorf("Boom"))

	h.Handle(rr, r)

	c.AssertCalled(t, "Do", mock.Anything)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Equal(t, "Boom\n", rr.Body.String())
}
