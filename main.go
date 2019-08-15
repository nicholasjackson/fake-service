package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/nicholasjackson/env"
	"github.com/nicholasjackson/upstream-echo/timing"
)

var upstreamURIs = env.String("UPSTREAM_URIS", false, "", "Comma separated URIs of the upstream services to call")
var upstreamWorkers = env.Int("UPSTREAM_WORKERS", false, 1, "Number of parallel workers for calling upstreams, defualt is 1 which is sequential operation")

var message = env.String("MESSAGE", false, "Hello World", "Message to be returned from service")
var name = env.String("NAME", false, "Service", "Name of the service")

var listenAddress = env.String("LISTEN_ADDR", false, ":9090", "IP address and port to bind service to")

// Upstream client configuration
var upstreamClientKeepAlives = env.Bool("HTTP_CLIENT_KEEP_ALIVES", false, true, "Enable HTTP connection keep alives for upstream calls")

// Service timing
var timing50Percentile = env.Duration("TIMING_50_PERCENTILE", false, time.Duration(1*time.Millisecond), "Median duration for a request")
var timing90Percentile = env.Duration("TIMING_90_PERCENTILE", false, time.Duration(1*time.Millisecond), "90 percentile duration for a request")
var timing99Percentile = env.Duration("TIMING_99_PERCENTILE", false, time.Duration(1*time.Millisecond), "99 percentile duration for a request")
var timingVariance = env.Float64("TIMING_VARIANCE", false, 0, "Decimal percentage variance for each request, every request will vary by a random amount to a maximum of a percentage of the total request time")

var logger hclog.Logger

var defaultClient *http.Client
var requestDuration *timing.RequestDuration

func main() {

	logger = hclog.Default()

	env.Parse()

	requestDuration = timing.NewRequestDuration(
		*timing50Percentile,
		*timing90Percentile,
		*timing99Percentile,
		*timingVariance,
	)

	// create the httpClient
	defaultClient = createClient()

	http.HandleFunc("/", requestHandler)
	http.HandleFunc("/health", healthHandler)

	logger.Info(
		"Starting service",
		"message", *message,
		"upstreamURIs", *upstreamURIs,
		"upstreamWorkers", *upstreamWorkers,
		"listenAddress", *listenAddress,
		"http_client_keep_alives", *upstreamClientKeepAlives,
	)

	logger.Error("Error starting service", "error", http.ListenAndServe(*listenAddress, nil))
}

func createClient() *http.Client {
	client := &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: !*upstreamClientKeepAlives,
		},
	}

	return client
}

type done struct {
	uri  string
	data []byte
}

func requestHandler(rw http.ResponseWriter, r *http.Request) {
	logger.Info("Handling request", "request", formatRequest(r))

	// randomize the time the request takes
	time.Sleep(requestDuration.Calculate())

	workers := *upstreamWorkers
	workChan := make(chan string)
	errChan := make(chan error)
	respChan := make(chan done)
	doneChan := make(chan struct{})

	// start the workers
	for n := 0; n < workers; n++ {
		go worker(workChan, respChan, errChan)
	}

	uris := tidyURIs(*upstreamURIs)

	// create the wait group to signal when all processes are complete
	wg := sync.WaitGroup{}
	wg.Add(len(uris))

	// monitor the threads and send a message when done
	monitorStatus(&wg, doneChan)

	// setup response capture
	responses := []done{}
	captureResponses(respChan, &responses, &wg)

	// process all the uris
	doWork(workChan, uris)

	// wait for all threads to complete or an error to be raised
	select {
	case err := <-errChan:
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	case <-doneChan:
		logger.Info("All workers complete")
	}

	data := processResponses(responses)
	rw.Write(data)
}

func tidyURIs(uris string) []string {
	resp := []string{}
	rawResp := strings.Split(*upstreamURIs, ",")

	for _, r := range rawResp {
		r = strings.Trim(r, " ")
		if r != "" {
			resp = append(resp, r)
		}
	}

	return resp
}

func doWork(workChan chan string, uris []string) {
	go func(workChan chan string) {
		for _, uri := range uris {
			uri = strings.Trim(uri, " ")

			if uri == "" {
				continue
			}

			workChan <- uri
		}
	}(workChan)
}

func captureResponses(respChan chan done, responses *[]done, wg *sync.WaitGroup) {
	go func(respChan chan done, responses *[]done, wg *sync.WaitGroup) {
		for r := range respChan {
			logger.Info("Done")
			*responses = append(*responses, r)
			wg.Done()
		}
	}(respChan, responses, wg)
}

func monitorStatus(wg *sync.WaitGroup, doneChan chan struct{}) {
	go func(wg *sync.WaitGroup) {
		wg.Wait()
		doneChan <- struct{}{}
	}(wg)
}

func processResponses(responses []done) []byte {
	respLines := []string{}
	respLines = append(respLines, fmt.Sprintf("# Reponse from: %s #", *name))
	respLines = append(respLines, *message)

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

func worker(workChan chan string, respChan chan done, errChan chan error) {
	for {
		uri := <-workChan

		resp, err := callUpstream(uri)
		if err != nil {
			errChan <- err
			continue
		}

		respChan <- done{uri, resp}
	}
}

func callUpstream(uri string) ([]byte, error) {
	var data []byte

	// call the upstream service
	resp, err := defaultClient.Get(uri)
	if err != nil {
		logger.Error("Error communicating with upstream service", "error", err)

		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		logger.Error("Expected status 200 from service got", "status", resp.StatusCode)

		return nil, err
	}

	defer resp.Body.Close()

	data, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Error reading response body", "error", err)

		return nil, err
	}

	logger.Info("Received response from upstream", "response", string(data))

	return data, nil
}

func healthHandler(rw http.ResponseWriter, r *http.Request) {
	logger.Info("Handling health request")

	fmt.Fprint(rw, "OK")
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
