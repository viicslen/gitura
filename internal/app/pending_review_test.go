package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-github/v93/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitura/internal/model"
)

func TestUpdateAndDeleteDraftComment_StaleOrBlankThenValid_RejectsThenApplies(t *testing.T) {
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

func TestPostImmediateComment_WholeFile_SendsFileSubjectWithoutLine(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id": 1}`))
	}))
	defer srv.Close()
	base := srv.URL + "/"
	client, err := github.NewClient(github.WithURLs(&base, nil))
	require.NoError(t, err)
	app := &App{ctx: context.Background(), ghClient: client, prCache: &model.PullRequestSummary{HeadSHA: "sha"}}

	_, err = app.PostImmediateComment(model.DraftCommentDTO{Path: "a.go", Side: "RIGHT", Body: "x"})
	require.NoError(t, err)
	assert.Equal(t, "file", got["subject_type"])
	assert.NotContains(t, got, "line")
	assert.NotContains(t, got, "side")
}

func TestSubmitReview_WithFileComment_CreatesPendingReviewWithLineCommentsOnly(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	base := srv.URL + "/"
	client, err := github.NewClient(github.WithURLs(&base, nil))
	require.NoError(t, err)
	app := &App{ctx: context.Background(), ghClient: client, prCache: &model.PullRequestSummary{HeadSHA: "sha"}}
	app.pendingComments = []model.DraftCommentDTO{
		{Path: "a.go", Line: 3, Side: "RIGHT", Body: "line"},
		{Path: "a.go", Side: "RIGHT", Body: "file"},
	}

	_, err = app.SubmitReview(model.ReviewSubmitRequest{Verdict: "APPROVE", Body: "lgtm"})
	require.Error(t, err)
	assert.NotContains(t, got, "event", "the review must stay pending until file comments join it")
	assert.Len(t, got["comments"], 1)
	assert.Len(t, app.pendingComments, 2, "a failed submit keeps the drafts")
}

func TestSubmitReview_DeletingPendingReviewFails_KeepsItForRetry(t *testing.T) {
	var posted bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posted = posted || r.Method == http.MethodPost
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	base := srv.URL + "/"
	client, err := github.NewClient(github.WithURLs(&base, nil))
	require.NoError(t, err)
	app := &App{ctx: context.Background(), ghClient: client, prCache: &model.PullRequestSummary{HeadSHA: "sha"}, pendingReviewID: 5}
	app.pendingComments = []model.DraftCommentDTO{{Path: "a.go", Side: "RIGHT", Body: "file"}}

	_, err = app.SubmitReview(model.ReviewSubmitRequest{Verdict: "COMMENT"})
	require.ErrorContains(t, err, "github: delete pending review")
	assert.Equal(t, int64(5), app.pendingReviewID)
	assert.False(t, posted, "no new review while the old pending one may still exist")
}

func TestAddDraftComment_HeadMovedAfterFirstDraft_KeepsOriginalCommit(t *testing.T) {
	client, err := github.NewClient()
	require.NoError(t, err)
	app := &App{ghClient: client, prCache: &model.PullRequestSummary{HeadSHA: "old"}}

	pending, err := app.AddDraftComment(model.DraftCommentDTO{Path: "a.go", Line: 1, Side: "RIGHT", Body: "x"})
	require.NoError(t, err)
	assert.Equal(t, "old", pending.HeadSHA)

	app.prCache.HeadSHA = "new" // author pushed, user refreshed
	_, err = app.AddDraftComment(model.DraftCommentDTO{Path: "a.go", Line: 2, Side: "RIGHT", Body: "y"})
	require.ErrorContains(t, err, "conflict:")
	assert.Len(t, app.pendingComments, 1)
	assert.Equal(t, "old", app.reviewCommitSHA())

	app.pendingComments = nil
	assert.Equal(t, "new", app.reviewCommitSHA())
}
