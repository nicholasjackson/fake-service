package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"runtime"
	"strings"
	"syscall"
	"time"

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
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"

	"github.com/soheilhy/cmux"
	//"net/http/pprof"
)

var upstreamURIs = env.String("UPSTREAM_URIS", false, "", "Comma separated URIs of the upstream services to call")
var upstreamAllowInsecure = env.Bool("UPSTREAM_ALLOW_INSECURE", false, false, "Allow calls to upstream servers, ignoring TLS certificate validation")
var upstreamWorkers = env.Int("UPSTREAM_WORKERS", false, 1, "Number of parallel workers for calling upstreams, defualt is 1 which is sequential operation")

var upstreamRequestBody = env.String("UPSTREAM_REQUEST_BODY", false, "", "Request body to send to send with upstream requests, NOTE: UPSTREAM_REQUEST_SIZE and UPSTREAM_REQUEST_VARIANCE are ignored if this is set")
var upstreamRequestSize = env.Int("UPSTREAM_REQUEST_SIZE", false, 0, "Size of the randomly generated request body to send with upstream requests")
var upstreamRequestVariance = env.Int("UPSTREAM_REQUEST_VARIANCE", false, 0, "Percentage variance of the randomly generated request body")

var message = env.String("MESSAGE", false, "Hello World", "Message to be returned from service")
var name = env.String("NAME", false, "Service", "Name of the service")

var listenAddress = env.String("LISTEN_ADDR", false, "0.0.0.0:9090", "IP address and port to bind service to")

var allowedOrigins = env.String("ALLOWED_ORIGINS", false, "*", "Comma separated list of allowed origins for CORS requests")
var allowedHeaders = env.String("ALLOWED_HEADERS", false, "Accept,Accept-Language,Content-Language,Origin,Content-Type", "Comma separated list of allowed headers for cors requests")
var allowCredentials = env.Bool("ALLOW_CREDENTIALS", false, false, "Are credentials allowed for CORS requests")

// Server configuration
var serverKeepAlives = env.Bool("HTTP_SERVER_KEEP_ALIVES", false, false, "Enables the HTTP servers handling of keep alives.")
var serverReadTimeout = env.Duration("HTTP_SERVER_READ_TIMEOUT", false, time.Duration(5*time.Second), "Maximum duration for reading an entire HTTP request, if zero no read timeout is used.")
var serverReadHeaderTimeout = env.Duration("HTTP_SERVER_READHEADER_TIMEOUT", false, time.Duration(0*time.Second), "Maximum duration for reading the HTTP headers, if zero read timeout is used.")
var serverWriteTimeout = env.Duration("HTTP_SERVER_WRITE_TIMEOUT", false, time.Duration(10*time.Second), "Maximum duration for writing HTTP body, if zero no write timeout is used.")
var serverIdleTimeout = env.Duration("HTTP_SERVER_IDLE_TIMEOUT", false, time.Duration(30*time.Second), "Maximum duration to wait for next request when HTTP Keep alives are used.")

// Upstream client configuration
var upstreamClientKeepAlives = env.Bool("HTTP_CLIENT_KEEP_ALIVES", false, false, "Enable HTTP connection keep alives for upstream calls.")
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
var loadCPUAllocated = env.Int("LOAD_CPU_ALLOCATED", false, 0, "MHz of CPU allocated to the service, when specified, load percentage is a percentage of CPU allocated")
var loadCPUClockSpeed = env.Int("LOAD_CPU_CLOCK_SPEED", false, 1000, "MHz of a Single logical core, default 1000Mhz")
var loadCPUCores = env.Int("LOAD_CPU_CORES", false, -1, "Number of cores to generate fake CPU load over, by default fake-service will use all cores")
var loadCPUPercentage = env.Float64("LOAD_CPU_PERCENTAGE", false, 0, "Percentage of CPU cores to consume as a percentage. I.e: 50, 50% load for LOAD_CPU_CORES. If LOAD_CPU_ALLOCATED is not specified CPU percentage is based on the Total CPU available")

