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

func setupReady(t *testing.T, successCode, failureCode int, delay time.Duration) *Ready {
	return NewReady(
		logging.NewLogger(&logging.NullMetrics{}, hclog.Default(), nil),
		successCode,
		failureCode,
		delay,
	)
}

func TestReadyReturnsCorrectResponseWhenNoDelay(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h := setupReady(t, http.StatusOK, http.StatusServiceUnavailable, 0)

	h.Handle(rr, r)

	calls := 0
	assert.Eventually(
		t,
		func() bool {

			r := httptest.NewRequest(http.MethodGet, "/", nil)
			rr := httptest.NewRecorder()
			h.Handle(rr, r)

			if rr.Code == http.StatusOK && rr.Body.String() == OKMessage {
				calls++
				return true
			}

			return false
		},
		100*time.Millisecond,
		1*time.Millisecond,
	)

	assert.Equal(t, 1, calls)
}

func TestReadyReturnsUnavailableResponseWhenDelayNotElapsed(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h := setupReady(t, http.StatusOK, http.StatusServiceUnavailable, 10*time.Millisecond)

	h.Handle(rr, r)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	assert.Equal(t, StartingMessage, rr.Body.String())
}

func TestReadyReturnsOKResponseWhenDelayElapsed(t *testing.T) {
	h := setupReady(t, http.StatusOK, http.StatusServiceUnavailable, 10*time.Millisecond)

	calls := 0

	assert.Eventually(
		t,
		func() bool {

			r := httptest.NewRequest(http.MethodGet, "/", nil)
			rr := httptest.NewRecorder()
			h.Handle(rr, r)
			calls++
			return rr.Code == http.StatusOK && rr.Body.String() == OKMessage
		},
		100*time.Millisecond,
		1*time.Millisecond,
	)

	// should be more than 1 call, as there should be at least one unavailable response
	// this test is not coded to a fixed amound due to varinng speeds on CI
	assert.Greater(t, calls, 1)
}

func TestReadyReturnsCompleteWhenDelayElapsed(t *testing.T) {
	h := setupReady(t, http.StatusOK, http.StatusServiceUnavailable, 10*time.Millisecond)

	assert.Eventually(t, func() bool { return h.Complete() }, 100*time.Millisecond, 1*time.Millisecond)
}
