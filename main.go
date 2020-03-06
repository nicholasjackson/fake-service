package main

import (
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gobuffalo/packr/v2"
	"github.com/hashicorp/go-hclog"
	"github.com/nicholasjackson/env"
	"github.com/nicholasjackson/fake-service/client"
	"github.com/nicholasjackson/fake-service/errors"
	"github.com/nicholasjackson/fake-service/grpc/api"
	"github.com/nicholasjackson/fake-service/handlers"
	"github.com/nicholasjackson/fake-service/load"
	"github.com/nicholasjackson/fake-service/logging"
	"github.com/nicholasjackson/fake-service/timing"
	"github.com/nicholasjackson/fake-service/tracing"
	"github.com/rs/cors"
	"google.golang.org/grpc"
)

var upstreamURIs = env.String("UPSTREAM_URIS", false, "", "Comma separated URIs of the upstream services to call")
var upstreamWorkers = env.Int("UPSTREAM_WORKERS", false, 1, "Number of parallel workers for calling upstreams, defualt is 1 which is sequential operation")

var serviceType = env.String("SERVER_TYPE", false, "http", "Service type: [http or grpc], default:http. Determines the type of service HTTP or gRPC")
var message = env.String("MESSAGE", false, "Hello World", "Message to be returned from service")
var name = env.String("NAME", false, "Service", "Name of the service")

var listenAddress = env.String("LISTEN_ADDR", false, "0.0.0.0:9090", "IP address and port to bind service to")

// Upstream client configuration
var upstreamClientKeepAlives = env.Bool("HTTP_CLIENT_KEEP_ALIVES", false, false, "Enable HTTP connection keep alives for upstream calls")
var upstreamAppendRequest = env.Bool("HTTP_CLIENT_APPEND_REQUEST", false, true, "When true the path, querystring, and any headers sent to the service will be appended to any upstream calls")
var upstreamRequestTimeout = env.Duration("HTTP_CLIENT_REQUEST_TIMEOUT", false, 30*time.Second, "Max time to wait before timeout for upstream requests, default 30s")

// Service timing
var timing50Percentile = env.Duration("TIMING_50_PERCENTILE", false, time.Duration(0*time.Millisecond), "Median duration for a request")
var timing90Percentile = env.Duration("TIMING_90_PERCENTILE", false, time.Duration(0*time.Millisecond), "90 percentile duration for a request, if no value is set, will use value from TIMING_50_PERCENTILE")
var timing99Percentile = env.Duration("TIMING_99_PERCENTILE", false, time.Duration(0*time.Millisecond), "99 percentile duration for a request, if no value is set, will use value from TIMING_90_PERCENTILE")
var timingVariance = env.Int("TIMING_VARIANCE", false, 0, "Percentage variance for each request, every request will vary by a random amount to a maximum of a percentage of the total request time")

// performance testing flags
// these flags allow the user to inject faults into the service for testing purposes
var errorRate = env.Float64("ERROR_RATE", false, 0.0, "Decimal percentage of request where handler will report an error. e.g. 0.1 = 10% of all requests will result in an error")
var errorType = env.String("ERROR_TYPE", false, "http_error", "Type of error [http_error, delay]")
var errorCode = env.Int("ERROR_CODE", false, http.StatusInternalServerError, "Error code to return on error")
var errorDelay = env.Duration("ERROR_DELAY", false, 0*time.Second, "Error delay [1s,100ms]")

// rate limit request to the service
var rateLimitRPS = env.Float64("RATE_LIMIT", false, 0.0, "Rate in req/second after which service will return an error code")
var rateLimitCode = env.Int("RATE_LIMIT_CODE", false, 503, "Code to return when service call is rate limited")

// load generation
var loadCPUCores = env.Int("LOAD_CPU_CORES", false, 0, "Number of cores to generate fake CPU load over")
var loadCPUPercentage = env.Int("LOAD_CPU_PERCENTAGE", false, 0, "Percentage of CPU cores to consume as a percentage. I.e: 50, 50% load for LOAD_CPU_CORES")

