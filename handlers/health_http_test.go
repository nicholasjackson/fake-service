package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/nicholasjackson/fake-service/logging"
	"github.com/stretchr/testify/assert"
)

func setupHealth(t *testing.T, code int) *Health {
	return NewHealth(
		logging.NewLogger(&logging.NullMetrics{}, hclog.Default(), nil),
		code,
	)
}

func TestHealthReturnsCorrectResponse(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h := setupHealth(t, 200)

	h.Handle(rr, r)

	assert.Equal(t, 200, rr.Code)
	assert.Equal(t, "OK", rr.Body.String())
}
