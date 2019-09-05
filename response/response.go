package response

import (
	"bytes"
	"encoding/json"
)

type Response struct {
	Name          string     `json:"name,omitempty"`
	URI           string     `json:"uri,omitempty"`
	Type          string     `json:"type,omitempty"`
	Duration      string     `json:"duration,omitempty"`
	Body          string     `json:"body,omitempty"`
	UpstreamCalls []Response `json:"upstream_calls,omitempty"`
}

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

func (r *Response) FromJSON(d []byte) error {
	resp := &Response{}
	err := json.Unmarshal(d, resp)
	if err != nil {
		return err
	}

	*r = *resp

	return nil
}

func (r *Response) AppendUpstreams(reps []*Response) {
	for _, u := range reps {
		r.AppendUpstream(u)
	}
}

func (r *Response) AppendUpstream(resp *Response) {
	if r.UpstreamCalls == nil {
		r.UpstreamCalls = make([]Response, 0)
	}

	r.UpstreamCalls = append(r.UpstreamCalls, *resp)
}
