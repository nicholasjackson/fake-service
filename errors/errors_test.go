package errors

import (
	"net/http"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

func setup(t *testing.T) *Injector {
	return &Injector{logger: hclog.Default()}
}

func TestRateIsLimited(t *testing.T) {
	e := setup(t)
	e.rateLimitRPS = 1
	e.rateLimitCode = http.StatusTooManyRequests

	err1 := e.Do()
	err2 := e.Do()

	assert.Nil(t, err1)
	assert.NotNil(t, err2)
	assert.Equal(t, ErrorRateLimit, err2.Error)
	assert.Equal(t, http.StatusTooManyRequests, err2.Code)
}

func TestRateIsNotLimited(t *testing.T) {
	e := setup(t)
	e.rateLimitRPS = 3

	err1 := e.Do()
	err2 := e.Do()

	assert.Nil(t, err1)
	assert.Nil(t, err2)
}

func TestErrorsReturnValidCode(t *testing.T) {
	e := setup(t)
	e.errorPercentage = 0.5 // 50% errors
	e.errorCode = http.StatusInternalServerError
	e.errorType = "http_error"

	err1 := e.Do()
	err2 := e.Do()

	assert.Nil(t, err1)
	assert.NotNil(t, err2)
	assert.Equal(t, ErrorInjection, err2.Error)
	assert.Equal(t, http.StatusInternalServerError, err2.Code)
}

func TestErrorsDelayForCorrectTime(t *testing.T) {
	e := setup(t)
	e.errorPercentage = 1 // 50% errors
	e.errorType = "delay"
	e.errorDelay = 100 * time.Millisecond

	st := time.Now()
	err1 := e.Do()
	dur := time.Now().Sub(st)

	assert.NotNil(t, err1)
	assert.Equal(t, err1.Error, ErrorDelay)
	assert.True(t, dur > 100*time.Millisecond)
}
