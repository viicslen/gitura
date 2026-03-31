package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-github/v67/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestClientForPR creates a *github.Client that routes to the given httptest server.
func newTestClientForPR(srv *httptest.Server) *github.Client {
	client := github.NewClient(nil)
	parsed, _ := url.Parse(srv.URL + "/")
	client.BaseURL = parsed
	return client
}

// makePRJSON creates a minimal PR JSON response from the GitHub REST API.
func makePRJSON(number int, title, state, nodeID, htmlURL, headRef, baseRef, headSHA string, isDraft bool) map[string]interface{} {
	return map[string]interface{}{
		"id":       int64(number) * 100,
		"number":   number,
		"title":    title,
		"state":    state,
		"draft":    isDraft,
		"node_id":  nodeID,
		"html_url": htmlURL,
		"body":     "PR body",
		"head": map[string]interface{}{
			"ref": headRef,
			"sha": headSHA,
		},
		"base": map[string]interface{}{
			"ref": baseRef,
		},
	}
}

// TestFetchPR_Success_MapsAllFields verifies that a successful API response is
// correctly mapped to PullRequestSummary, including IsDraft.
func TestFetchPR_Success_MapsAllFields(t *testing.T) {
	prJSON := makePRJSON(42, "My Feature", "open", "MDExOlB1bGxSZXF1ZXN0", "https://github.com/owner/repo/pull/42",
		"feature-branch", "main", "abc123sha", false)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/owner/repo/pulls/42", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(prJSON)
		_, _ = fmt.Fprint(w, string(b))
	}))
	defer srv.Close()

	client := newTestClientForPR(srv)
	summary, err := FetchPR(context.Background(), client, "owner", "repo", 42)
	require.NoError(t, err)
	require.NotNil(t, summary)

	assert.Equal(t, 42, summary.Number)
	assert.Equal(t, "My Feature", summary.Title)
	assert.Equal(t, "open", summary.State)
	assert.False(t, summary.IsDraft)
	assert.Equal(t, "feature-branch", summary.HeadBranch)
	assert.Equal(t, "main", summary.BaseBranch)
	assert.Equal(t, "abc123sha", summary.HeadSHA)
	assert.Equal(t, "https://github.com/owner/repo/pull/42", summary.HTMLURL)
	assert.Equal(t, "owner", summary.Owner)
	assert.Equal(t, "repo", summary.Repo)
}

// TestFetchPR_Draft_SetsDraftTrue verifies IsDraft is mapped correctly.
func TestFetchPR_Draft_SetsDraftTrue(t *testing.T) {
	prJSON := makePRJSON(1, "WIP", "open", "N1", "https://github.com/o/r/pull/1",
		"wip-branch", "main", "sha1", true)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(prJSON)
		_, _ = fmt.Fprint(w, string(b))
	}))
	defer srv.Close()

	client := newTestClientForPR(srv)
	summary, err := FetchPR(context.Background(), client, "o", "r", 1)
	require.NoError(t, err)
	assert.True(t, summary.IsDraft)
}

// TestFetchPR_NotFound_ReturnsNotFoundError verifies HTTP 404 produces a
// "notfound:" prefixed error.
func TestFetchPR_NotFound_ReturnsNotFoundError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"message":"Not Found"}`)
	}))
	defer srv.Close()

	client := newTestClientForPR(srv)
	_, err := FetchPR(context.Background(), client, "owner", "repo", 99)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "notfound:")
}

// TestFetchPR_NetworkError_ReturnsGithubError verifies non-404 errors are
// prefixed with "github:".
func TestFetchPR_NetworkError_ReturnsGithubError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"message":"Internal Server Error"}`)
	}))
	defer srv.Close()

	client := newTestClientForPR(srv)
	_, err := FetchPR(context.Background(), client, "owner", "repo", 42)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "github:")
}
