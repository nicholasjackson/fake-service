package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nicholasjackson/fake-service/client"
	"github.com/nicholasjackson/fake-service/errors"
	"github.com/nicholasjackson/fake-service/load"
	"github.com/nicholasjackson/fake-service/logging"
	"github.com/nicholasjackson/fake-service/response"
	"github.com/nicholasjackson/fake-service/timing"
	"github.com/nicholasjackson/fake-service/worker"
)

// done is a message sent when an upstream worker has completed
type done struct {
	uri  string
	data []byte
}

// Request handles inbound requests and makes any necessary upstream calls
type Request struct {
	// name of the service
	name string
	// message to return to caller
	message          string
	duration         *timing.RequestDuration
	upstreamURIs     []string
	workerCount      int
	defaultClient    client.HTTP
	grpcClients      map[string]client.GRPC
	errorInjector    *errors.Injector
	loadGenerator    *load.Generator
	log              *logging.Logger
	requestGenerator load.RequestGenerator
	waitTillReady    bool
	readinessHandler *Ready
}

// NewRequest creates a new request handler
func NewRequest(
	name, message string,
	duration *timing.RequestDuration,
	upstreamURIs []string,
	workerCount int,
	defaultClient client.HTTP,
	grpcClients map[string]client.GRPC,
	errorInjector *errors.Injector,
	loadGenerator *load.Generator,
	log *logging.Logger,
	requestGenerator load.RequestGenerator,
	waitTillReady bool,
	readinessHandler *Ready,
) *Request {

	return &Request{
		name:             name,
		message:          message,
		duration:         duration,
		upstreamURIs:     upstreamURIs,
		workerCount:      workerCount,
		defaultClient:    defaultClient,
		grpcClients:      grpcClients,
		errorInjector:    errorInjector,
		loadGenerator:    loadGenerator,
		log:              log,
		requestGenerator: requestGenerator,
		waitTillReady:    waitTillReady,
		readinessHandler: readinessHandler,
	}
}

// Handle the request and call the upstream servers
func (rq *Request) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	if rq.waitTillReady && !rq.readinessHandler.Complete() {
		rq.log.Log().Info("Service not ready")
		rw.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	// generate 100% CPU load for service
	finished := rq.loadGenerator.Generate()
	defer finished()

	// start timing the service this is used later for the total request time
	ts := time.Now()

	// log start request
	hq := rq.log.HandleHTTPRequest(r)
	defer hq.Finished()

	resp := &response.Response{}
	resp.Name = rq.name
	resp.Type = "HTTP"
	resp.URI = r.URL.String()
	resp.IPAddresses = getIPInfo()

	// are we injecting errors, if so return the error
	if er := rq.errorInjector.Do(); er != nil {
		resp.Code = er.Code
		resp.Error = er.Error.Error()

		// log the error response
		hq.SetError(er.Error)
		hq.SetMetadata("response", strconv.Itoa(er.Code))

		rw.WriteHeader(er.Code)
		rw.Write([]byte(resp.ToJSON()))
		return
	}

	// if we need to create upstream requests create a worker pool
	var upstreamError error
	if len(rq.upstreamURIs) > 0 {
		body := rq.requestGenerator.Generate()
		wp := worker.New(rq.workerCount, func(uri string) (*response.Response, error) {
			if strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://") {
				return workerHTTP(hq.Span.Context(), uri, rq.defaultClient, r, rq.log, body)
			}

			return workerGRPC(hq.Span.Context(), uri, rq.grpcClients, rq.log, body)
		})

		err := wp.Do(rq.upstreamURIs)

		if err != nil {
			upstreamError = err
		}

		for _, v := range wp.Responses() {
			resp.AppendUpstream(v.URI, *v.Response)
		}
	}

	if upstreamError != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		resp.Code = http.StatusInternalServerError

		// log error
		hq.SetMetadata("response", strconv.Itoa(http.StatusInternalServerError))
		hq.SetError(upstreamError)
	} else {
		// service time is equal to the randomised time - the current time take
		d := rq.duration.Calculate()
		et := time.Now().Sub(ts)
		rd := d - et
		if rd > 0 {
			// randomize the time the request takes if no error
			lp := rq.log.SleepService(hq.Span, rd)
			time.Sleep(rd)
			lp.Finished()
		}

		resp.Code = http.StatusOK

		// log response code
		hq.SetMetadata("response", strconv.Itoa(http.StatusOK))
	}

	// compute total elapsed time including delay
	te := time.Now()
	resp.StartTime = ts.Format(timeFormat)
	resp.EndTime = te.Format(timeFormat)
	resp.Duration = te.Sub(ts).String()

	// add the response body
	if strings.HasPrefix(rq.message, "{") {
		resp.Body = json.RawMessage(rq.message)
	} else {
		resp.Body = json.RawMessage(fmt.Sprintf(`"%s"`, rq.message))
	}

	rw.Write([]byte(resp.ToJSON()))
}
