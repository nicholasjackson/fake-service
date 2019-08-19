package worker

import (
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

func TestUpstreamWorkerWithSingleURIAndSingleWorker(t *testing.T) {
	callCount := 0
	l := hclog.New(&hclog.LoggerOptions{Level: hclog.Debug})
	w := New(1, l, func(uri string) (string, error) {
		callCount++

		return "", nil
	})

	w.Do([]string{"123"})

	assert.Equal(t, 1, callCount)
}

func TestUpstreamWorkerWithTwoURIAndSingleWorker(t *testing.T) {
	callCount := 0
	l := hclog.New(&hclog.LoggerOptions{Level: hclog.Debug})
	w := New(1, l, func(uri string) (string, error) {
		callCount++

		return "", nil
	})

	w.Do([]string{"123", "abc"})

	assert.Equal(t, 2, callCount)
}

func TestUpstreamWorkerWithTwoURIAndTwoWorkers(t *testing.T) {
	startOrder := []string{}
	calls := []string{}
	callCount := 0
	sleepTime := []time.Duration{20 * time.Millisecond, 10 * time.Millisecond}

	l := hclog.New(&hclog.LoggerOptions{Level: hclog.Debug})
	w := New(2, l, func(uri string) (string, error) {
		startOrder = append(startOrder, uri)
		callCount++
		time.Sleep(sleepTime[callCount-1])
		calls = append(calls, uri)

		return "", nil
	})

	w.Do([]string{"123", "abc"})

	assert.Equal(t, 2, callCount)
	// if parallel first finished should not be equal to first started due to
	// sleepTimes
	assert.NotEqual(t, startOrder[0], calls[0])
}
