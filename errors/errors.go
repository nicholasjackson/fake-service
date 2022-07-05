package errors

import (
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"golang.org/x/time/rate"
)

type Response struct {
	Code  int
	Error error
}

var ErrorRateLimit = fmt.Errorf("Service exceeded rate limit")
var ErrorInjection = fmt.Errorf("Service error automatically injected")
var ErrorDelay = fmt.Errorf("Service delay automatically injected")

// Injector allows errors and ratelmiting to be injected to a service
type Injector struct {
	logger          hclog.Logger
	errorPercentage float64
	errorCode       int
	errorType       string
	errorDelay      time.Duration
	rateLimitRPS    float64
	rateLimitBurst  int
	rateLimitCode   int

	limiter      *rate.Limiter
	requestCount int
}

func NewInjector(l hclog.Logger, errorPercentage float64, errorCode int, errorType string, errorDelay time.Duration, rateLimitRPS float64, rateLimitCode int) *Injector {
	return &Injector{
		logger:          l,
		errorPercentage: errorPercentage,
		errorCode:       errorCode,
		errorType:       errorType,
		errorDelay:      errorDelay,
		rateLimitRPS:    rateLimitRPS,
		rateLimitCode:   rateLimitCode,
	}
}

// SetErrorPercentage sets the error rate for the injector
// must be a floating point number  between 0 and 1
func (e *Injector) SetErrorPercentage(rate float64) {
	e.errorPercentage = rate
}

// Do returns an error
func (e *Injector) Do() *Response {
	e.requestCount++ // increment the request count

	// lazy instatiate rate limiter
	if e.rateLimitRPS > 0 && e.limiter == nil {
		if e.rateLimitBurst == 0 {
			// if no burst limit, set the initial bucket size to the rps
			e.rateLimitBurst = int(e.rateLimitRPS)
		}

		e.limiter = rate.NewLimiter(rate.Limit(e.rateLimitRPS), e.rateLimitBurst)
	}

	if e.limiter != nil && !e.limiter.Allow() {
		e.logger.Info("Rate limiting service")

		return &Response{Error: ErrorRateLimit, Code: e.rateLimitCode}
	}

	// if the request count is greater than max int reset
	if e.requestCount == int(^uint(0)>>1) {
		e.requestCount = 1
	}

	// calculate if we need to throw an error or continue as normal
	if e.requestCount%int(1/e.errorPercentage) == 0 {
		e.logger.Info("Injecting error", "request_count", e.requestCount, "error_percentage", e.errorPercentage, "error_type", e.errorType)

		// is our error a delay or a timeout
		if e.errorType == "http_error" {
			return &Response{Error: ErrorInjection, Code: e.errorCode}
		}

		// delay
		e.logger.Info("Delaying service execution", "duration", e.errorDelay)
		time.Sleep(e.errorDelay)
		return &Response{Error: ErrorDelay, Code: e.errorCode}
	}

	return nil
}