var loadMemoryAllocated = env.Int("LOAD_MEMORY_PER_REQUEST", false, 0, "Memory in bytes consumed per request")
var loadMemoryVariance = env.Int("LOAD_MEMORY_VARIANCE", false, 0, "Percentage variance of the memory consumed per request, i.e with a value of 50 = 50%, and given a LOAD_MEMORY_PER_REQUEST of 1024 bytes, actual consumption per request would be in the range 516 - 1540 bytes")

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
var readyRootPathWaitTillReady = env.Bool("READY_CHECK_ROOT_PATH_WAIT_TILL_READY", false, false, "Should the main handler at path `/` wait for the readiness check to pass before returning a response?")
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
		*loadCPUCores = runtime.NumCPU()
	}

	if *loadCPUAllocated != 0 {
		*loadCPUPercentage = float64(*loadCPUAllocated) / (float64(*loadCPUClockSpeed) * float64(*loadCPUCores)) * float64(*loadCPUPercentage)
	}

	// create a generator that will be used to create memory and CPU load per request
	generator := load.NewGenerator(*loadCPUCores, *loadCPUPercentage, *loadMemoryAllocated, *loadMemoryVariance, logger.Log().Named("load_generator"))
	requestGenerator := load.NewRequestGenerator(*upstreamRequestBody, *upstreamRequestSize, *upstreamRequestVariance, int64(*seed))

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

	// setup the listener
	l, err := net.Listen("tcp", *listenAddress)
	if err != nil {
		logger.Log().Error("Unable to listen at", "address", *listenAddress, "error", err)
		os.Exit(1)
	}

	// if we are using TLS wrap the listener in a TLS listener
	if *tlsCertificate != "" && *tlsKey != "" {
		logger.Log().Info("Enabling TLS for HTTP endpoint")

		var certificate tls.Certificate
		certificate, err = tls.LoadX509KeyPair(*tlsCertificate, *tlsKey)
		if err != nil {
			logger.Log().Error("Error loading certificates", "error", err)
			os.Exit(1)
		}

		config := &tls.Config{
			Certificates: []tls.Certificate{certificate},
			Rand:         rand.Reader,
		}

		// Create TLS listener.
		l = tls.NewListener(l, config)
	}

	// create a cmux
	// cmux allows us to have a grpc and a http server listening on the same port
	m := cmux.New(l)
	httpListener := m.Match(cmux.HTTP1Fast())
	grpcListener := m.Match(cmux.Any())

	// create the http handlers
	hh := handlers.NewHealth(logger, *healthResponseCode)
	rh := handlers.NewReady(logger, *readySuccessResponseCode, *readyFailureResponseCode, *readyResponseDelay)
	rq := handlers.NewRequest(
		*name,
		*message,
		requestDuration,
		tidyURIs(*upstreamURIs),
		*upstreamWorkers,
		defaultClient,
		grpcClients,
		errorInjector,
		generator,
		logger,
		requestGenerator,
		*readyRootPathWaitTillReady,
		rh,
	)
	cq := handlers.NewConfig(logger, errorInjector, hh)

	grpcServer := createGRPCServer(logger, requestDuration, errorInjector, generator, grpcClients, defaultClient, requestGenerator, *readyRootPathWaitTillReady, rh)
	httpServer := createHTTPServer(hh, rh, rq, cq, logger)

	// start the http/s server
	go func() {
		var err error
		err = httpServer.Serve(httpListener)

		if err != nil && err != http.ErrServerClosed {
			logger.Log().Error("Error starting http server", "error", err)
			os.Exit(1)
		}
	}()

	// start the grpc server
	go func() {
		err := grpcServer.Serve(grpcListener)
		if err != nil {
			logger.Log().Error("Error starting gRPC server", "error", err)
			os.Exit(1)
		}
	}()

	// start the multiplexer listening
	go m.Serve()

	logger.ServiceStarted(*name, *upstreamURIs, *upstreamWorkers, *listenAddress)

	// trap sigterm or interupt and gracefully shutdown the server
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	// Block until a signal is received.
	sig := <-c
	log.Println("Graceful shutdown, got signal:", sig)

	// if the server does not gracefully stop after 30s, kill it
	timer := time.AfterFunc(30*time.Second, func() {
		grpcServer.Stop() // force stop the server
	})

	grpcServer.GracefulStop()
	timer.Stop()

	// gracefully shutdown the HTTP server, waiting max 30 seconds for current operations to complete
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	httpServer.Shutdown(ctx)

	m.Close()
}

//go:embed ui/build
var uiFiles embed.FS

// An fs that adds the ui/build prefix to all requested files
type embedFs struct {
}

func (e *embedFs) Open(name string) (fs.File, error) {
	return uiFiles.Open(path.Join("ui/build", name))
}

func createHTTPServer(
	hh *handlers.Health,
	rh *handlers.Ready,
	rq http.Handler,
	con *handlers.Config,
	logger *logging.Logger,
) *http.Server {
	mux := http.NewServeMux()

	// add the static files
	logger.Log().Info("Adding handler for UI static files")
	mux.Handle("/ui/", http.StripPrefix("/ui", http.FileServer(http.FS(&embedFs{}))))

	// Add the generic health and ready handlers
	mux.HandleFunc("/health", hh.Handle)
	mux.HandleFunc("/ready", rh.Handle)

	// Add the config handler that allows modification of config values dynamically
	mux.HandleFunc("/config/", con.Handle)

	// uncomment to enable pprof
	//mux.HandleFunc("/debug/pprof/", pprof.Index)
	//mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	//mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	//mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	//mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	mux.Handle("/", rq)

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

	server := &http.Server{
		Addr:              *listenAddress,
		ReadTimeout:       *serverReadTimeout,
		ReadHeaderTimeout: *serverReadHeaderTimeout,
		WriteTimeout:      *serverWriteTimeout,
		IdleTimeout:       *serverIdleTimeout,
		Handler:           ch(mux),
		ErrorLog:          logger.Log().StandardLogger(&hclog.StandardLoggerOptions{InferLevels: true}),
	}

	server.SetKeepAlivesEnabled(*serverKeepAlives)

	return server
}

func createGRPCServer(
	logger *logging.Logger,
	rd *timing.RequestDuration,
	errorInjector *errors.Injector,
	generator *load.Generator,
	grpcClients map[string]client.GRPC,
	defaultClient client.HTTP,
	requestGenerator load.RequestGenerator,
	waitForReadyCheck bool,
	readyHandler *handlers.Ready,
) *grpc.Server {

	serverOptions := []grpc.ServerOption{}

	// disable keep alives
	if !*upstreamClientKeepAlives {
		serverOptions = append(serverOptions, grpc.KeepaliveParams(keepalive.ServerParameters{MaxConnectionAge: 5 * time.Second}))
	}

	//if *tlsCertificate != "" && *tlsKey != "" {
	//	creds, err := credentials.NewServerTLSFromFile(*tlsCertificate, *tlsKey)
	//	if err != nil {
	//		logger.Log().Error("Unable to load TLS certificate for gRPC server", "error", err)
	//		os.Exit(1)
	//	}
	//	serverOptions = append(serverOptions, grpc.Creds(creds))
	//}

	grpcServer := grpc.NewServer()

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
		waitForReadyCheck, // hard code to false until we
		readyHandler,
	)

	api.RegisterFakeServiceServer(grpcServer, fakeServer)

	return grpcServer
}

// tidyURIs splits the upstream URIs passed by environment variable and returns
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
