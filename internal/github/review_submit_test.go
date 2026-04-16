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

	"gitura/internal/model"
)

func newTestClientForReview(srv *httptest.Server) *github.Client {
	client := github.NewClient(nil)
	parsed, _ := url.Parse(srv.URL + "/")
	client.BaseURL = parsed
	return client
}

// TestCreatePendingReview_Success verifies that a 201 response from the
// reviews endpoint returns the review ID.
func TestCreatePendingReview_Success_ReturnsReviewID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"id": 42, "html_url": "https://github.com/owner/repo/pull/1#pullrequestreview-42"}`)
	}))
	defer srv.Close()

	id, err := CreatePendingReview(context.Background(), newTestClientForReview(srv), "owner", "repo", 1)
	require.NoError(t, err)
	assert.Equal(t, int64(42), id)
}

// TestCreatePendingReview_APIError verifies that an API error is wrapped
// with a "github:" prefix.
func TestCreatePendingReview_APIError_ReturnsGithubPrefix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, `{"message":"Validation Failed"}`)
	}))
	defer srv.Close()

	_, err := CreatePendingReview(context.Background(), newTestClientForReview(srv), "owner", "repo", 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "github:")
}

// TestAddCommentToPendingReview_Success verifies a 201 response is treated as
// success and the request body contains expected fields.
func TestAddCommentToPendingReview_Success_SendsCorrectPayload(t *testing.T) {
	var captured addCommentRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&captured))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"id": 99}`)
	}))
	defer srv.Close()

	orig := githubRESTBaseURL
	githubRESTBaseURL = srv.URL
	defer func() { githubRESTBaseURL = orig }()

	comment := model.DraftCommentDTO{
		Path: "main.go",
		Body: "nit: rename this",
		Line: 10,
		Side: "RIGHT",
	}
	err := AddCommentToPendingReview(context.Background(), srv.Client(), "owner", "repo", 1, 42, comment)
	require.NoError(t, err)
	assert.Equal(t, "main.go", captured.Path)
	assert.Equal(t, "nit: rename this", captured.Body)
	assert.Equal(t, 10, captured.Line)
	assert.Equal(t, "RIGHT", captured.Side)
	assert.Equal(t, 0, captured.StartLine) // single-line: omitted
}

// TestAddCommentToPendingReview_MultiLine verifies that StartLine and StartSide
// are included when a multi-line comment is requested.
func TestAddCommentToPendingReview_MultiLine_SendsStartFields(t *testing.T) {
	var captured addCommentRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&captured))
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"id": 100}`)
	}))
	defer srv.Close()

	orig := githubRESTBaseURL
	githubRESTBaseURL = srv.URL
	defer func() { githubRESTBaseURL = orig }()

	comment := model.DraftCommentDTO{
		Path:      "api.go",
		Body:      "consider splitting this block",
		Line:      20,
		Side:      "RIGHT",
		StartLine: 15,
		StartSide: "RIGHT",
	}
	err := AddCommentToPendingReview(context.Background(), srv.Client(), "owner", "repo", 1, 42, comment)
	require.NoError(t, err)
	assert.Equal(t, 15, captured.StartLine)
	assert.Equal(t, "RIGHT", captured.StartSide)
}

// TestAddCommentToPendingReview_HTTPError verifies that a non-2xx response is
// returned as a "github:" error.
func TestAddCommentToPendingReview_HTTPError_ReturnsGithubPrefix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, `{"message":"Validation Failed"}`)
	}))
	defer srv.Close()

	orig := githubRESTBaseURL
	githubRESTBaseURL = srv.URL
	defer func() { githubRESTBaseURL = orig }()

	err := AddCommentToPendingReview(
		context.Background(), srv.Client(),
		"owner", "repo", 1, 42,
		model.DraftCommentDTO{Path: "f.go", Body: "x", Line: 1, Side: "RIGHT"},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "github:")
}

// TestSubmitReview_Success verifies that the review ID and HTML URL are returned
// after a successful submission.
func TestSubmitReview_Success_ReturnsIDAndURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id": 42, "html_url": "https://github.com/owner/repo/pull/1#pullrequestreview-42"}`)
	}))
	defer srv.Close()

	req := model.ReviewSubmitDTO{Verdict: "APPROVE"}
	result, err := SubmitReview(context.Background(), newTestClientForReview(srv), "owner", "repo", 1, 42, req)
	require.NoError(t, err)
	assert.Equal(t, int64(42), result.ReviewID)
	assert.Contains(t, result.HTMLURL, "pullrequestreview-42")
}

// TestSubmitReview_APIError verifies that an API error is wrapped with "github:".
func TestSubmitReview_APIError_ReturnsGithubPrefix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, `{"message":"Validation Failed"}`)
	}))
	defer srv.Close()

	_, err := SubmitReview(context.Background(), newTestClientForReview(srv), "owner", "repo", 1, 42,
		model.ReviewSubmitDTO{Verdict: "REQUEST_CHANGES", Body: "needs work"},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "github:")
}

// TestDeletePendingReview_Success verifies that a successful DELETE returns no error.
func TestDeletePendingReview_Success_NoError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id": 42}`)
	}))
	defer srv.Close()

	err := DeletePendingReview(context.Background(), newTestClientForReview(srv), "owner", "repo", 1, 42)
	require.NoError(t, err)
}

// TestDeletePendingReview_APIError verifies that an API error is wrapped with "github:".
func TestDeletePendingReview_APIError_ReturnsGithubPrefix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"message":"Not Found"}`)
	}))
	defer srv.Close()

	err := DeletePendingReview(context.Background(), newTestClientForReview(srv), "owner", "repo", 1, 99)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "github:")
}
