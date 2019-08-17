package worker

import (
	"sync"

	"github.com/hashicorp/go-hclog"
)

// WorkFunc defines a function which is called when work is to be done
type WorkFunc func(uri string) error

// done is a message sent when an upstream worker has completed
type done struct {
	uri  string
	data []byte
}

// UpstreamWorker manages parallel upstream requests
type UpstreamWorker struct {
	workerCount int
	workChan    chan string
	errChan     chan error
	respChan    chan done
	doneChan    chan struct{}
	workFunc    WorkFunc
	waitGroup   *sync.WaitGroup
	logger      hclog.Logger
}

// New UpstreamWorker
func New(workerCount int, logger hclog.Logger, f WorkFunc) *UpstreamWorker {
	return &UpstreamWorker{
		workerCount: workerCount,
		workChan:    make(chan string),
		errChan:     make(chan error),
		respChan:    make(chan done),
		doneChan:    make(chan struct{}),
		workFunc:    f,
		waitGroup:   &sync.WaitGroup{},
		logger:      logger,
	}
}

// Do runs the worker with the given uris
func (u *UpstreamWorker) Do(uris []string) error {

	// start the workers
	u.logger.Debug("Starting workers", "count", u.workerCount)
	for n := 0; n < u.workerCount; n++ {
		go u.worker()
	}

	u.waitGroup.Add(len(uris))

	// monitor the threads and send a message when done
	u.monitorStatus()

	// setup response capture
	u.captureResponses()

	// start the work
	go func() {
		for _, uri := range uris {
			u.workChan <- uri
		}
	}()

	select {
	case err := <-u.errChan:
		return err
	case <-u.doneChan:
		return nil
	}
}

func (u *UpstreamWorker) captureResponses() {
	go func() {
		for range u.respChan {
			u.waitGroup.Done()
		}
	}()
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
		uri := <-u.workChan
		u.logger.Debug("Starting Work", "uri", uri)

		err := u.workFunc(uri)

		if err != nil {
			u.errChan <- err
			continue
		}

		u.logger.Debug("Finished Work", "uri", uri)
		u.respChan <- done{uri, nil}
	}
}
