package timing

import (
	"math/rand"
	"time"
)

// generate a random number
func generateRandom(max int) int {
	return rand.Intn(max)
}

// RequestDuration is a structure which allows the calculation of randomised
// request durations
type RequestDuration struct {
	percentile50 time.Duration
	percentile90 time.Duration
	percentile99 time.Duration
	// random variance for the request as percentage of total
	variance   int
	randomFunc func(max int) int
}

// NewRequestDuration creates a new RequestDuration
func NewRequestDuration(percentile50, percentile90, percentile99 time.Duration, variance int) *RequestDuration {

	if percentile50 > 0 && percentile90 == 0 {
		percentile90 = percentile50
	}

	if percentile90 > 0 && percentile99 == 0 {
		percentile99 = percentile90
	}

	return &RequestDuration{
		percentile50: percentile50,
		percentile90: percentile90,
		percentile99: percentile99,
		variance:     variance,
		randomFunc:   generateRandom,
	}
}

// Calculate a new random request duration
func (r *RequestDuration) Calculate() time.Duration {

	// calculate the random variance percentage
	var rv = 0
	if r.variance > 0 {
		rv = r.randomFunc(r.variance)
	}

	// generate a random percentile
	switch p := r.randomFunc(100); {
	case p < 90:
		return r.calculateDuration(r.percentile50, rv)
	case p < 99:
		return r.calculateDuration(r.percentile90, rv)
	default:
		return r.calculateDuration(r.percentile99, rv)
	}
}

func (r *RequestDuration) calculateDuration(rq time.Duration, vp int) time.Duration {
	return rq + time.Duration(float64(rq.Nanoseconds())*float64(vp)/float64(100))
}
