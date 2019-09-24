package errors

import (
	"time"

	"cloud.google.com/go/logging"
	"golang.org/x/time/rate"
)

type Errors struct {
	logger          logging.Logger
	errorPercentage int
	errorCode       int
	errorType       string
	errorDelay      time.Duration
	requestCount    int
	rateLimit       int
}

func (e *Errors) Do() error {

	if *upstreamRateLimit > 0 {
		limiter = rate.NewLimiter(rate.Limit(*upstreamRateLimit), int(*upstreamRateLimit/10.0))
	}
}
