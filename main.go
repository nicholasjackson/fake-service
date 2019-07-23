package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/nicholasjackson/env"
)

var upstreamCall = env.Bool("UPSTREAM_CALL", false, false, "Should we call the upstream service?")
var upstreamURI = env.String("UPSTREAM_URI", false, "localhost:9091", "URI of the upstream service")

var message = env.String("MESSAGE", false, "Hello World", "Message to be returned from service")

var listenAddress = env.String("LISTEN_ADDR", false, ":9090", "IP address and port to bind service to")

var upstreamClientKeepAlives = env.Bool("HTTP_CLIENT_KEEP_ALIVES", false, true, "Enable HTTP connection keep alives for upstream calls")

var logger hclog.Logger

var defaultClient *http.Client

func main() {

	logger = hclog.Default()

	env.Parse()

	// create the httpClient
	defaultClient = createClient()

	http.HandleFunc("/", requestHandler)
	http.HandleFunc("/health", healthHandler)

	logger.Info(
		"Starting service",
		"upstreamCall", *upstreamCall,
		"message", *message,
		"upstreamURI", *upstreamURI,
		"listenAddress", *listenAddress)

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

func requestHandler(rw http.ResponseWriter, r *http.Request) {
	logger.Info("Handling request", "request", formatRequest(r))

	var data []byte

	if *upstreamCall {
		// call the upstream service
		resp, err := http.Get(fmt.Sprintf("http://%s", *upstreamURI))
		if err != nil {
			logger.Error("Error communicating with upstream service", "error", err)
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		if resp.StatusCode != http.StatusOK {
			logger.Error("Expected status 200 from service got", "status", resp.StatusCode)
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		defer resp.Body.Close()

		data, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			logger.Error("Error reading response body", "error", err)
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		logger.Info("Received response from upstream", "response", string(data))
	}

	if *upstreamCall {
		// pad the response with two spaces
		respLines := []string{}
		for _, s := range strings.Split(string(data), "\n") {
			respLines = append(respLines, fmt.Sprintf("  %s", s))
		}

		resp := strings.Join(respLines, "\n")

		fmt.Fprintf(rw, "%s\n###Upstream Data: %s###\n%s", *message, *upstreamURI, resp)
		return
	}

	rw.Write([]byte(*message))

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
