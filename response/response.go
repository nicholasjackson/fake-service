package response

import (
	"bytes"
	"encoding/json"
)

// Response defines the type which is returned from the service
type Response struct {
	Name          string              `json:"name,omitempty"`
	URI           string              `json:"uri,omitempty"` // Called URI by downstream
	Type          string              `json:"type,omitempty"`
	IPAddresses   []string            `json:"ip_addresses,omitempty"`
	Path          []string            `json:"path,omitempty"` // Path received by upstream
	StartTime     string              `json:"start_time,omitempty"`
	EndTime       string              `json:"end_time,omitempty"`
	Duration      string              `json:"duration,omitempty"`
	Headers       map[string]string   `json:"headers,omitempty"`
	Cookies       map[string]string   `json:"cookies,omitempty"`
	Body          json.RawMessage     `json:"body,omitempty"`
	UpstreamCalls map[string]Response `json:"upstream_calls,omitempty"`
	Code          int                 `json:"code"`
	Error         string              `json:"error,omitempty"`
}

// ToJSON converts the response to a JSON string
func (r *Response) ToJSON() string {
	buffer := new(bytes.Buffer)
	encoder := json.NewEncoder(buffer)
	encoder.SetIndent("", "  ")

	err := encoder.Encode(r)
	if err != nil {
		panic(err)
	}

	return buffer.String()
}

// FromJSON populates the response from a JSON string
func (r *Response) FromJSON(d []byte) error {
	resp := &Response{}
	err := json.Unmarshal(d, resp)
	if err != nil {
		return err
	}

	*r = *resp

	return nil
}

// AppendUpstreams appends multiple upstream responses to this object
func (r *Response) AppendUpstreams(reps map[string]Response) {
	for k, u := range reps {
		r.AppendUpstream(k, u)
	}
}

// AppendUpstream appends an upstream response to this object
func (r *Response) AppendUpstream(key string, resp Response) {
	if r.UpstreamCalls == nil {
		r.UpstreamCalls = make(map[string]Response)
	}

	r.UpstreamCalls[key] = resp
}
