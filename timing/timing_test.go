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

func setup(t *testing.T, p int) *RequestDuration {
	randomPercentile = p
	randomVariance = 10

	return &RequestDuration{
		percentile50: 1 * time.Millisecond,
		percentile90: 2 * time.Millisecond,
		percentile99: 3 * time.Millisecond,
		variance:     randomVariance,
		randomFunc:   pseudoRandom,
	}
}

func TestGenerates50Percentile(t *testing.T) {
	rd := setup(t, 50)

	d := rd.Calculate()

	assert.Equal(t, 1100*time.Microsecond, d)
}

func TestGenerates90Percentile(t *testing.T) {
	rd := setup(t, 90)

	d := rd.Calculate()

	assert.Equal(t, 2200*time.Microsecond, d)
}

func TestGenerates99Percentile(t *testing.T) {
	rd := setup(t, 99)

	d := rd.Calculate()

	assert.Equal(t, 3300*time.Microsecond, d)
}
