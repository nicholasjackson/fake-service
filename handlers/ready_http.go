package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/nicholasjackson/fake-service/logging"
)

// Health defines the health handler for the service
type Ready struct {
	logger     *logging.Logger
	statusCode int
	delay      time.Duration
}

// NewReady creates a new ready handler
func NewReady(logger *logging.Logger, code int, delay time.Duration) *Ready {
	r := &Ready{
		logger,
		code,
		delay,
	}

	if delay != 0 {
		// set the status code to unavailable until the delay has passed
		r.statusCode = http.StatusServiceUnavailable
		time.AfterFunc(delay, func() {
			r.statusCode = code
		})
	}

	return r
}

// Handle the request
func (h *Ready) Handle(rw http.ResponseWriter, r *http.Request) {
	hq := h.logger.CallReadyHTTP()

	hq.SetMetadata("response", fmt.Sprintf("%d", h.statusCode))

	rw.WriteHeader(h.statusCode)
	fmt.Fprint(rw, "OK")

	hq.SetMetadata("code", fmt.Sprintf("%d", h.statusCode))
	hq.Finished()
}
