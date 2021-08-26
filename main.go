package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
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

	cors "github.com/gorilla/handlers"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	//"net/http/pprof"
)

var upstreamURIs = env.String("UPSTREAM_URIS", false, "", "Comma separated URIs of the upstream services to call")
var upstreamAllowInsecure = env.Bool("UPSTREAM_ALLOW_INSECURE", false, false, "Allow calls to upstream servers, ignoring TLS certificate validation")
var upstreamWorkers = env.Int("UPSTREAM_WORKERS", false, 1, "Number of parallel workers for calling upstreams, defualt is 1 which is sequential operation")

var serviceType = env.String("SERVER_TYPE", false, "http", "Service type: [http or grpc], default:http. Determines the type of service HTTP or gRPC")
var message = env.String("MESSAGE", false, "Hello World", "Message to be returned from service")
var name = env.String("NAME", false, "Service", "Name of the service")

var listenAddress = env.String("LISTEN_ADDR", false, "0.0.0.0:9090", "IP address and port to bind service to")

var allowedOrigins = env.String("ALLOWED_ORIGINS", false, "*", "Comma separated list of allowed origins for CORS requests")
var allowedHeaders = env.String("ALLOWED_HEADERS", false, "Accept,Accept-Language,Content-Language,Origin,Content-Type", "Comma separated list of allowed headers for cors requests")
var allowCredentials = env.Bool("ALLOW_CREDENTIALS", false, false, "Are credentials allowed for CORS requests")

// Upstream client configuration
var upstreamClientKeepAlives = env.Bool("HTTP_CLIENT_KEEP_ALIVES", false, false, "Enable HTTP connection keep alives for upstream calls, also enables the HTTP servers handling of keep alives.")
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
var loadCPUAllocated = env.Float64("LOAD_CPU_ALLOCATED", false, 0, "MHz of CPU allocated to the service, when specified, load percentage is a percentage of CPU allocated")
var loadCPUClockSpeed = env.Float64("LOAD_CPU_CLOCK_SPEED", false, 1000, "MHz of a Single logical core, default 1000Mhz")
var loadCPUCores = env.Float64("LOAD_CPU_CORES", false, -1, "Number of cores to generate fake CPU load over, by default fake-service will use all cores")
var loadCPUPercentage = env.Float64("LOAD_CPU_PERCENTAGE", false, 0, "Percentage of CPU cores to consume as a percentage. I.e: 50, 50% load for LOAD_CPU_CORES. If LOAD_CPU_ALLOCATED is not specified CPU percentage is based on the Total CPU available")

var loadMemoryAllocated = env.Int("LOAD_MEMORY_PER_REQUEST", false, 0, "Memory in bytes consumed per request")
var loadMemoryVariance = env.Int("LOAD_MEMORY_VARIANCE", false, 0, "Percentage variance of the memory consumed per request, i.e with a value of 50 = 50%, and given a LOAD_MEMORY_PER_REQUEST of 1024 bytes, actual consumption per request would be in the range 516 - 1540 bytes")

var loadRequestBody = env.String("LOAD_REQUEST_BODY", false, "", "The body of the request to send (ignores other LOAD_REQUEST_* params if this is set")
var loadRequestSize = env.Int("LOAD_REQUEST_SIZE", false, 0, "Size of the request sent")
var loadRequestVariance = env.Int("LOAD_REQUEST_VARIANCE", false, 0, "Percentage variance of the request size")

// metrics / tracing / logging
var zipkinEndpoint = env.String("TRACING_ZIPKIN", false, "", "Location of Zipkin tracing collector")
var datadogTracingEndpointHost = env.String("TRACING_DATADOG_HOST", false, "", "Hostname or IP for Datadog tracing collector")
var datadogTracingEndpointPort = env.String("TRACING_DATADOG_PORT", false, "8126", "Port for Datadog tracing collector")
var datadogMetricsEndpointHost = env.String("METRICS_DATADOG_HOST", false, "", "Hostname or IP for Datadog metrics collector")
var datadogMetricsEndpointPort = env.String("METRICS_DATADOG_PORT", false, "8125", "Port for Datadog metrics collector")
var datadogMetricsEnvironment = env.String("METRICS_DATADOG_ENVIRONMENT", false, "production", "Environment tag for Datadog metrics collector")
var logFormat = env.String("LOG_FORMAT", false, "text", "Log file format. [text|json]")
var logLevel = env.String("LOG_LEVEL", false, "info", "Log level for output. [info|debug|trace|warn|error]")
var logOutput = env.String("LOG_OUTPUT", false, "stdout", "Location to write log output, default is stdout, e.g. /var/log/web.log")

