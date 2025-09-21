package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHttpClient(t *testing.T) {
	httpClient := NewDefaultHttpClient("test", true)
	get, err := httpClient.Get("https://graphigo.prd.galaxy.eco/")
	assert.NoError(t, err)
	assert.NotNil(t, get)
}
