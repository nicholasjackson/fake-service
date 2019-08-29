package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/nicholasjackson/env"
	"github.com/nicholasjackson/fake-service/client"
	"github.com/nicholasjackson/fake-service/grpc/api"
	"github.com/nicholasjackson/fake-service/handlers"
	"github.com/nicholasjackson/fake-service/timing"
	"github.com/nicholasjackson/fake-service/tracing"
	"google.golang.org/grpc"
)

var upstreamURIs = env.String("UPSTREAM_URIS", false, "", "Comma separated URIs of the upstream services to call")
var upstreamWorkers = env.Int("UPSTREAM_WORKERS", false, 1, "Number of parallel workers for calling upstreams, defualt is 1 which is sequential operation")

var serviceType = env.String("SERVER_TYPE", false, "http", "Service type: [http or grpc], default:http. Determines the type of service HTTP or gRPC")
var message = env.String("MESSAGE", false, "Hello World", "Message to be returned from service")
var name = env.String("NAME", false, "Service", "Name of the service")

var listenAddress = env.String("LISTEN_ADDR", false, "0.0.0.0:9090", "IP address and port to bind service to")

// Upstream client configuration
var upstreamClientKeepAlives = env.Bool("HTTP_CLIENT_KEEP_ALIVES", false, true, "Enable HTTP connection keep alives for upstream calls")

// Service timing
var timing50Percentile = env.Duration("TIMING_50_PERCENTILE", false, time.Duration(0*time.Millisecond), "Median duration for a request")
var timing90Percentile = env.Duration("TIMING_90_PERCENTILE", false, time.Duration(0*time.Millisecond), "90 percentile duration for a request")
var timing99Percentile = env.Duration("TIMING_99_PERCENTILE", false, time.Duration(0*time.Millisecond), "99 percentile duration for a request")
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

	// build the map of gRPCClients
	grpcClients := make(map[string]client.GRPC)
	for _, u := range tidyURIs(*upstreamURIs) {
		//strip the grpc:// from the uri
		u2 := strings.TrimPrefix(u, "grpc://")

		c, err := client.NewGRPC(u2)
		if err != nil {
			logger.Error("Error creating GRPC client", "error", err)
			os.Exit(1)
		}

		grpcClients[u] = c
	}

	// do we need to setup tracing
	var tracingClient tracing.Client
	if *zipkinEndpoint != "" {
		tracingClient = tracing.NewOpenTracingClient(*zipkinEndpoint, *name, *listenAddress)
	}

	logger.Info(
		"Starting service",
		"name", *name,
		"message", *message,
		"upstreamURIs", *upstreamURIs,
		"upstreamWorkers", *upstreamWorkers,
		"listenAddress", *listenAddress,
		"http_client_keep_alives", *upstreamClientKeepAlives,
		"service type", *serviceType,
		"zipkin_endpoint", *zipkinEndpoint,
	)

	if *serviceType == "http" {
		rq := handlers.NewRequest(
			*name,
			*message,
			logger,
			rd,
			tidyURIs(*upstreamURIs),
			*upstreamWorkers,
			defaultClient,
			grpcClients,
			tracingClient,
		)

		hq := handlers.NewHealth(logger)

		http.HandleFunc("/", rq.Handle)
		http.HandleFunc("/health", hq.Handle)
		logger.Error(
			"Error starting service", "error",
			http.ListenAndServe(*listenAddress, nil))
	}

	if *serviceType == "grpc" {
		lis, err := net.Listen("tcp", *listenAddress)
		if err != nil {
			log.Fatalf("failed to listen: %v", err)
		}

		grpcServer := grpc.NewServer()
		fakeServer := handlers.NewFakeServer(
			*name,
			*message,
			rd,
			tidyURIs(*upstreamURIs),
			*upstreamWorkers,
			defaultClient,
			grpcClients,
			logger,
		)

		api.RegisterFakeServiceServer(grpcServer, fakeServer)
		grpcServer.Serve(lis)
	}
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
