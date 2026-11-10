package ghapi

import (
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-github/v67/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestClient returns a go-github client whose REST calls go to srv.
func newTestClient(srv *httptest.Server) *github.Client {
	client := github.NewClient(nil)
	parsed, _ := url.Parse(srv.URL + "/")
	client.BaseURL = parsed
	return client
}

// TestNewClient_Token_ReturnsNonNilClient verifies that NewClient returns a usable
// go-github client with a non-empty base URL.
func TestNewClient_Token_ReturnsNonNilClient(t *testing.T) {
	client := NewClient("test-token")
	require.NotNil(t, client)
	assert.NotEmpty(t, client.BaseURL.String())
}

// TestNewHTTPClient_Token_ReturnsNonNilClient verifies that NewHTTPClient returns a
// non-nil *http.Client.
func TestNewHTTPClient_Token_ReturnsNonNilClient(t *testing.T) {
	client := NewHTTPClient("test-token")
	require.NotNil(t, client)
}
