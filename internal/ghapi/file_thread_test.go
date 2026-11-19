package ghapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddFileReviewThread_Success_SendsFileSubjectForReview(t *testing.T) {
	var got struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		_, _ = w.Write([]byte(`{"data":{"addPullRequestReviewThread":{"thread":{"id":"PRRT_1"}}}}`))
	}))
	defer srv.Close()
	origURL := githubGraphQLURL
	githubGraphQLURL = srv.URL
	defer func() { githubGraphQLURL = origURL }()

	require.NoError(t, AddFileReviewThread(context.Background(), srv.Client(), "PRR_1", "a.go", "split this"))
	assert.Contains(t, got.Query, "subjectType: FILE")
	assert.Equal(t, map[string]any{"reviewID": "PRR_1", "path": "a.go", "body": "split this"}, got.Variables)
}
