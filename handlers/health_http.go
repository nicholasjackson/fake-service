package handlers

import (
	"fmt"
	"net/http"

	"github.com/nicholasjackson/fake-service/logging"
)

// Health defines the health handler for the service
type Health struct {
	logger *logging.Logger
}

// NewHealth creates a new health handler
func NewHealth(logger *logging.Logger) *Health {
	return &Health{
		logger,
	}
}

// Handle the request
func (h *Health) Handle(rw http.ResponseWriter, r *http.Request) {
	hq := h.logger.CallHealthHTTP()
	defer hq.Finished()

	hq.SetMetadata("response", "200")

	fmt.Fprint(rw, "OK")
}
