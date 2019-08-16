package client

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

// HTTP defines an interface for upstream HTTP client requests
type HTTP interface {
	Do(r *http.Request) ([]byte, error)
}

// HTTPImpl is the concrete implementation of the HTTP interface
type HTTPImpl struct {
	defaultClient *http.Client
}

// NewHTTP creates a new HTTP client
func NewHTTP(upstreamClientKeepAlives bool) HTTP {
	client := &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: !upstreamClientKeepAlives,
		},
	}

	return &HTTPImpl{
		defaultClient: client,
	}
}

// Do makes the upstream request and returns a response
func (h *HTTPImpl) Do(r *http.Request) ([]byte, error) {
	var data []byte

	// call the upstream service
	resp, err := h.defaultClient.Do(r)
	if err != nil {
		return nil, fmt.Errorf("Error communicating with upstream service: %s", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Expected status 200 from service got %d", resp.StatusCode)
	}

	defer resp.Body.Close()

	data, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading response body: %d", err)
	}

	return data, nil
}
