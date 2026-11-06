package main

import (
	"testing"

	"github.com/google/go-github/v67/github"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitura/internal/model"
)

func TestGetPROverview_FiltersResolvedReviewsAndEmptyThreadlessReviews(t *testing.T) {
	app := &App{
		prCache:    &model.PullRequestSummary{Number: 42, Title: "PR"},
		prComments: []model.PRCommentDTO{{ID: 1, Body: "hello"}},
		reviews: []model.ReviewOverviewDTO{
			{ReviewID: 10, AllThreadsResolved: true, HasThreads: true},
			{ReviewID: 11, AllThreadsResolved: false, HasThreads: true},
			{ReviewID: 12, AllThreadsResolved: false, HasThreads: false},
			{ReviewID: 13, Body: "Has summary", AllThreadsResolved: false, HasThreads: false},
			{ReviewID: 14, State: "APPROVED"},
		},
	}

	overview, err := app.GetPROverview(false)
	require.NoError(t, err)
	assert.Equal(t, 42, overview.Summary.Number)
	require.Len(t, overview.GeneralComments, 1)
	require.Len(t, overview.Reviews, 3)
	assert.Equal(t, int64(11), overview.Reviews[0].ReviewID)
	assert.Equal(t, int64(13), overview.Reviews[1].ReviewID)
	assert.Equal(t, int64(14), overview.Reviews[2].ReviewID)
}

func TestRefreshReviewResolutionState_RecomputesFromThreads(t *testing.T) {
	app := &App{
		reviews: []model.ReviewOverviewDTO{{ReviewID: 10}},
		threads: []model.CommentThreadDTO{
			{ReviewID: 10, Resolved: true},
			{ReviewID: 10, Resolved: true},
		},
	}

	app.refreshReviewResolutionState()

	require.Len(t, app.reviews, 1)
	assert.True(t, app.reviews[0].AllThreadsResolved)
	assert.Equal(t, 2, app.reviews[0].ResolvedThreadCount)
	assert.Equal(t, 2, app.reviews[0].TotalThreadCount)
}

func TestIsLoadedPR(t *testing.T) {
	app := &App{}
	assert.False(t, app.isLoadedPR("o", "r", 7))

	app = &App{prCache: &model.PullRequestSummary{}, prOwner: "o", prRepo: "r", prNumber: 7}
	assert.True(t, app.isLoadedPR("o", "r", 7))
	assert.False(t, app.isLoadedPR("o", "r", 8))
	assert.False(t, app.isLoadedPR("o", "x", 7))
}

func TestDraftsKeepTheCommitTheyWereWrittenAgainst(t *testing.T) {
	app := &App{ghClient: github.NewClient(nil), prCache: &model.PullRequestSummary{HeadSHA: "old"}}

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
