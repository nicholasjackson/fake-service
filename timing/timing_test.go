package timing

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var randomPercentile int
var randomVariance int

func pseudoRandom(max int) int {
	switch max {
	case 100:
		return randomPercentile
	default:
		return randomVariance
	}
}

func setup(t *testing.T) *RequestDuration {
	randomPercentile = 50
	randomVariance = 10

	return &RequestDuration{
		percentile50: 1 * time.Millisecond,
		percentile90: 2 * time.Millisecond,
		percentile99: 3 * time.Millisecond,
		variance:     0.1,
		randomFunc:   pseudoRandom,
	}
}

func TestGenerates50Percentile(t *testing.T) {
	rd := setup(t)

	d := rd.Calculate()

	assert.Equal(t, 1100*time.Microsecond, d)
}
