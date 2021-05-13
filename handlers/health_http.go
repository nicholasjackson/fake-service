package handlers

import (
	"fmt"
	"net/http"

	"github.com/nicholasjackson/fake-service/logging"
)

// Health defines the health handler for the service
type Health struct {
	logger     *logging.Logger
	statusCode int
}

// NewHealth creates a new health handler
func NewHealth(logger *logging.Logger, code int) *Health {
	return &Health{
		logger,
		code,
	}
}

// Handle the request
func (h *Health) Handle(rw http.ResponseWriter, r *http.Request) {
	hq := h.logger.CallHealthHTTP()
	defer hq.Finished()

	hq.SetMetadata("response", fmt.Sprintf("%d", h.statusCode))

	rw.WriteHeader(h.statusCode)
	fmt.Fprint(rw, "OK")
}
