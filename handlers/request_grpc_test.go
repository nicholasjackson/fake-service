package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/nicholasjackson/fake-service/client"
	"github.com/nicholasjackson/fake-service/timing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupFakeServer(t *testing.T, uris []string) (*FakeServer, *client.MockHTTP) {
	l := hclog.Default()
	c := &client.MockHTTP{}
	d := timing.NewRequestDuration(
		1*time.Nanosecond,
		1*time.Nanosecond,
		1*time.Nanosecond,
		0)

	return NewFakeServer("test", "hello world", d, uris, 1, c, l), c
}

func TestGRPCServiceHandlesRequestWithNoUpstream(t *testing.T) {
	fs, _ := setupFakeServer(t, nil)

	resp, err := fs.Handle(context.Background(), nil)

	assert.Nil(t, err)
	assert.Equal(t, "# Reponse from: test #\nhello world\n", resp.Message)
}

func TestGRPCServiceHandlesRequestWithHTTPUpstream(t *testing.T) {
	uris := []string{"http://test.com"}
	fs, mc := setupFakeServer(t, uris)
	mc.On("Do", mock.Anything).Return([]byte("# Response from: upstream #\nOK\n"), nil)

	resp, err := fs.Handle(context.Background(), nil)

	assert.Nil(t, err)
	mc.AssertCalled(t, "Do", mock.Anything)
	assert.Equal(t, "# Reponse from: test #\nhello world\n## Called upstream uri: http://test.com\n  # Response from: upstream #\n  OK\n  ", resp.Message)
}
