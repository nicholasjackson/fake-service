package client

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

// HTTP defines an interface for upstream HTTP client requests
type HTTP interface {
	Do(r *http.Request, pr *http.Request) (int, []byte, map[string]string, map[string]string, error)
}

// HTTPImpl is the concrete implementation of the HTTP interface
type HTTPImpl struct {
	defaultClient *http.Client
	appendRequest bool // should we append the headers path and query from the original request
}

// NewHTTP creates a new HTTP client
func NewHTTP(upstreamClientKeepAlives bool, appendRequest bool, timeOut time.Duration, allowInsecure bool) HTTP {
	client := &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: !upstreamClientKeepAlives,
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: allowInsecure},
		},
		Timeout: timeOut,
	}

	return &HTTPImpl{
		defaultClient: client,
		appendRequest: appendRequest,
	}
}

// Do makes the upstream request and returns a response
func (h *HTTPImpl) Do(r *http.Request, pr *http.Request) (int, []byte, map[string]string, map[string]string, error) {
	var data []byte

	// do we need to append the headers, path and querystring from the original request?
	if pr != nil && h.appendRequest == true {
		appendHeaders(r, pr)
		appendPath(r, pr)
	}

	// call the upstream service
	resp, err := h.defaultClient.Do(r)
	if err != nil {
		return -1, nil, nil, nil, fmt.Errorf("Error communicating with upstream service: %s", err)
	}

	defer resp.Body.Close()

	data, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, nil, nil, fmt.Errorf("Error reading response body: %d", err)
	}

	var statusError error
	if resp.StatusCode != http.StatusOK {
		// if a request err
		statusError = fmt.Errorf("Error processing upstream request: %s, expected code 200, got %d", r.URL.String(), resp.StatusCode)
	}

	headers := map[string]string{}
	for k, v := range resp.Header {
		headers[k] = strings.Join(v, ",")
	}

	cookies := map[string]string{}
	for _, c := range resp.Cookies() {
		cookies[c.Name] = c.Value
	}

	return resp.StatusCode, data, headers, cookies, statusError
}

// appendHeaders from the original request
func appendHeaders(r, pr *http.Request) {
	for k, v := range pr.Header {
		if r.Header.Get(k) == "" {
			for _, vv := range v {
				r.Header.Set(k, vv)
			}
		}
	}
}

// appendPath from the original request to this request
func appendPath(r, pr *http.Request) {
	op := pr.URL.Path
	r.URL.Path = r.URL.Path + op
}
