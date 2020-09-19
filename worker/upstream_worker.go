package worker

import (
	"sync"

	"github.com/nicholasjackson/fake-service/response"
)

// WorkFunc defines a function which is called when work is to be done
type WorkFunc func(uri string) (*response.Response, error)

// Done is a message sent when an upstream worker has completed
type Done struct {
	URI      string
	Response *response.Response
}

// UpstreamWorker manages parallel upstream requests
type UpstreamWorker struct {
	workerCount int
	workChan    chan string
	errChan     chan error
	doneChan    chan struct{}
	workFunc    WorkFunc
	waitGroup   *sync.WaitGroup
	responses   []Done
}

// New UpstreamWorker
func New(workerCount int, f WorkFunc) *UpstreamWorker {
	return &UpstreamWorker{
		workerCount: workerCount,
		workChan:    make(chan string),
		errChan:     make(chan error),
		doneChan:    make(chan struct{}),
		workFunc:    f,
		waitGroup:   &sync.WaitGroup{},
		responses:   []Done{},
	}
}

// Do runs the worker with the given uris
func (u *UpstreamWorker) Do(uris []string) error {
	// start the workers
	for n := 0; n < u.workerCount; n++ {
		go u.worker()
	}

	u.waitGroup.Add(len(uris))

	// monitor the threads and send a message when done
	u.monitorStatus()

	// start the work
	go func() {
		for _, uri := range uris {
			u.workChan <- uri
		}
	}()

	var err error
	select {
	case err = <-u.errChan:
	case <-u.doneChan:
	}

	// close the work channel to ensure the worker does
	// not leak goroutines
	close(u.workChan)
	return err
}

// Responses returns the responses from the upstream calls
func (u *UpstreamWorker) Responses() []Done {
	return u.responses
}

//
func (u *UpstreamWorker) monitorStatus() {
	go func() {
		u.waitGroup.Wait()
		u.doneChan <- struct{}{}
	}()
}

func (u *UpstreamWorker) worker() {
	for {
		uri, ok := <-u.workChan

		// all work is complete exit
		if !ok {
			break
		}

		resp, err := u.workFunc(uri)
		u.responses = append(u.responses, Done{uri, resp})

		if err != nil {
			u.errChan <- err
			break
		}
		u.waitGroup.Done()
	}
}
