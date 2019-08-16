package handlers

import (
	"fmt"
	"net/http"

	"github.com/hashicorp/go-hclog"
)

// Health defines the health handler for the service
type Health struct {
	logger hclog.Logger
}

// NewHealth creates a new health handler
func NewHealth(logger hclog.Logger) *Health {
	return &Health{
		logger,
	}
}

// Handle the request
func (h *Health) Handle(rw http.ResponseWriter, r *http.Request) {
	h.logger.Info("Handling health request")

	fmt.Fprint(rw, "OK")
}
