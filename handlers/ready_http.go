package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/nicholasjackson/fake-service/logging"
)

const (
	OKMessage       = "OK"
	StartingMessage = "Starting Process"
)

// Health defines the health handler for the service
type Ready struct {
	logger        *logging.Logger
	statusCode    int
	statusMessage string
	delay         time.Duration
}

// NewReady creates a new ready handler
func NewReady(logger *logging.Logger, successCode, failureCode int, delay time.Duration) *Ready {
	r := &Ready{
		logger:        logger,
		statusCode:    failureCode,
		statusMessage: StartingMessage,
		delay:         delay,
	}

	// set the status code to unavailable until the delay has passed
	time.AfterFunc(delay, func() {
		r.statusCode = successCode
		r.statusMessage = OKMessage
	})

	return r
}

// Handle the request
func (h *Ready) Handle(rw http.ResponseWriter, r *http.Request) {
	hq := h.logger.CallReadyHTTP()

	hq.SetMetadata("response", fmt.Sprintf("%d", h.statusCode))

	rw.WriteHeader(h.statusCode)
	fmt.Fprint(rw, h.statusMessage)

	hq.SetMetadata("code", fmt.Sprintf("%d", h.statusCode))
	hq.Finished()
}
