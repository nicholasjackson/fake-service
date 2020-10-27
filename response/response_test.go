package response

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertsFromJSON(t *testing.T) {
	d := `
   {
	   "name": "Test App",
		 "uri": "http://something.com",
		 "upstream_calls": {"abc": {"name": "upstream"}}
	 }
	`

	r := &Response{}
	r.FromJSON([]byte(d))

	assert.Equal(t, "Test App", r.Name)
	assert.Equal(t, "http://something.com", r.URI)
	assert.Len(t, r.UpstreamCalls, 1)
	assert.Equal(t, "upstream", r.UpstreamCalls["abc"].Name)
}