// TLS Certs
var tlsCertificate = env.String("TLS_CERT_LOCATION", false, "", "Location of PEM encoded x.509 certificate for securing server")
var tlsKey = env.String("TLS_KEY_LOCATION", false, "", "Location of PEM encoded private key for securing server")

var healthResponseCode = env.Int("HEALTH_CHECK_RESPONSE_CODE", false, 200, "Response code returned from the HTTP health check at /health")

var readySuccessResponseCode = env.Int("READY_CHECK_RESPONSE_SUCCESS_CODE", false, 200, "Response code returned from the HTTP readiness handler `/ready` after the response delay has elapsed")
var readyFailureResponseCode = env.Int("READY_CHECK_RESPONSE_FAILURE_CODE", false, 503, "Response code returned from the HTTP readiness handler `/ready` before the response delay has elapsed, this simulates the response code a service would return while starting")
var readyResponseDelay = env.Duration("READY_CHECK_RESPONSE_DELAY", false, 0*time.Second, "Delay before the readyness check returns the READY_CHECK_RESPONSE_CODE")
var seed = env.Int("RAND_SEED", false, int(time.Now().Unix()), "A seed to initialize the random number generators")

var version = "dev"

func main() {
	env.Parse()

	var sdf tracing.SpanDetailsFunc

	// do we need to setup tracing
	if *zipkinEndpoint != "" {
		tracing.NewOpenTracingClient(*zipkinEndpoint, *name, *listenAddress)
		sdf = tracing.GetZipkinSpanDetails
	}

	if *datadogTracingEndpointHost != "" {
		hostname := fmt.Sprintf("%s:%s", *datadogTracingEndpointHost, *datadogTracingEndpointPort)
		tracing.NewDataDogClient(hostname, *name)
		sdf = tracing.GetDataDogSpanDetails
	}

	// do we need to setup metrics
	var metrics logging.Metrics = &logging.NullMetrics{}

	if *datadogMetricsEndpointHost != "" {
		hostname := fmt.Sprintf("%s:%s", *datadogMetricsEndpointHost, *datadogMetricsEndpointPort)
		metrics = logging.NewStatsDMetrics(*name, *datadogMetricsEnvironment, hostname)
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
	logger.Log().Info("Using seed", "seed", *seed)

	requestDuration := timing.NewRequestDuration(
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
	// get the total CPU amount
	// If original CPU percent is 10, however the service has only been allocated 10% of the available CPU then percent should be 1 as it is total of avaiable
	// Allocated Percentage = Allocated / (Max * Cores) * Percentage
	// 100 / (1000 * 10) * 10 = 1

	if *loadCPUCores == -1 {
		*loadCPUCores = float64(runtime.NumCPU())
	}

	if *loadCPUAllocated != 0 {
		*loadCPUPercentage = *loadCPUAllocated / (*loadCPUClockSpeed * *loadCPUCores) * *loadCPUPercentage
	}

	// create a generator that will be used to create memory and CPU load per request
	generator := load.NewGenerator(*loadCPUCores, *loadCPUPercentage, *loadMemoryAllocated, *loadMemoryVariance, logger.Log().Named("load_generator"))
	requestGenerator := load.NewRequestGenerator(*loadRequestBody, *loadRequestSize, *loadRequestVariance, int64(*seed))

	// create the httpClient
	defaultClient := client.NewHTTP(*upstreamClientKeepAlives, *upstreamAppendRequest, *upstreamRequestTimeout, *upstreamAllowInsecure)

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

	var httpServer *http.Server
	var grpcServer *grpc.Server

	switch *serviceType {
	case "http":
		httpServer = startupHTTP(logger, requestDuration, errorInjector, generator, grpcClients, defaultClient, requestGenerator)
	case "grpc":
		grpcServer = startupGRPC(logger, requestDuration, errorInjector, generator, grpcClients, defaultClient, requestGenerator)
	}

	// trap sigterm or interupt and gracefully shutdown the server
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, os.Kill)

	// Block until a signal is received.
	sig := <-c
	log.Println("Graceful shutdown, got signal:", sig)

	// gracefully shutdown the server, waiting max 30 seconds for current operations to complete

	switch *serviceType {
	case "http":
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		httpServer.Shutdown(ctx)
	case "grpc":
		// if the server does not gracefully stop after 30s, kill it
		timer := time.AfterFunc(30*time.Second, func() {
			grpcServer.Stop() // force stop the server
		})

		grpcServer.GracefulStop()
		timer.Stop()
	}
}

func startupHTTP(
	logger *logging.Logger,
	rd *timing.RequestDuration,
	errorInjector *errors.Injector,
	generator *load.Generator,
	grpcClients map[string]client.GRPC,
	defaultClient client.HTTP,
	requestGenerator load.RequestGenerator,
) *http.Server {

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
		requestGenerator,
	)

	hh := handlers.NewHealth(logger, *healthResponseCode)
	rh := handlers.NewReady(logger, *readySuccessResponseCode, *readyFailureResponseCode, *readyResponseDelay)

	mux := http.NewServeMux()

	// add the static files
	logger.Log().Info("Adding handler for UI static files")
	box := packr.New("ui", "./ui/build")
	for _, f := range box.List() {
		logger.Log().Info("File", "path", f)
	}

	// Add the User interface handler
	mux.Handle("/ui/", http.StripPrefix("/ui", http.FileServer(box)))

	// Add the generic health and ready handlers
	mux.HandleFunc("/health", hh.Handle)
	mux.HandleFunc("/ready", rh.Handle)

	// uncomment to enable pprof
	//mux.HandleFunc("/debug/pprof/", pprof.Index)
	//mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	//mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	//mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	//mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	mux.HandleFunc("/", rq.Handle)

	// CORS
	corsOptions := make([]cors.CORSOption, 0)
	if *allowedOrigins != "" {
		corsOptions = append(corsOptions, cors.AllowedOrigins(strings.Split(*allowedOrigins, ",")))
	}

	if *allowedHeaders != "" {
		corsOptions = append(corsOptions, cors.AllowedHeaders(strings.Split(*allowedHeaders, ",")))
	}

	if *allowCredentials {
		corsOptions = append(corsOptions, cors.AllowCredentials())
	}

	logger.Log().Info("Settings CORS options", "allow_creds", *allowCredentials, "allow_headers", *allowedHeaders, "allow_origins", *allowedOrigins)
	ch := cors.CORS(corsOptions...)

	var err error
	server := &http.Server{Addr: *listenAddress, Handler: ch(mux)}
	server.SetKeepAlivesEnabled(*upstreamClientKeepAlives)

	go func() {
		if *tlsCertificate != "" && *tlsKey != "" {
			logger.Log().Info("Enabling TLS")
			err = server.ListenAndServeTLS(*tlsCertificate, *tlsKey)
		} else {
			err = server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			logger.Log().Error("Error starting service", "address", *listenAddress, "error", err)
			os.Exit(1)
		}
	}()

	return server
}

