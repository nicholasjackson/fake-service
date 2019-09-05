package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/nicholasjackson/fake-service/client"
	"github.com/nicholasjackson/fake-service/grpc/api"
	"github.com/nicholasjackson/fake-service/response"
	"github.com/nicholasjackson/fake-service/worker"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/log"
	"google.golang.org/grpc/metadata"
)

func workerHTTP(ctx opentracing.SpanContext, uri string, defaultClient client.HTTP, pr *http.Request) (*response.Response, error) {
	httpReq, _ := http.NewRequest("GET", uri, nil)

	clientSpan := opentracing.StartSpan(
		"call_upstream",
		opentracing.ChildOf(ctx),
	)

	ext.SpanKindRPCClient.Set(clientSpan)
	ext.HTTPUrl.Set(clientSpan, uri)
	ext.HTTPMethod.Set(clientSpan, "GET")
	clientSpan.LogFields(log.String("upstream.type", "http"))

	// Transmit the span's TraceContext as HTTP headers on our
	// outbound request.
	opentracing.GlobalTracer().Inject(
		clientSpan.Context(),
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(httpReq.Header))

	resp, err := defaultClient.Do(httpReq, pr)
	clientSpan.Finish()

	if err != nil {
		return nil, err
	}

	r := &response.Response{}
	err = r.FromJSON(resp)
	if err != nil {
		return nil, err
	}

	// set the local URI for the upstream
	r.URI = uri

	return r, nil
}

func workerGRPC(ctx opentracing.SpanContext, uri string, grpcClients map[string]client.GRPC) (*response.Response, error) {
	c := grpcClients[uri]

	clientSpan := opentracing.StartSpan(
		"call_upstream",
		opentracing.ChildOf(ctx),
	)
	ext.SpanKindRPCClient.Set(clientSpan)

	// add the upstream type
	clientSpan.LogFields(log.String("upstream.type", "grpc"))

	req := &http.Request{Header: http.Header{}}
	opentracing.GlobalTracer().Inject(
		clientSpan.Context(),
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(req.Header))

	// create the grpc metadata and inject the client span
	md := httpRequestToGrpcMetadata(req)

	outCtx := metadata.NewOutgoingContext(context.Background(), md)

	resp, err := c.Handle(outCtx, &api.Nil{})
	clientSpan.Finish()

	if err != nil {
		return nil, err
	}

	r := &response.Response{}
	err = r.FromJSON([]byte(resp.Message))
	if err != nil {
		return nil, err
	}

	// set the local URI for the upstream
	r.URI = uri

	return r, nil
}

func processResponses(responses []worker.Done) []byte {
	respLines := []string{}

	// append the output from the upstreams
	for _, r := range responses {
		respLines = append(respLines, fmt.Sprintf("## Called upstream uri: %s", r.URI))
		/*
			// indent the reposne from the upstream
			lines := strings.Split(r.Message, "\n")
			for _, l := range lines {
				respLines = append(respLines, fmt.Sprintf("  %s", l))
			}
		*/
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

// the following two functions are a hack to get round that
// opentracing zipkin can not deal with grpc metadata for
// Inject and extract
func grpcMetaDataToHTTPRequest(md metadata.MD) *http.Request {
	h := http.Header{}
	for k, v := range md {
		for _, vv := range v {
			h.Add(k, vv)
		}
	}
	return &http.Request{Header: h}
}

func httpRequestToGrpcMetadata(r *http.Request) metadata.MD {
	md := metadata.MD{}

	for k, v := range r.Header {
		md.Set(k, v...)
	}

	return md
}