// metrics / tracing / logging
var zipkinEndpoint = env.String("TRACING_ZIPKIN", false, "", "Location of Zipkin tracing collector")
var datadogTracingEndpoint = env.String("TRACING_DATADOG", false, "", "Location of Datadog tracing collector")
var datadogMetricsEndpoint = env.String("METRICS_DATADOG", false, "", "Location of Datadog metrics collector")
var logFormat = env.String("LOG_FORMAT", false, "text", "Log file format. [text|json]")
var logLevel = env.String("LOG_LEVEL", false, "info", "Log level for output. [info|debug|trace|warn|error]")
var logOutput = env.String("LOG_OUTPUT", false, "stdout", "Location to write log output, default is stdout, e.g. /var/log/web.log")

var version = "dev"

func main() {
	env.Parse()

	var sdf tracing.SpanDetailsFunc

	// do we need to setup tracing
	if *zipkinEndpoint != "" {
		tracing.NewOpenTracingClient(*zipkinEndpoint, *name, *listenAddress)
		sdf = tracing.GetZipkinSpanDetails
	}

	if *datadogTracingEndpoint != "" {
		tracing.NewDataDogClient(*datadogTracingEndpoint, *name)
		sdf = tracing.GetDataDogSpanDetails
	}

	// do we need to setup metrics
	var metrics logging.Metrics = &logging.NullMetrics{}

	if *datadogMetricsEndpoint != "" {
		metrics = logging.NewStatsDMetrics(*name, "production", *datadogMetricsEndpoint)
	}

	lo := hclog.DefaultOptions
	lo.Level = hclog.LevelFromString(*logLevel) // set the log level

	// set the log format
	if *logFormat == "json" {
		lo.JSONFormat = true
	}

	switch *logOutput {
	case "stdout":
		lo.Output = os.Stdout
	case "stderr":
		lo.Output = os.Stderr
	default:
		f, err := os.Create(*logOutput)
		if err != nil {
			panic(err)
		}

		lo.Output = f
	}

	logger := logging.NewLogger(metrics, hclog.New(lo), sdf)

	rd := timing.NewRequestDuration(
		*timing50Percentile,
		*timing90Percentile,
		*timing99Percentile,
		*timingVariance,
	)

	// create the error injector
	errorInjector := errors.NewInjector(
		logger.Log().Named("error_injector"),
		*errorRate,
		*errorCode,
		*errorType,
		*errorDelay,
		*rateLimitRPS,
		*rateLimitCode,
	)

	// create the load generator
	generator := load.NewGenerator(*loadCPUCores, *loadCPUPercentage)

	// create the httpClient
	defaultClient := client.NewHTTP(*upstreamClientKeepAlives, *upstreamAppendRequest, *upstreamRequestTimeout)

	// build the map of gRPCClients
	grpcClients := make(map[string]client.GRPC)
	for _, u := range tidyURIs(*upstreamURIs) {
		//strip the grpc:// from the uri
		u2 := strings.TrimPrefix(u, "grpc://")

		c, err := client.NewGRPC(u2, *upstreamRequestTimeout)
		if err != nil {
			logger.Log().Error("Error creating GRPC client", "error", err)
			os.Exit(1)
		}

		grpcClients[u] = c
	}

	logger.ServiceStarted(*name, *upstreamURIs, *upstreamWorkers, *listenAddress, *serviceType)

	if *serviceType == "http" {

		rq := handlers.NewRequest(
			*name,
			*message,
			rd,
			tidyURIs(*upstreamURIs),
			*upstreamWorkers,
			defaultClient,
			grpcClients,
			errorInjector,
			generator,
			logger,
		)

		hq := handlers.NewHealth(logger)

		mux := http.NewServeMux()

		// add the static files
		logger.Log().Info("Adding handler for UI static files")
		box := packr.New("ui", "./ui/build")
		for _, f := range box.List() {
			logger.Log().Info("File", "path", f)
		}
		mux.Handle("/ui/", http.StripPrefix("/ui", http.FileServer(box)))

		mux.HandleFunc("/health", hq.Handle)
		mux.HandleFunc("/", rq.Handle)

		// CORS handler
		hc := cors.Default().Handler(mux)

		err := http.ListenAndServe(*listenAddress, hc)

		if err != nil {
			logger.Log().Error("Error starting service", "address", *listenAddress, "error", err)
		}
	}

	if *serviceType == "grpc" {
		lis, err := net.Listen("tcp", *listenAddress)
		if err != nil {
			logger.Log().Error("failed to create lister", "address", *listenAddress, "error", err)
			os.Exit(1)
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
			errorInjector,
			generator,
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

// return the ip addresses for this service
