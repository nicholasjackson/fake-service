package response

import (
	"bytes"
	"encoding/json"
)

// Response defines the type which is returned from the service
type Response struct {
	Name          string            `json:"name,omitempty"`
	URI           string            `json:"uri,omitempty"`
	Type          string            `json:"type,omitempty"`
	IPAddresses   []string          `json:"ip_addresses,omitempty"`
	StartTime     string            `json:"start_time,omitempty"`
	EndTime       string            `json:"end_time,omitempty"`
	Duration      string            `json:"duration,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	Cookies       map[string]string `json:"cookies,omitempty"`
	Body          json.RawMessage   `json:"body,omitempty"`
	UpstreamCalls []Response        `json:"upstream_calls,omitempty"`
	Code          int               `json:"code"`
	Error         string            `json:"error,omitempty"`
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
func (r *Response) AppendUpstreams(reps []*Response) {
	for _, u := range reps {
		r.AppendUpstream(u)
	}
}

// AppendUpstream appends an upstream response to this object
func (r *Response) AppendUpstream(resp *Response) {
	if resp == nil {
		return
	}

	if r.UpstreamCalls == nil {
		r.UpstreamCalls = make([]Response, 0)
	}

	r.UpstreamCalls = append(r.UpstreamCalls, *resp)
}
