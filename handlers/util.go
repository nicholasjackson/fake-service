package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/nicholasjackson/fake-service/client"
	"github.com/nicholasjackson/fake-service/worker"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

func workerHTTP(ctx opentracing.SpanContext, uri string, defaultClient client.HTTP) (string, error) {
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

	resp, err := defaultClient.Do(httpReq)
	clientSpan.Finish()

	if err != nil {
		return "", err
	}

	return string(resp), nil
}

func processResponses(responses []worker.Done) []byte {
	respLines := []string{}

	// append the output from the upstreams
	for _, r := range responses {
		respLines = append(respLines, fmt.Sprintf("## Called upstream uri: %s", r.URI))
		// indent the reposne from the upstream
		lines := strings.Split(r.Message, "\n")
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
