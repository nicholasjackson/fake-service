package worker

import (
	"sync"

	"github.com/hashicorp/go-hclog"
)

// WorkFunc defines a function which is called when work is to be done
type WorkFunc func(uri string) (string, error)

// Done is a message sent when an upstream worker has completed
type Done struct {
	URI     string
	Message string
}

// UpstreamWorker manages parallel upstream requests
type UpstreamWorker struct {
	workerCount int
	workChan    chan string
	errChan     chan error
	doneChan    chan struct{}
	workFunc    WorkFunc
	waitGroup   *sync.WaitGroup
	logger      hclog.Logger
	responses   []Done
}

// New UpstreamWorker
func New(workerCount int, logger hclog.Logger, f WorkFunc) *UpstreamWorker {
	return &UpstreamWorker{
		workerCount: workerCount,
		workChan:    make(chan string),
		errChan:     make(chan error),
		doneChan:    make(chan struct{}),
		workFunc:    f,
		waitGroup:   &sync.WaitGroup{},
		logger:      logger,
		responses:   []Done{},
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
		uri := <-u.workChan
		u.logger.Debug("Starting Work", "uri", uri)

		resp, err := u.workFunc(uri)

		if err != nil {
			u.errChan <- err
			continue
		}

		u.responses = append(u.responses, Done{uri, resp})
		u.waitGroup.Done()

		u.logger.Debug("Finished Work", "uri", uri)
	}
}
