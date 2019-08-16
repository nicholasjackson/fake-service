package errors

import (
	"time"

	"cloud.google.com/go/logging"
)

type Errors struct {
	logger          logging.Logger
	errorPercentage int
	errorCode       int
	errorType       string
	errorDelay      time.Duration
	requestCount    int
}
