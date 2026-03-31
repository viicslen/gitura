package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResolveThread_Success_ReturnsNil verifies that a successful resolve
// mutation returns no error.
func TestResolveThread_Success_ReturnsNil(t *testing.T) {
	body := loadFixture(t, "resolve_thread_success.json")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	origURL := resolveGraphQLURL
	resolveGraphQLURL = srv.URL
	defer func() { resolveGraphQLURL = origURL }()

	err := ResolveThread(context.Background(), srv.Client(), "PRRT_kwDOTest001")
	require.NoError(t, err)
}

// TestUnresolveThread_Success_ReturnsNil verifies that a successful unresolve
// mutation returns no error.
func TestUnresolveThread_Success_ReturnsNil(t *testing.T) {
	body := loadFixture(t, "unresolve_thread_success.json")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	origURL := resolveGraphQLURL
	resolveGraphQLURL = srv.URL
	defer func() { resolveGraphQLURL = origURL }()

	err := UnresolveThread(context.Background(), srv.Client(), "PRRT_kwDOTest001")
	require.NoError(t, err)
}

// TestResolveThread_GraphQLError_ReturnsGithubPrefixedError verifies that a
// GraphQL errors array surfaces as a "github:" prefixed error.
func TestResolveThread_GraphQLError_ReturnsGithubPrefixedError(t *testing.T) {
	body := loadFixture(t, "resolve_thread_graphql_error.json")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	origURL := resolveGraphQLURL
	resolveGraphQLURL = srv.URL
	defer func() { resolveGraphQLURL = origURL }()

	err := ResolveThread(context.Background(), srv.Client(), "PRRT_invalid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "github:")
}

// TestResolveThread_HTTP422_ReturnsGithubPrefixedError verifies that a non-200
// HTTP status returns a "github:" prefixed error.
func TestResolveThread_HTTP422_ReturnsGithubPrefixedError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	defer srv.Close()

	origURL := resolveGraphQLURL
	resolveGraphQLURL = srv.URL
	defer func() { resolveGraphQLURL = origURL }()

	err := ResolveThread(context.Background(), srv.Client(), "PRRT_kwDOTest001")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "github:")
}

// TestResolveThread_HTTP401_ReturnsAuthPrefixedError verifies HTTP 401 returns
// an "auth:" prefixed error.
func TestResolveThread_HTTP401_ReturnsAuthPrefixedError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	origURL := resolveGraphQLURL
	resolveGraphQLURL = srv.URL
	defer func() { resolveGraphQLURL = origURL }()

	err := ResolveThread(context.Background(), srv.Client(), "PRRT_kwDOTest001")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth:")
}
