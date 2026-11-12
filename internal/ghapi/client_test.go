package ghapi

import (
	"net/http/httptest"
	"testing"

	"github.com/google/go-github/v93/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestClient returns a go-github client whose REST calls go to srv.
func newTestClient(t *testing.T, srv *httptest.Server) *github.Client {
	t.Helper()
	base := srv.URL + "/"
	client, err := github.NewClient(github.WithURLs(&base, nil))
	require.NoError(t, err)
	return client
}

// TestNewClient_Token_ReturnsNonNilClient verifies that NewClient returns a usable
// go-github client with a non-empty base URL.
func TestNewClient_Token_ReturnsNonNilClient(t *testing.T) {
	client, err := NewClient("test-token")
	require.NoError(t, err)
	assert.NotEmpty(t, client.BaseURL())
}

// TestNewHTTPClient_Token_ReturnsNonNilClient verifies that NewHTTPClient returns a
// non-nil *http.Client.
func TestNewHTTPClient_Token_ReturnsNonNilClient(t *testing.T) {
	client := NewHTTPClient("test-token")
	require.NotNil(t, client)
}
