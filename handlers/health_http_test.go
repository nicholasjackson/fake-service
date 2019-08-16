package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

func setupHealth(t *testing.T) *Health {
	return &Health{
		hclog.Default(),
	}
}

func TestHealthReturnsCorrectResponse(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h := setupHealth(t)

	h.Handle(rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "OK", rr.Body.String())
}
