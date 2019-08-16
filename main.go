package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/nicholasjackson/env"
	"github.com/nicholasjackson/fake-service/client"
	"github.com/nicholasjackson/fake-service/handlers"
	"github.com/nicholasjackson/fake-service/timing"
	"github.com/nicholasjackson/fake-service/tracing"
)

var upstreamURIs = env.String("UPSTREAM_URIS", false, "", "Comma separated URIs of the upstream services to call")
var upstreamWorkers = env.Int("UPSTREAM_WORKERS", false, 1, "Number of parallel workers for calling upstreams, defualt is 1 which is sequential operation")

var message = env.String("MESSAGE", false, "Hello World", "Message to be returned from service")
var name = env.String("NAME", false, "Service", "Name of the service")

var listenAddress = env.String("LISTEN_ADDR", false, "0.0.0.0:9090", "IP address and port to bind service to")

// Upstream client configuration
var upstreamClientKeepAlives = env.Bool("HTTP_CLIENT_KEEP_ALIVES", false, true, "Enable HTTP connection keep alives for upstream calls")

// Service timing
var timing50Percentile = env.Duration("TIMING_50_PERCENTILE", false, time.Duration(1*time.Millisecond), "Median duration for a request")
var timing90Percentile = env.Duration("TIMING_90_PERCENTILE", false, time.Duration(1*time.Millisecond), "90 percentile duration for a request")
var timing99Percentile = env.Duration("TIMING_99_PERCENTILE", false, time.Duration(1*time.Millisecond), "99 percentile duration for a request")
var timingVariance = env.Int("TIMING_VARIANCE", false, 0, "Percentage variance for each request, every request will vary by a random amount to a maximum of a percentage of the total request time")

// performance testing flags
// these flags allow the user to inject faults into the service for testing purposes
var errorRate = env.Float64("ERROR_RATE", false, 0.0, "Percentage of request where handler will report an error")
var errorType = env.String("ERROR_TYPE", false, "http_error", "Type of error [http_error, delay]")
var errorCode = env.Int("ERROR_CODE", false, http.StatusInternalServerError, "Error code to return on error")
var errorDelay = env.Duration("ERROR_DELAY", false, 0*time.Second, "Error delay [1s,100ms]")

// metrics
var zipkinEndpoint = env.String("TRACING_ZIPKIN", false, "", "Location of Zipkin tracing collector")

var logger hclog.Logger

var help = flag.Bool("help", false, "--help to show help")

var version = "dev"

func main() {

	logger = hclog.Default()

	env.Parse()
	flag.Parse()

	// if the help flag is passed show configuration options
	if *help == true {
		fmt.Println("Fake service version:", version)
		fmt.Println("Configuration values are set using environment variables, for info please see the following list:")
		fmt.Println("")
		fmt.Println(env.Help())
		os.Exit(0)
	}

	rd := timing.NewRequestDuration(
		*timing50Percentile,
		*timing90Percentile,
		*timing99Percentile,
		*timingVariance,
	)

	// create the httpClient
	defaultClient := client.NewHTTP(*upstreamClientKeepAlives)

	// do we need to setup tracing
	var tracingClient tracing.Client
	if *zipkinEndpoint != "" {
		tracingClient = tracing.NewOpenTracingClient(*zipkinEndpoint, *name, *listenAddress)
	}

	rq := handlers.NewRequest(
		*name,
		*message,
		logger,
		rd,
		tidyURIs(*upstreamURIs),
		*upstreamWorkers,
		defaultClient,
		tracingClient,
	)

	hq := handlers.NewHealth(logger)

	http.HandleFunc("/", rq.Handle)
	http.HandleFunc("/health", hq.Handle)

	logger.Info(
		"Starting service",
		"name", *name,
		"message", *message,
		"upstreamURIs", *upstreamURIs,
		"upstreamWorkers", *upstreamWorkers,
		"listenAddress", *listenAddress,
		"http_client_keep_alives", *upstreamClientKeepAlives,
		"zipkin_endpoint", *zipkinEndpoint,
	)

	logger.Error(
		"Error starting service", "error",
		http.ListenAndServe(*listenAddress, nil))
}

// tidyURIs splits the upstream URIs passed by environment variable and reuturns
// a sanitised slice
func tidyURIs(uris string) []string {
	resp := []string{}
	rawResp := strings.Split(uris, ",")

	for _, r := range rawResp {
		r = strings.Trim(r, " ")
		if r != "" {
			resp = append(resp, r)
		}
	}

	return resp
}
