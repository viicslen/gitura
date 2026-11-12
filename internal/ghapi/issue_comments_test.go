package ghapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchIssueComments_Success_ReturnsMappedComments(t *testing.T) {
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

	comments, err := FetchIssueComments(context.Background(), newTestClient(t, srv), "o", "r", 1)
	require.NoError(t, err)
	require.Len(t, comments, 1)
	assert.Equal(t, int64(101), comments[0].ID)
	assert.Equal(t, "alice", comments[0].AuthorLogin)
	assert.Equal(t, "Top-level PR comment", comments[0].Body)
	assert.Equal(t, createdAt.Format(time.RFC3339), comments[0].CreatedAt)
	assert.Equal(t, updatedAt.Format(time.RFC3339), comments[0].UpdatedAt)
}
