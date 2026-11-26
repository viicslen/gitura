package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
	app := &App{ctx: context.Background(), ghClient: client, prCache: &model.PullRequestSummary{HeadSHA: "sha"}, prFilesCache: []*github.CommitFile{}}

	_, err = app.PostImmediateComment(model.DraftCommentDTO{Path: "a.go", Side: "RIGHT", Body: "x"})
	require.NoError(t, err)
	assert.Equal(t, "file", got["subject_type"])
	assert.NotContains(t, got, "line")
	assert.NotContains(t, got, "side")
}

func TestSubmitReview_WithFileComment_CreatesPendingReviewWithLineCommentsOnly(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		if r.Method == http.MethodPost {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	base := srv.URL + "/"
	client, err := github.NewClient(github.WithURLs(&base, nil))
	require.NoError(t, err)
	app := &App{ctx: context.Background(), ghClient: client, prCache: &model.PullRequestSummary{HeadSHA: "sha"}, prFilesCache: []*github.CommitFile{}}
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

func TestSubmitReview_PendingCreateAnswers5xx_RetryDeletesOnlyTheReviewItLeft(t *testing.T) {
	for name, tc := range map[string]struct {
		before     bool
		remoteBody string
		deleted    []string
	}{
		"left by the failed request":        {false, "line", []string{"/repos/o/r/pulls/1/reviews/9"}},
		"started on github.com":             {false, "mine on the web", nil},
		"there before, matching the drafts": {true, "line", nil},
	} {
		t.Run(name, func(t *testing.T) {
			var deleted []string
			created := false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPost && !created:
					created = true
					w.WriteHeader(http.StatusBadGateway)
				case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/reviews/9/comments"):
					_, _ = w.Write([]byte(`[{"body":"` + tc.remoteBody + `"}]`))
				case r.Method == http.MethodGet && r.URL.Query().Get("page") == "":
					w.Header().Set("Link", `<http://`+r.Host+r.URL.Path+`?page=2>; rel="next"`)
					_, _ = w.Write([]byte(`[{"id":1,"state":"COMMENTED"}]`))
				case r.Method == http.MethodGet && (created || tc.before):
					_, _ = w.Write([]byte(`[{"id":9,"state":"PENDING","commit_id":"sha"}]`))
				case r.Method == http.MethodGet:
					_, _ = w.Write([]byte(`[]`))
				case r.Method == http.MethodDelete:
					deleted = append(deleted, r.URL.Path)
					_, _ = w.Write([]byte(`{}`))
				default:
					w.WriteHeader(http.StatusUnprocessableEntity)
				}
			}))
			defer srv.Close()
			base := srv.URL + "/"
			client, err := github.NewClient(github.WithURLs(&base, nil))
			require.NoError(t, err)
			app := &App{ctx: context.Background(), ghClient: client, prCache: &model.PullRequestSummary{HeadSHA: "sha"}, prFilesCache: []*github.CommitFile{}, prOwner: "o", prRepo: "r", prNumber: 1}
			app.pendingComments = []model.DraftCommentDTO{
				{Path: "a.go", Line: 3, Side: "RIGHT", Body: "line"},
				{Path: "a.go", Side: "RIGHT", Body: "file"},
			}

			_, err = app.SubmitReview(model.ReviewSubmitRequest{Verdict: "COMMENT"})
			require.Error(t, err)
			_, _ = app.SubmitReview(model.ReviewSubmitRequest{Verdict: "COMMENT"})
			assert.Equal(t, tc.deleted, deleted)
		})
	}
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
	app := &App{ctx: context.Background(), ghClient: client, prCache: &model.PullRequestSummary{HeadSHA: "sha"}, prFilesCache: []*github.CommitFile{}, pendingReviewID: 5}
	app.pendingComments = []model.DraftCommentDTO{{Path: "a.go", Side: "RIGHT", Body: "file"}}

	_, err = app.SubmitReview(model.ReviewSubmitRequest{Verdict: "COMMENT"})
	require.ErrorContains(t, err, "github: delete pending review")
	assert.Equal(t, int64(5), app.pendingReviewID)
	assert.False(t, posted, "no new review while the old pending one may still exist")
}

