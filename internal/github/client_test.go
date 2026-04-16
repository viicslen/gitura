package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewClient_ReturnsNonNilClient verifies that NewClient returns a usable
// go-github client with a non-empty base URL.
func TestNewClient_ReturnsNonNilClient(t *testing.T) {
	client := NewClient("test-token")
	require.NotNil(t, client)
	assert.NotEmpty(t, client.BaseURL.String())
}

// TestNewHTTPClient_ReturnsNonNilClient verifies that NewHTTPClient returns a
// non-nil *http.Client.
func TestNewHTTPClient_ReturnsNonNilClient(t *testing.T) {
	client := NewHTTPClient("test-token")
	require.NotNil(t, client)
}
