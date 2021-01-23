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
	err         error
	workFunc    WorkFunc
	waitGroup   *sync.WaitGroup
	responses   []Done
}

// New UpstreamWorker
func New(workerCount int, f WorkFunc) *UpstreamWorker {
	return &UpstreamWorker{
		workerCount: workerCount,
		workChan:    make(chan string),
		workFunc:    f,
		waitGroup:   &sync.WaitGroup{},
		responses:   []Done{},
	}
}

// Do runs the worker with the given uris
func (u *UpstreamWorker) Do(uris []string) error {
	if u.workerCount > len(uris) {
		u.workerCount = len(uris)
	}

	// start the workers
	u.waitGroup.Add(u.workerCount)
	for n := 0; n < u.workerCount; n++ {
		go u.worker()
	}

	// start the work
	go func() {
		for _, uri := range uris {
			u.workChan <- uri
		}

		// close the work channel
		close(u.workChan)
	}()

	u.waitGroup.Wait()
	return u.err
}

// Responses returns the responses from the upstream calls
func (u *UpstreamWorker) Responses() []Done {
	return u.responses
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

		if err != nil && u.err == nil {
			u.err = err
		}
	}
	u.waitGroup.Done()
}