func TestAddDraftComment_HeadMovedAfterFirstDraft_KeepsOriginalCommit(t *testing.T) {
	client, err := github.NewClient()
	require.NoError(t, err)
	app := &App{ghClient: client, prCache: &model.PullRequestSummary{HeadSHA: "old"}, prFilesCache: []*github.CommitFile{}}

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

func TestAddDraftComment_WrittenAgainstAnotherHead_RejectedAndNotStored(t *testing.T) {
	client, err := github.NewClient()
	require.NoError(t, err)
	app := &App{ghClient: client, prCache: &model.PullRequestSummary{HeadSHA: "prB"}, prFilesCache: []*github.CommitFile{}}

	_, err = app.AddDraftComment(model.DraftCommentDTO{Path: "a.go", Line: 1, Side: "RIGHT", Body: "x", HeadSHA: "prA"})
	require.ErrorContains(t, err, "conflict:")
	assert.Empty(t, app.pendingComments)

	_, err = app.PostImmediateComment(model.DraftCommentDTO{Path: "a.go", Line: 1, Side: "RIGHT", Body: "x", HeadSHA: "prA"})
	require.ErrorContains(t, err, "conflict:")
}

func TestSyncPendingReview_DraftsAtPointerRowAndFileReplacingIt_RestoresEachOnItsRow(t *testing.T) {
	thread := func(path string, line int, side, hunk string) map[string]any {
		th := map[string]any{"diffSide": side, "subjectType": "LINE", "originalLine": line, "comments": map[string]any{"nodes": []map[string]any{
			{"pullRequestReview": map[string]any{"databaseId": 9}, "path": path, "body": hunk, "diffHunk": hunk},
		}}}
		if line == 0 {
			th["subjectType"], th["originalLine"] = "FILE", nil
		}
		return th
	}
	threads := []map[string]any{
		thread("vendor", 3, "RIGHT", "@@ -0,0 +1,3 @@\n+x\n+y\n+z"),
		thread("vendor", 1, "LEFT", "@@ -1 +1 @@\n-Subproject commit a"),
		thread("vendor", 1, "RIGHT", "@@ -1 +1 @@\n-Subproject commit a\n+Subproject commit b"),
		thread("vendor", 1, "RIGHT", "@@ -0,0 +1,3 @@\n+x"),
		thread("vendor", 0, "RIGHT", "file-level"),
		thread("solo", 1, "RIGHT", "@@ -1 +1 @@\n-Subproject commit c\n+Subproject commit d"),
	}
	fakeGraphQL(t, func(w http.ResponseWriter, _ *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"repository": map[string]any{"pullRequest": map[string]any{
			"reviewThreads": map[string]any{"nodes": threads, "pageInfo": map[string]any{"hasNextPage": false}},
		}}}}))
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"id":9,"state":"PENDING","commit_id":"sha"}]`))
	}))
	defer srv.Close()
	base := srv.URL + "/"
	client, err := github.NewClient(github.WithURLs(&base, nil))
	require.NoError(t, err)
	app := &App{
		ctx: context.Background(), ghClient: client, prOwner: "o", prRepo: "r", prNumber: 1,
		prCache: &model.PullRequestSummary{HeadSHA: "sha"},
		prFilesCache: []*github.CommitFile{
			{Filename: new("vendor"), Status: new("modified"), Patch: new("@@ -1 +1 @@\n-Subproject commit a\n+Subproject commit b")},
			{Filename: new("vendor"), Status: new("added"), Patch: new("@@ -0,0 +1,3 @@\n+x\n+y\n+z")},
			{Filename: new("solo"), Status: new("modified"), Patch: new("@@ -1 +1 @@\n-Subproject commit c\n+Subproject commit d")},
		},
		prSubmodules:  map[string]*prSubmodule{"vendor": {path: "vendor"}, "solo": {path: "solo"}},
		prSubmoduleOf: map[*github.CommitFile]*prSubmodule{},
	}

	state, err := app.SyncPendingReview()
	require.NoError(t, err)
	var pointer []bool
	for _, d := range state.Comments {
		pointer = append(pointer, d.Pointer)
		assert.Empty(t, d.Submodule, d.Body)
	}
	assert.Equal(t, []bool{false, true, true, false, false, true}, pointer,
		"only the file's hunk holds RIGHT 3; only the pointer's LEFT 1; RIGHT 1 by its line; a file-level draft goes to the file; a lone row is itself")
}
