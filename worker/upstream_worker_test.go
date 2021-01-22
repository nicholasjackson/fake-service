package worker

import (
	"fmt"
	"testing"
	"time"

	"github.com/nicholasjackson/fake-service/response"
	"github.com/stretchr/testify/assert"
)

func TestUpstreamWorkerWithSingleURIAndSingleWorker(t *testing.T) {
	callCount := 0
	w := New(1, func(uri string) (*response.Response, error) {
		callCount++

		return &response.Response{}, nil
	})

	w.Do([]string{"123"})

	assert.Equal(t, 1, callCount)
}

func TestUpstreamWorkerWithTwoURIAndSingleWorker(t *testing.T) {
	callCount := 0
	w := New(1, func(uri string) (*response.Response, error) {
		callCount++

		return &response.Response{}, nil
	})

	w.Do([]string{"123", "abc"})

	assert.Equal(t, 2, callCount)
}
func TestUpstreamWorkerWithTwoURIAndSingleWorkerFirstFail(t *testing.T) {
	callCount := 0
	w := New(1, func(uri string) (*response.Response, error) {
		callCount++

		return &response.Response{}, fmt.Errorf(fmt.Sprintf("%d", callCount))
	})

	w.Do([]string{"123", "abc"})

	assert.Equal(t, w.err.Error(), "1")
}

func TestUpstreamWorkerWithTwoURIAndTwoWorkers(t *testing.T) {
	startOrder := []string{}
	calls := []string{}
	callCount := 0
	sleepTime := []time.Duration{20 * time.Millisecond, 10 * time.Millisecond}

	w := New(2, func(uri string) (*response.Response, error) {
		startOrder = append(startOrder, uri)
		callCount++
		time.Sleep(sleepTime[callCount-1])
		calls = append(calls, uri)

		return &response.Response{}, nil
	})

	w.Do([]string{"123", "abc"})

	assert.Equal(t, 2, callCount)
	// if parallel first finished should not be equal to first started due to
	// sleepTimes
	assert.NotEqual(t, startOrder[0], calls[0])
}

func TestUpstreamWorkerWithTwoURIAndTwoWorkerFirstFail(t *testing.T) {
	callCount := 0
	w := New(2, func(uri string) (*response.Response, error) {
		callCount++

		return &response.Response{}, fmt.Errorf(fmt.Sprintf("%d", callCount))
	})

	w.Do([]string{"123", "abc"})

	assert.Equal(t, w.err.Error(), "1")
}
