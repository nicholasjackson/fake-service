package main

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/hashicorp/go-hclog"
	"github.com/nicholasjackson/env"
)

var isUpstream = env.Bool("UPSTREAM", false, false, "Is the service acting as an upstream service?")
var upstreamMessage = env.String("UPSTREAM_MESSAGE", false, "Hello from upstream service", "Message to be broadcast from upstream service")
var upstreamURI = env.String("UPSTREAM_URI", false, "localhost:9091", "URI of the upstream service")
var listenAddress = env.String("LISTEN_ADDR", false, ":9090", "IP address and port to bind service to")

var logger hclog.Logger

func main() {

	logger = hclog.Default()

	env.Parse()

	if *isUpstream {
		http.HandleFunc("/", upstreamHandler)
	} else {
		http.HandleFunc("/", downstreamHandler)
	}

	logger.Info("Starting service", "is_upstream", *isUpstream, "upstreamMessage", *upstreamMessage, "upstreamURI", *upstreamURI, "listenAddress", *listenAddress)
	logger.Error("Error starting service", "error", http.ListenAndServe(*listenAddress, nil))
}

// downstreamHandler handles requests when the service is acting like a downstream service
func downstreamHandler(rw http.ResponseWriter, r *http.Request) {
	logger.Info("Handling downstream request")

	// call the upstream service
	resp, err := http.Get(fmt.Sprintf("http://%s", *upstreamURI))
	if err != nil {
		logger.Error("Error communicating with upstream service", "error", err)
	}

	if resp.StatusCode != http.StatusOK {
		logger.Error("Expected status 200 from service got", "status", resp.StatusCode)
	}

	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Error reading response body", "error", err)
	}

	logger.Info("Received response from upstream", "response", string(data))

	rw.Write(data)
}

// upstreamHandler handles requests when the service is acting like a upstream service
func upstreamHandler(rw http.ResponseWriter, r *http.Request) {
	logger.Info("Handling upstream request")

	fmt.Fprint(rw, *upstreamMessage)
}