func startupGRPC(
	logger *logging.Logger,
	rd *timing.RequestDuration,
	errorInjector *errors.Injector,
	generator *load.Generator,
	grpcClients map[string]client.GRPC,
	defaultClient client.HTTP,
	requestGenerator load.RequestGenerator,
) *grpc.Server {

	lis, err := net.Listen("tcp", *listenAddress)
	if err != nil {
		logger.Log().Error("failed to create lister", "address", *listenAddress, "error", err)
		os.Exit(1)
	}

	serverOptions := []grpc.ServerOption{}

	// disable keep alives
	if !*upstreamClientKeepAlives {
		serverOptions = append(serverOptions, grpc.KeepaliveParams(keepalive.ServerParameters{MaxConnectionAge: 5 * time.Second}))
	}

	if *tlsCertificate != "" && *tlsKey != "" {
		creds, err := credentials.NewServerTLSFromFile(*tlsCertificate, *tlsKey)
		if err != nil {
			log.Fatalf("Failed to setup TLS: %v", err)
		}
		serverOptions = append(serverOptions, grpc.Creds(creds))
	}

	grpcServer := grpc.NewServer(serverOptions...)

	// register the reflection service which allows clients to determine the methods
	// for this gRPC service
	reflection.Register(grpcServer)

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
		requestGenerator,
	)

	api.RegisterFakeServiceServer(grpcServer, fakeServer)
	go grpcServer.Serve(lis)

	return grpcServer
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
