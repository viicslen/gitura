package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	gh "github.com/google/go-github/v67/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitura/internal/model"
)

func newOverviewTestClient(srv *httptest.Server) *gh.Client {
	client := gh.NewClient(nil)
	parsed, _ := url.Parse(srv.URL + "/")
	client.BaseURL = parsed
	return client
}

func TestListPRComments_Success_ReturnsMappedComments(t *testing.T) {
	createdAt := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(2 * time.Hour)
	body := []map[string]any{{
		"id":         101,
		"body":       "Top-level PR comment",
		"html_url":   "https://github.com/o/r/pull/1#issuecomment-101",
		"created_at": createdAt.Format(time.RFC3339),
		"updated_at": updatedAt.Format(time.RFC3339),
		"user": map[string]any{
			"login":      "alice",
			"avatar_url": "https://avatars.githubusercontent.com/u/1?v=4",
		},
	}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/o/r/issues/1/comments", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer srv.Close()

	comments, err := ListPRComments(context.Background(), newOverviewTestClient(srv), "o", "r", 1)
	require.NoError(t, err)
	require.Len(t, comments, 1)
	assert.Equal(t, int64(101), comments[0].ID)
	assert.Equal(t, "alice", comments[0].AuthorLogin)
	assert.Equal(t, "Top-level PR comment", comments[0].Body)
	assert.Equal(t, createdAt.Format(time.RFC3339), comments[0].CreatedAt)
	assert.Equal(t, updatedAt.Format(time.RFC3339), comments[0].UpdatedAt)
}

func TestListPRReviews_Success_ExcludesPendingAndMapsFields(t *testing.T) {
	submittedAt := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	body := []map[string]any{
		{
			"id":           201,
			"state":        "COMMENTED",
			"body":         "Looks good overall",
			"html_url":     "https://github.com/o/r/pull/1#pullrequestreview-201",
			"submitted_at": submittedAt.Format(time.RFC3339),
			"user": map[string]any{
				"login":      "reviewer",
				"avatar_url": "https://avatars.githubusercontent.com/u/2?v=4",
			},
		},
		{
			"id":       202,
			"state":    "PENDING",
			"body":     "draft",
			"html_url": "https://github.com/o/r/pull/1#pullrequestreview-202",
			"user":     map[string]any{"login": "draft-user"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/o/r/pulls/1/reviews", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, mustJSON(t, body))
	}))
	defer srv.Close()

	reviews, err := ListPRReviews(context.Background(), newOverviewTestClient(srv), "o", "r", 1)
	require.NoError(t, err)
	require.Len(t, reviews, 1)
	assert.Equal(t, int64(201), reviews[0].ReviewID)
	assert.Equal(t, "COMMENTED", reviews[0].State)
	assert.Equal(t, "Looks good overall", reviews[0].Body)
	assert.Equal(t, submittedAt.Format(time.RFC3339), reviews[0].SubmittedAt)
}

func TestDecorateReviewsWithThreadStatus_ComputesResolutionState(t *testing.T) {
	reviews := []model.ReviewOverviewDTO{
		{ReviewID: 10},
		{ReviewID: 11},
		{ReviewID: 12},
	}
	threads := []model.CommentThreadDTO{
		{ReviewID: 10, Resolved: true},
		{ReviewID: 10, Resolved: true},
		{ReviewID: 11, Resolved: false},
	}

	decorated := DecorateReviewsWithThreadStatus(reviews, threads)
	require.Len(t, decorated, 3)
	assert.True(t, decorated[0].HasThreads)
	assert.True(t, decorated[0].AllThreadsResolved)
	assert.Equal(t, 2, decorated[0].ResolvedThreadCount)
	assert.Equal(t, 2, decorated[0].TotalThreadCount)
	assert.True(t, decorated[1].HasThreads)
	assert.False(t, decorated[1].AllThreadsResolved)
	assert.Equal(t, 0, decorated[1].ResolvedThreadCount)
	assert.Equal(t, 1, decorated[1].TotalThreadCount)
	assert.False(t, decorated[2].HasThreads)
	assert.False(t, decorated[2].AllThreadsResolved)
	assert.Equal(t, 0, decorated[2].TotalThreadCount)
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return string(data)
}
