package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitisesURIParameters(t *testing.T) {
	in := "http://abc.com, https://123.com,"

	out := tidyURIs(in)

	assert.Equal(t, 2, len(out))
	assert.Equal(t, "http://abc.com", out[0])
	assert.Equal(t, "https://123.com", out[1])
}
