package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitura/internal/model"
)

func TestGetPROverview_ResolvedAndEmptyReviews_FiltersThemOut(t *testing.T) {
	app := &App{
		prCache:       &model.PullRequestSummary{Number: 42, Title: "PR"},
		issueComments: []model.IssueCommentDTO{{ID: 1, Body: "hello"}},
		reviews: []model.ReviewDTO{
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
	assert.Equal(t, map[int64]int{10: 1, 11: 2, 13: 3, 14: 4}, overview.ReviewNumbers)
}

func TestRefreshReviewResolutionState_ThreadsChanged_RecomputesFromThreads(t *testing.T) {
	app := &App{
		reviews: []model.ReviewDTO{{ReviewID: 10}},
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

func TestIsLoadedPR_OwnerRepoNumber_MatchesOnlyLoadedPR(t *testing.T) {
	app := &App{}
	assert.False(t, app.isLoadedPR("o", "r", 7))

	app = &App{prCache: &model.PullRequestSummary{}, prOwner: "o", prRepo: "r", prNumber: 7}
	assert.True(t, app.isLoadedPR("o", "r", 7))
	assert.False(t, app.isLoadedPR("o", "r", 8))
	assert.False(t, app.isLoadedPR("o", "x", 7))
}
