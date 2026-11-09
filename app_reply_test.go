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

func TestDeleteComment_DropsReviewCommentAndEmptyThread(t *testing.T) {
	app := &App{
		ctx:      context.Background(),
		prOwner:  "o",
		prRepo:   "r",
		prNumber: 1,
		prCache:  &model.PullRequestSummary{},
		threads: []model.CommentThreadDTO{
			{RootID: 5, Comments: []model.CommentDTO{{ID: 5}}},
			{RootID: 10, Comments: []model.CommentDTO{{ID: 10}, {ID: 11}}},
		},
	}
	var deleted []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deleted = append(deleted, r.Method+" "+r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	app.ghClient = github.NewClient(nil)
	app.ghClient.BaseURL, _ = url.Parse(srv.URL + "/")

	require.NoError(t, app.DeleteComment("review_comment", 5))
	require.NoError(t, app.DeleteComment("review_comment", 11))

	assert.Equal(t, []string{"DELETE /repos/o/r/pulls/comments/5", "DELETE /repos/o/r/pulls/comments/11"}, deleted)
	require.Len(t, app.threads, 1)
	assert.Equal(t, []model.CommentDTO{{ID: 10}}, app.threads[0].Comments)
	assert.ErrorContains(t, app.DeleteComment("review", 1), "validation:")
}

func TestDraftComment_EditAndDelete(t *testing.T) {
	app := &App{pendingComments: []model.DraftCommentDTO{{Body: "a"}, {Body: "b"}}, pendingHeadSHA: "sha"}

	_, err := app.UpdateDraftComment(1, "stale", "x")
	assert.ErrorContains(t, err, "conflict:")
	_, err = app.UpdateDraftComment(1, "b", "  ")
	assert.ErrorContains(t, err, "validation:")

	got, err := app.UpdateDraftComment(1, "b", "b2")
	require.NoError(t, err)
	assert.Equal(t, "b2", got.Comments[1].Body)

	got, err = app.DeleteDraftComment(0, "a")
	require.NoError(t, err)
	assert.Equal(t, []model.DraftCommentDTO{{Body: "b2"}}, got.Comments)

	got, err = app.DeleteDraftComment(0, "b2")
	require.NoError(t, err)
	assert.False(t, got.HasPending)
	assert.Empty(t, app.pendingHeadSHA)
}
