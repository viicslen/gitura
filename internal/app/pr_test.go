package app

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitura/internal/model"
	"gitura/internal/settings"
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

func TestLoadPR_DraftForPRAAroundSwitchToPRB_NeverLandsOnPRB(t *testing.T) {
	fakeGraphQL(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"repository":{"pullRequest":{"reviewThreads":{"nodes":[],"pageInfo":{"hasNextPage":false}}}}}}`))
	})
	// loadB is an App on PR A (head "sha") whose fetch of PR B (head "shaB") waits for release once fetching is closed.
	loadB := func(t *testing.T) (app *App, fetching chan struct{}, release chan struct{}) {
		fetching, release = make(chan struct{}), make(chan struct{})
		var once sync.Once
		app = subAppRaw(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/repos/o/r/pulls/2" {
				once.Do(func() { close(fetching) })
				<-release
				_, _ = w.Write([]byte(`{"number":2,"head":{"sha":"shaB"}}`))
				return
			}
			_, _ = w.Write([]byte(`[]`))
		})
		app.cfg.Store(&settings.Config{})
		return app, fetching, release
	}
	draftOnA := model.DraftCommentDTO{Path: "a.go", Line: 1, Side: "RIGHT", Body: "x", HeadSHA: "sha"}

	t.Run("draft added while PR B loads", func(t *testing.T) {
		app, fetching, release := loadB(t)
		loaded := make(chan error, 1)
		go func() {
			_, err := app.LoadPR("o", "r", 2)
			loaded <- err
		}()
		select {
		case <-fetching:
		case err := <-loaded:
			t.Fatalf("LoadPR returned before fetching PR B: %v", err)
		case <-time.After(10 * time.Second):
			t.Fatal("LoadPR never fetched PR B")
		}
		_, err := app.AddDraftComment(draftOnA)
		assert.NoError(t, err, "PR A is still loaded")
		close(release)
		require.NoError(t, <-loaded)

		assert.Equal(t, 2, app.prNumber)
		assert.Empty(t, app.pendingComments)
		assert.Empty(t, app.pendingHeadSHA)
	})

	t.Run("draft added after PR B loaded", func(t *testing.T) {
		app, _, release := loadB(t)
		close(release)
		_, err := app.LoadPR("o", "r", 2)
		require.NoError(t, err)

		_, err = app.AddDraftComment(draftOnA)
		require.ErrorContains(t, err, "conflict:")
		assert.Empty(t, app.pendingComments)

		_, err = app.AddDraftComment(model.DraftCommentDTO{Path: "a.go", Line: 1, Side: "RIGHT", Body: "y"})
		require.NoError(t, err)
		assert.Equal(t, "shaB", app.pendingHeadSHA, "a draft on PR B is pinned to PR B's head")
	})
}
