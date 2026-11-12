package app

import (
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
