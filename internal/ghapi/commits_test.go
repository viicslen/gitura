package ghapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchCommits_Success_MapsHeadlineAuthorAndCommitterDate(t *testing.T) {
	body := []map[string]any{
		{
			"sha":      "abc123",
			"html_url": "https://github.com/o/r/commit/abc123",
			"author":   map[string]any{"login": "dev", "avatar_url": "https://avatars.githubusercontent.com/u/1"},
			"commit": map[string]any{
				"message":   "fix: thing\n\nlonger body",
				"author":    map[string]any{"name": "Dev", "date": "2026-01-01T10:00:00Z"},
				"committer": map[string]any{"name": "Dev", "date": "2026-01-02T10:00:00Z"},
			},
		},
		{
			"sha":    "def456",
			"commit": map[string]any{"message": "chore", "author": map[string]any{"name": "No Account"}, "committer": map[string]any{"date": "2026-01-03T10:00:00Z"}},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/o/r/pulls/1/commits", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, mustJSON(t, body))
	}))
	defer srv.Close()

	commits, err := FetchCommits(context.Background(), newTestClient(t, srv), "o", "r", 1)
	require.NoError(t, err)
	require.Len(t, commits, 2)
	assert.Equal(t, "fix: thing", commits[0].Headline)
	assert.Equal(t, "dev", commits[0].AuthorLogin)
	assert.Equal(t, "2026-01-02T10:00:00Z", commits[0].CommittedAt)
	assert.Equal(t, "No Account", commits[1].AuthorLogin)
}
