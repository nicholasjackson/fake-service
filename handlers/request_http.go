package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/nicholasjackson/fake-service/client"
	"github.com/nicholasjackson/fake-service/timing"
	"github.com/nicholasjackson/fake-service/tracing"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
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
	message       string
	logger        hclog.Logger
	duration      *timing.RequestDuration
	upstreamURIs  []string
	workerCount   int
	defaultClient client.HTTP
	tracingClient tracing.Client
}

// NewRequest creates a new request handler
func NewRequest(
	name, message string,
	logger hclog.Logger,
	duration *timing.RequestDuration,
	upstreamURIs []string,
	workerCount int,
	defaultClient client.HTTP,
	tracingClient tracing.Client,
) *Request {

	return &Request{
		name:          name,
		message:       message,
		logger:        logger,
		duration:      duration,
		upstreamURIs:  upstreamURIs,
		workerCount:   workerCount,
		defaultClient: defaultClient,
		tracingClient: tracingClient,
	}
}

// Handle the request and call the upstream servers
func (rq *Request) Handle(rw http.ResponseWriter, r *http.Request) {
	rq.logger.Info("Handling request", "request", formatRequest(r))

	var serverSpan opentracing.Span
	wireContext, err := opentracing.GlobalTracer().Extract(
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(r.Header),
	)
	if err != nil {
		// Optionally record something about err here
		rq.logger.Error("Error obtaining context", "error", err)
	}

	// Create the span referring to the RPC client if available.
	// If wireContext == nil, a root span will be created.
	serverSpan = opentracing.StartSpan(
		"handle_request",
		ext.RPCServerOption(wireContext))

	defer serverSpan.Finish()

	// randomize the time the request takes
	d := rq.duration.Calculate()
	sp := serverSpan.Tracer().StartSpan(
		"service_delay",
		opentracing.ChildOf(serverSpan.Context()),
	)

	// wait for a predetermined time
	time.Sleep(d)

	sp.Finish()

	workChan := make(chan string)
	errChan := make(chan error)
	respChan := make(chan done)
	doneChan := make(chan struct{})

	// start the workers
	for n := 0; n < rq.workerCount; n++ {
		go rq.worker(serverSpan.Context(), workChan, respChan, errChan)
	}

	// create the wait group to signal when all processes are complete
	wg := sync.WaitGroup{}
	wg.Add(len(rq.upstreamURIs))

	// monitor the threads and send a message when done
	rq.monitorStatus(&wg, doneChan)

	// setup response capture
	responses := []done{}
	rq.captureResponses(respChan, &responses, &wg)

	// call the upstreams
	rq.doWork(workChan, rq.upstreamURIs)

	// wait for all threads to complete or an error to be raised
	select {
	case err := <-errChan:
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	case <-doneChan:
		rq.logger.Info("All workers complete")
	}

	data := rq.processResponses(responses)
	rw.Write(data)
}

func (rq *Request) doWork(workChan chan string, uris []string) {
	go func(workChan chan string) {
		for _, uri := range uris {
			workChan <- uri
		}
	}(workChan)
}

func (rq *Request) captureResponses(respChan chan done, responses *[]done, wg *sync.WaitGroup) {
	go func(respChan chan done, responses *[]done, wg *sync.WaitGroup) {
		for r := range respChan {
			rq.logger.Info("Done")
			*responses = append(*responses, r)
			wg.Done()
		}
	}(respChan, responses, wg)
}

//
func (rq *Request) monitorStatus(wg *sync.WaitGroup, doneChan chan struct{}) {
	go func(wg *sync.WaitGroup) {
		wg.Wait()
		doneChan <- struct{}{}
	}(wg)
}

func (rq *Request) worker(ctx opentracing.SpanContext, workChan chan string, respChan chan done, errChan chan error) {
	for {
		uri := <-workChan

		httpReq, _ := http.NewRequest("GET", uri, nil)

		clientSpan := opentracing.StartSpan(
			"call_upstream",
			opentracing.ChildOf(ctx),
		)

		ext.SpanKindRPCClient.Set(clientSpan)
		ext.HTTPUrl.Set(clientSpan, uri)
		ext.HTTPMethod.Set(clientSpan, "GET")

		// Transmit the span's TraceContext as HTTP headers on our
		// outbound request.
		opentracing.GlobalTracer().Inject(
			clientSpan.Context(),
			opentracing.HTTPHeaders,
			opentracing.HTTPHeadersCarrier(httpReq.Header))

		resp, err := rq.defaultClient.Do(httpReq)
		if err != nil {
			errChan <- err
			clientSpan.Finish()
			continue
		}

		clientSpan.Finish()
		respChan <- done{uri, resp}
	}
}

func (rq Request) processResponses(responses []done) []byte {
	respLines := []string{}
	respLines = append(respLines, fmt.Sprintf("# Reponse from: %s #", rq.name))
	respLines = append(respLines, rq.message)

	// append the output from the upstreams
	for _, r := range responses {
		respLines = append(respLines, fmt.Sprintf("## Called upstream uri: %s", r.uri))
		// indent the reposne from the upstream
		lines := strings.Split(string(r.data), "\n")
		for _, l := range lines {
			respLines = append(respLines, fmt.Sprintf("  %s", l))
		}
	}

	return []byte(strings.Join(respLines, "\n"))
}

// formatRequest generates ascii representation of a request
func formatRequest(r *http.Request) string {
	// Create return string
	var request []string
	// Add the request string
	url := fmt.Sprintf("%v %v %v", r.Method, r.URL, r.Proto)
	request = append(request, url)
	// Add the host
	request = append(request, fmt.Sprintf("Host: %v", r.Host))
	// Loop through headers
	for name, headers := range r.Header {
		name = strings.ToLower(name)
		for _, h := range headers {
			request = append(request, fmt.Sprintf("%v: %v", name, h))
		}
	}

	// If this is a POST, add post data
	if r.Method == "POST" {
		r.ParseForm()
		request = append(request, "\n")
		request = append(request, r.Form.Encode())
	}
	// Return the request as a string
	return strings.Join(request, "\n")
}
