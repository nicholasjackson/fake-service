package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/nicholasjackson/fake-service/logging"
	"github.com/stretchr/testify/assert"
)

func setupReady(t *testing.T, code int, delay time.Duration) *Ready {
	return NewReady(
		logging.NewLogger(&logging.NullMetrics{}, hclog.Default(), nil),
		code,
		delay,
	)
}

func TestReadyReturnsCorrectResponseWhenNoDelay(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h := setupReady(t, http.StatusOK, 0)

	h.Handle(rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "OK", rr.Body.String())
}

func TestReadyReturnsUnavailableResponseWhenDelayNotElapsed(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h := setupReady(t, http.StatusOK, 10*time.Millisecond)

	h.Handle(rr, r)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	assert.Equal(t, "OK", rr.Body.String())
}

func TestReadyReturnsOKResponseWhenDelayElapsed(t *testing.T) {
	h := setupReady(t, http.StatusOK, 10*time.Millisecond)

	calls := 0

	assert.Eventually(
		t,
		func() bool {

			r := httptest.NewRequest(http.MethodGet, "/", nil)
			rr := httptest.NewRecorder()
			h.Handle(rr, r)
			calls++
			return rr.Code == http.StatusOK
		},
		100*time.Millisecond,
		1*time.Millisecond,
	)

	// should be ten calls made as it take 10 milliseconds to become ready
	assert.Equal(t, 10, calls)
}
