package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-github/v67/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitura/internal/model"
)

// A thread reload that lands while a reply is in flight must not redirect or break the cache write.
func TestReplyToComment_SurvivesThreadReloadDuringRequest(t *testing.T) {
	app := &App{
		ctx:      context.Background(),
		prOwner:  "o",
		prRepo:   "r",
		prNumber: 1,
		threads: []model.CommentThreadDTO{
			{RootID: 5},
			{RootID: 10},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.threads = []model.CommentThreadDTO{{RootID: 10}} // reload dropped thread 5
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id": 99, "body": "reply", "pull_request_review_id": 7}`))
	}))
	defer srv.Close()
	app.ghClient = github.NewClient(nil)
	app.ghClient.BaseURL, _ = url.Parse(srv.URL + "/")

	dto, err := app.ReplyToComment(10, "reply")
	require.NoError(t, err)
	assert.Equal(t, int64(7), dto.ReviewID)
	require.Len(t, app.threads, 1)
	assert.Len(t, app.threads[0].Comments, 1)
}
