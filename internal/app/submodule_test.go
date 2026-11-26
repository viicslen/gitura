package app

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-github/v93/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitura/internal/db"
	"gitura/internal/ghapi"
	"gitura/internal/model"
	"gitura/internal/unidiff"
)

const subPatch = "@@ -1,3 +1,4 @@\n a\n+b\n c\n d"

// subApp is an App on PR o/r#1 that bumps submodule lib (so/sr), whose open PR #5 changes a.txt but not b.txt.
// Reading a pull request again finds it still open at "new"; subAppRaw leaves that to h.
func subApp(t *testing.T, h http.HandlerFunc) *App {
	t.Helper()
	return subAppRaw(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && openPRPath.MatchString(r.URL.Path) {
			_, _ = w.Write([]byte(`{"state":"open","head":{"sha":"new"}}`))
			return
		}
		h(w, r)
	})
}

var openPRPath = regexp.MustCompile(`^/repos/[^/]+/[^/]+/pulls/\d+$`)

func subAppRaw(t *testing.T, h http.HandlerFunc) *App {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	base := srv.URL + "/"
	client, err := github.NewClient(github.WithURLs(&base, nil))
	require.NoError(t, err)
	sqlDB, err := db.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	app := &App{
		ctx: context.Background(), ghClient: client, prOwner: "o", prRepo: "r", prNumber: 1, login: "me",
		prCache:       &model.PullRequestSummary{HeadSHA: "sha"},
		prSubmodules:  map[string]*prSubmodule{},
		prSubmoduleOf: map[*github.CommitFile]*prSubmodule{},
		subViewed:     map[fileID]string{},
		queries:       db.New(sqlDB),
	}
	addSub(app, "lib", "so", "sr", 5)
	return app
}

// addSub adds to app a bump of submodule path (owner/repo) whose open PR number changes a.txt but not b.txt.
func addSub(app *App, path, owner, repo string, number int) {
	sub := &prSubmodule{
		SubmoduleChanges: ghapi.SubmoduleChanges{Owner: owner, Repo: repo, Files: []*github.CommitFile{
			{Filename: new(path + "/a.txt"), Patch: new(subPatch)},
			{Filename: new(path + "/b.txt"), Patch: new("@@ -1 +1 @@\n-x\n+y")},
		}},
		path: path, to: "new",
		pr:      &github.PullRequest{Number: new(number), Head: &github.PullRequestBranch{SHA: new("new")}},
		prFiles: map[string][]*github.CommitFile{"a.txt": {{Filename: new("a.txt"), Patch: new(subPatch)}}},
	}
	app.prFilesCache = append(app.prFilesCache, sub.Files...)
	app.prSubmodules[path] = sub
	for _, f := range sub.Files {
		app.prSubmoduleOf[f] = sub
	}
}

// fakeGraphQL sends the app's requests to api.github.com, the GraphQL endpoint, to h until the test ends.
func fakeGraphQL(t *testing.T, h http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	orig := http.DefaultTransport
	http.DefaultTransport = toHost{host: strings.TrimPrefix(srv.URL, "http://"), base: orig}
	t.Cleanup(func() { http.DefaultTransport = orig })
}

type toHost struct {
	host string
	base http.RoundTripper
}

func (t toHost) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Host == "api.github.com" {
		r = r.Clone(r.Context())
		r.URL.Scheme, r.URL.Host = "http", t.host
	}
	return t.base.RoundTrip(r)
}

func TestDraftInHunks_LinesAndRanges_TrueOnlyInsideOneHunk(t *testing.T) {
	hunks := unidiff.ParseHunks("@@ -1,3 +1,4 @@\n a\n+b\n c\n d\n@@ -20,2 +21,2 @@\n-x\n+y\n z")
	cases := []struct {
		name string
		d    model.DraftCommentDTO
		want bool
	}{
		{"right line in first hunk", model.DraftCommentDTO{Line: 4, Side: "RIGHT"}, true},
		{"right line between hunks", model.DraftCommentDTO{Line: 5, Side: "RIGHT"}, false},
		{"left line in second hunk", model.DraftCommentDTO{Line: 21, Side: "LEFT"}, true},
		{"left line past second hunk", model.DraftCommentDTO{Line: 22, Side: "LEFT"}, false},
		{"range in one hunk", model.DraftCommentDTO{StartLine: 1, StartSide: "RIGHT", Line: 3, Side: "RIGHT"}, true},
		{"range across hunks", model.DraftCommentDTO{StartLine: 3, StartSide: "RIGHT", Line: 21, Side: "RIGHT"}, false},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, draftInHunks(c.d, hunks), c.name)
	}
}

func TestFoldComments_LineRangeFileMultilineBodyAndSharedPath_FormatsOneBulletEachNamingItsRepo(t *testing.T) {
	got := withFolded("lgtm", []model.DraftCommentDTO{
		{Submodule: "lib", Bump: "..new", Path: "lib/a.txt", Line: 12, Side: "RIGHT", Body: "one"},
		{Submodule: "lib", Bump: "..new", Path: "lib/a.txt", StartLine: 10, Line: 12, Side: "LEFT", Body: "two\nlines"},
		{Submodule: "lib", Bump: "..new", Path: "lib/b.txt", Body: "whole"},
		{Submodule: "lib/nest", Bump: "..new", Path: "lib/nest/a.txt", Line: 12, Side: "RIGHT", Body: "nested"},
		{Submodule: "lib", Bump: "..new", Path: "lib/nest", Pointer: true, Line: 1, Side: "RIGHT", Body: "bump"},
	})
	assert.Equal(t, "lgtm\n\n**Comments on submodule files**\n"+
		"\n- `lib/a.txt` (submodule lib) L12 — one"+
		"\n- `lib/a.txt` (submodule lib) L10–12 (old) — two\n  lines"+
		"\n- `lib/b.txt` (submodule lib) file — whole"+
		"\n- `lib/nest/a.txt` (submodule lib/nest) L12 — nested"+
		"\n- `lib/nest` (pointer, in submodule lib) L1 — bump", got)
	assert.Equal(t, "body", withFolded("body", nil))
}

func TestSubmitReview_SubmoduleDrafts_PostsToSubmodulePROnceAndFoldsTheRest(t *testing.T) {
	var subReviews, parentReviews []map[string]any
	parentFails := true
	app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		switch r.URL.Path {
		case "/repos/so/sr/pulls/5/reviews":
			subReviews = append(subReviews, body)
			_, _ = w.Write([]byte(`{"id":7,"html_url":"https://sub/review"}`))
		case "/repos/o/r/pulls/1/reviews":
			parentReviews = append(parentReviews, body)
			if parentFails {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			_, _ = w.Write([]byte(`{"id":9,"html_url":"https://parent/review"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	app.pendingComments = []model.DraftCommentDTO{
		{Submodule: "lib", Bump: "..new", Path: "lib/a.txt", Line: 2, Side: "RIGHT", Body: "in PR diff"},
		{Submodule: "lib", Bump: "..new", Path: "lib/a.txt", Line: 50, Side: "RIGHT", Body: "outside hunks"},
		{Submodule: "lib", Bump: "..new", Path: "lib/b.txt", Line: 1, Side: "RIGHT", Body: "not in PR"},
		{Path: "a.go", Line: 3, Side: "RIGHT", Body: "parent"},
	}

	req := model.ReviewSubmitRequest{Verdict: "APPROVE", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "review", Verdict: "COMMENT"}}}
	_, err := app.SubmitReview(shown(t, app, req))
	require.Error(t, err)
	assert.Len(t, app.pendingComments, 3, "posted drafts leave, folded ones stay for the retry")
	parentFails = false
	res, err := app.SubmitReview(shown(t, app, req))
	require.NoError(t, err)

	require.Len(t, subReviews, 1, "a retry must not post the submodule review again")
	assert.Equal(t, "COMMENT", subReviews[0]["event"])
	assert.Equal(t, "new", subReviews[0]["commit_id"])
	assert.Equal(t, []any{map[string]any{"path": "a.txt", "body": "in PR diff", "line": 2.0, "side": "RIGHT"}}, subReviews[0]["comments"])

	last := parentReviews[len(parentReviews)-1]
	assert.Equal(t, []any{map[string]any{"path": "a.go", "body": "parent", "line": 3.0, "side": "RIGHT"}}, last["comments"])
	assert.Contains(t, last["body"], "- `lib/a.txt` (submodule lib) L50 — outside hunks")
	assert.Contains(t, last["body"], "- `lib/b.txt` (submodule lib) L1 — not in PR")
	assert.Equal(t, model.ReviewSubmitResult{
		ReviewID: 9, HTMLURL: "https://parent/review",
		SubmoduleReviewURLs: []string{"https://sub/review"}, FoldedComments: 2,
	}, res)
	assert.Empty(t, app.pendingComments)
}

func TestPostImmediateComment_SubmoduleFileOutsidePRDiff_PostsFoldedConversationComment(t *testing.T) {
	var path string
	var body map[string]any
	app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":1}`))
	})

	_, err := app.PostImmediateComment(model.DraftCommentDTO{Submodule: "lib", Bump: "..new", Path: "lib/b.txt", Line: 1, Side: "RIGHT", Body: "hm"})
	require.NoError(t, err)
	assert.Equal(t, "/repos/o/r/issues/1/comments", path)
	assert.Equal(t, "**Comments on submodule files**\n\n- `lib/b.txt` (submodule lib) L1 — hm", body["body"])

	_, err = app.PostImmediateComment(model.DraftCommentDTO{Submodule: "lib", Bump: "..new", Path: "lib/a.txt", Line: 2, Side: "RIGHT", Body: "ok"})
	require.NoError(t, err)
	assert.Equal(t, "/repos/so/sr/pulls/5/comments", path)
	assert.Equal(t, "a.txt", body["path"])
	assert.Equal(t, "new", body["commit_id"])
}

func TestSetFileViewed_SubmoduleFileWithoutPR_KeptInDatabaseUntilChangeDiffers(t *testing.T) {
	app := subApp(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	app.prSubmodules["lib"].checkedAt = time.Now()
	f := app.bumpFile(app.prSubmodules["lib"], "lib/b.txt", false)
	f.SHA = new("blob1")

	require.NoError(t, viewedErr(app.SetFileViewed("lib", "lib/b.txt", false, true)))
	assert.True(t, app.prFileDTO(f).Viewed)
	assert.True(t, app.prFileDTO(f).ViewedLocal)
	assert.False(t, app.prFileDTO(app.bumpFile(app.prSubmodules["lib"], "lib/a.txt", false)).ViewedLocal, "a.txt is in the submodule PR")

	app.subViewed = nil
	app.loadSubViewed()
	assert.True(t, app.prFileDTO(f).Viewed, "the mark outlives a reload")

	f.SHA = new("blob2") // a binary file's new content: no patch to tell it apart
	app.loadSubViewed()
	assert.False(t, app.prFileDTO(f).Viewed)
	rows, err := app.queries.ListSubmoduleViewed(app.ctx, db.ListSubmoduleViewedParams{Owner: "o", Repo: "r", Number: 1})
	require.NoError(t, err)
	assert.Empty(t, rows, "a stale mark is deleted")
}

func TestSetFileViewed_DirectoryTurnedNestedSubmodule_MarksKeptPerRepoAcrossReload(t *testing.T) {
	app := subApp(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	lib := app.prSubmodules["lib"]
	lib.pr, lib.checkedAt = nil, time.Now()
	removed := &github.CommitFile{Filename: new("lib/vendor/a.txt"), Status: new("removed"), Patch: new("@@ -1 +0,0 @@\n-x")}
	added := &github.CommitFile{Filename: new("lib/vendor/a.txt"), Status: new("added"), Patch: new("@@ -0,0 +1 @@\n+y")}
	lib.Files = append(lib.Files, removed)
	vendor := &prSubmodule{SubmoduleChanges: ghapi.SubmoduleChanges{Files: []*github.CommitFile{added}}, path: "lib/vendor", to: "v2", in: lib, checkedAt: time.Now()}
	app.prSubmodules["lib/vendor"] = vendor
	app.prSubmoduleOf[removed], app.prSubmoduleOf[added] = lib, vendor

	require.NoError(t, viewedErr(app.SetFileViewed("lib", "lib/vendor/a.txt", false, true)))
	require.NoError(t, viewedErr(app.SetFileViewed("lib/vendor", "lib/vendor/a.txt", false, true)))
	app.loadSubViewed()
	assert.True(t, app.prFileDTO(removed).Viewed)
	assert.True(t, app.prFileDTO(added).Viewed)

	require.NoError(t, viewedErr(app.SetFileViewed("lib", "lib/vendor/a.txt", false, false)))
	app.loadSubViewed()
	assert.False(t, app.prFileDTO(removed).Viewed)
	assert.True(t, app.prFileDTO(added).Viewed, "unmarking the removed file leaves the added one's mark")
}

// recordReviews answers every review POST, recording its body by URL path.
func recordReviews(t *testing.T, got map[string][]map[string]any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		got[r.URL.Path] = append(got[r.URL.Path], body)
		_, _ = w.Write([]byte(`{"id":7,"html_url":"https://review"}`))
	}
}

func subDrafts() []model.DraftCommentDTO {
	return []model.DraftCommentDTO{
		{Submodule: "lib", Bump: "..new", Path: "lib/a.txt", Line: 2, Side: "RIGHT", Body: "in PR diff"},
		{Submodule: "lib", Bump: "..new", Path: "lib/b.txt", Line: 1, Side: "RIGHT", Body: "not in PR"},
	}
}

func TestSubmitReview_SubmoduleReviewChoice_PostsItsVerdictAndText(t *testing.T) {
	for _, verdict := range []string{"APPROVE", "REQUEST_CHANGES"} {
		got := map[string][]map[string]any{}
		app := subApp(t, recordReviews(t, got))
		app.pendingComments = subDrafts()

		_, err := app.SubmitReview(shown(t, app, model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{
			{Path: "lib", Mode: "review", Verdict: verdict, Body: "Looks fine."},
		}}))
		require.NoError(t, err)

		sub := got["/repos/so/sr/pulls/5/reviews"]
		require.Len(t, sub, 1, verdict)
		assert.Equal(t, verdict, sub[0]["event"])
		assert.Equal(t, "Looks fine.", sub[0]["body"])
		assert.Contains(t, got["/repos/o/r/pulls/1/reviews"][0]["body"], "- `lib/b.txt` (submodule lib) L1 — not in PR")
	}
}

func TestSubmitReview_SubmoduleFoldChoice_AddsAllItsDraftsToTheReviewBody(t *testing.T) {
	got := map[string][]map[string]any{}
	app := subApp(t, recordReviews(t, got))
	app.pendingComments = subDrafts()

	res, err := app.SubmitReview(shown(t, app, model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "fold"}}}))
	require.NoError(t, err)

	assert.Empty(t, got["/repos/so/sr/pulls/5/reviews"])
	body := got["/repos/o/r/pulls/1/reviews"][0]["body"]
	assert.Contains(t, body, "- `lib/a.txt` (submodule lib) L2 — in PR diff")
	assert.Contains(t, body, "- `lib/b.txt` (submodule lib) L1 — not in PR")
	assert.Equal(t, 2, res.FoldedComments)
	assert.Empty(t, res.SubmoduleReviewURLs)
}

func TestSubmitReview_InvalidChoice_ReturnsValidationErrorWithoutPosting(t *testing.T) {
	cases := map[string]model.ReviewSubmitRequest{
		"parent verdict":           {Verdict: "LGTM"},
		"unknown path":             {Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{{Path: "other", Mode: "fold"}}},
		"unknown mode":             {Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "skip"}}},
		"comments on unknown path": {Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{{Path: "other", Mode: "comments"}}},
		"unknown verdict": {Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{
			{Path: "lib", Mode: "review", Verdict: "MERGE"},
		}},
	}
	for name, req := range cases {
		got := map[string][]map[string]any{}
		app := subApp(t, recordReviews(t, got))
		app.pendingComments = subDrafts()

		_, err := app.SubmitReview(shown(t, app, req))
		require.ErrorContains(t, err, "validation:", name)
		assert.Empty(t, got, name)
		assert.Len(t, app.pendingComments, 2, name)
	}
}

func TestSubmoduleDrafts_DraftsOnSubmoduleFiles_CountsWhereEachGoes(t *testing.T) {
	app := subApp(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	app.pendingComments = append(subDrafts(), model.DraftCommentDTO{Path: "a.go", Line: 1, Side: "RIGHT", Body: "parent"})

	got, err := app.SubmoduleDrafts(false)
	require.NoError(t, err)
	assert.Equal(t, []model.SubmoduleDraftsDTO{{Path: "lib", PR: "so/sr#5", ToPR: 1, Folded: 1}}, got)

	app.prSubmodules["lib"].from = "older"
	got, err = app.SubmoduleDrafts(false)
	require.NoError(t, err)
	assert.Equal(t, 2, got[0].Folded, "drafts on another bump all fold")
}

func TestSubmoduleDrafts_OnlyParentDrafts_ReturnsEmpty(t *testing.T) {
	app := subApp(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	app.pendingComments = []model.DraftCommentDTO{{Path: "a.go", Line: 1, Side: "RIGHT", Body: "parent"}}

	got, err := app.SubmoduleDrafts(false)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestSubmitReview_SubmoduleCommentsChoice_PostsEachDraftAloneWithoutAReview(t *testing.T) {
	got := map[string][]map[string]any{}
	app := subApp(t, recordReviews(t, got))
	app.pendingComments = append(subDrafts(), model.DraftCommentDTO{Submodule: "lib", Bump: "..new", Path: "lib/a.txt", Body: "whole file"})

	res, err := app.SubmitReview(shown(t, app, model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "comments"}}}))
	require.NoError(t, err)

	assert.Empty(t, got["/repos/so/sr/pulls/5/reviews"])
	assert.Equal(t, []map[string]any{
		{"path": "a.txt", "body": "in PR diff", "commit_id": "new", "line": 2.0, "side": "RIGHT"},
		{"path": "a.txt", "body": "whole file", "commit_id": "new", "subject_type": "file"},
	}, got["/repos/so/sr/pulls/5/comments"])
	assert.Contains(t, got["/repos/o/r/pulls/1/reviews"][0]["body"], "- `lib/b.txt` (submodule lib) L1 — not in PR")
	assert.Len(t, res.SubmoduleCommentURLs, 2)
	assert.Equal(t, 1, res.FoldedComments)
}

func TestSubmitReview_SubmoduleCommentsFailPartWay_RetryPostsOnlyTheRest(t *testing.T) {
	posted := map[string]int{}
	failBody, parentFails := "second", true
	app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`[]`)) // nothing went through despite the 502
			return
		}
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		switch {
		case r.URL.Path == "/repos/so/sr/pulls/5/comments" && body["body"] == failBody:
			failBody = ""
			w.WriteHeader(http.StatusBadGateway)
			return
		case r.URL.Path == "/repos/o/r/pulls/1/reviews" && parentFails:
			parentFails = false
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		posted[fmt.Sprint(r.URL.Path, " ", body["body"])]++
		_, _ = w.Write([]byte(`{"id":7,"html_url":"https://x"}`))
	})
	app.pendingComments = []model.DraftCommentDTO{
		{Submodule: "lib", Bump: "..new", Path: "lib/a.txt", Line: 2, Side: "RIGHT", Body: "first"},
		{Submodule: "lib", Bump: "..new", Path: "lib/a.txt", Line: 3, Side: "RIGHT", Body: "second"},
		{Path: "a.go", Line: 1, Side: "RIGHT", Body: "parent"},
	}
	req := model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "comments"}}}

	_, err := app.SubmitReview(shown(t, app, req))
	require.ErrorContains(t, err, "so/sr#5")
	assert.Len(t, app.pendingComments, 2, "the posted comment leaves the drafts")
	_, err = app.SubmitReview(shown(t, app, req))
	require.Error(t, err, "the parent review fails after the second comment went through")
	res, err := app.SubmitReview(shown(t, app, req))
	require.NoError(t, err)

	assert.Equal(t, map[string]int{
		"/repos/so/sr/pulls/5/comments first":  1,
		"/repos/so/sr/pulls/5/comments second": 1,
		"/repos/o/r/pulls/1/reviews ":          1,
	}, posted)
	assert.Len(t, res.SubmoduleCommentURLs, 2)
	assert.Empty(t, app.pendingComments)
}

func TestSubmitReview_SubmodulePendingCreateAnswers5xx_RetryDeletesTheOrphanAndPostsOnce(t *testing.T) {
	var deleted []string
	threads, submits, orphan, failed := 0, 0, false, false
	fakeGraphQL(t, func(w http.ResponseWriter, _ *http.Request) {
		threads++
		_, _ = w.Write([]byte(`{"data":{}}`))
	})
	app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/repos/so/sr/pulls/5/reviews":
			switch {
			case orphan:
				w.WriteHeader(http.StatusUnprocessableEntity)
			case !failed:
				failed, orphan = true, true
				w.WriteHeader(http.StatusBadGateway)
			default:
				_, _ = w.Write([]byte(`{"id":10,"node_id":"PRR_10"}`))
			}
		case r.Method == http.MethodGet && r.URL.Path == "/repos/so/sr/pulls/5/reviews" && !orphan:
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodGet && r.URL.Path == "/repos/so/sr/pulls/5/reviews":
			_, _ = w.Write([]byte(`[{"id":9,"state":"PENDING","commit_id":"new"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/repos/so/sr/pulls/5/reviews/9/comments":
			_, _ = w.Write([]byte(`[{"body":"in PR diff"}]`))
		case r.Method == http.MethodDelete && r.URL.Path == "/repos/so/sr/pulls/5/reviews/9":
			deleted, orphan = append(deleted, r.URL.Path), false
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPost && r.URL.Path == "/repos/so/sr/pulls/5/reviews/10/events":
			submits++
			_, _ = w.Write([]byte(`{"id":10,"html_url":"https://sub/review"}`))
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	})
	app.pendingComments = []model.DraftCommentDTO{
		{Submodule: "lib", Bump: "..new", Path: "lib/a.txt", Line: 2, Side: "RIGHT", Body: "in PR diff"},
		{Submodule: "lib", Bump: "..new", Path: "lib/a.txt", Body: "whole file"},
	}

	req := model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "review", Verdict: "COMMENT"}}}
	_, err := app.SubmitReview(shown(t, app, req))
	require.ErrorContains(t, err, "so/sr#5")
	res, err := app.SubmitReview(shown(t, app, req))
	require.NoError(t, err)

	assert.Equal(t, []string{"/repos/so/sr/pulls/5/reviews/9"}, deleted)
	assert.Equal(t, 1, threads)
	assert.Equal(t, 1, submits)
	assert.Equal(t, []string{"https://sub/review"}, res.SubmoduleReviewURLs)
	assert.Empty(t, app.orphans)
}

func TestSubmitReview_MixedSubmoduleModesFailPartWay_RetryPostsEachOnceAndFoldsOnce(t *testing.T) {
	posted := map[string]int{}
	var parentBody string
	failBody, parentFails := "second", true
	app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`[]`)) // nothing went through despite the 502
			return
		}
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		switch {
		case r.URL.Path == "/repos/so2/sr2/pulls/6/comments" && body["body"] == failBody:
			failBody = ""
			w.WriteHeader(http.StatusBadGateway)
			return
		case r.URL.Path == "/repos/o/r/pulls/1/reviews" && parentFails:
			parentFails = false
			w.WriteHeader(http.StatusBadGateway)
			return
		case r.URL.Path == "/repos/o/r/pulls/1/reviews":
			parentBody = fmt.Sprint(body["body"])
		}
		posted[fmt.Sprint(r.URL.Path, " ", body["event"], " ", body["body"])]++
		_, _ = w.Write([]byte(`{"id":7,"html_url":"https://x"}`))
	})
	addSub(app, "lib2", "so2", "sr2", 6)
	addSub(app, "lib3", "so3", "sr3", 7)
	app.pendingComments = []model.DraftCommentDTO{
		{Submodule: "lib", Bump: "..new", Path: "lib/a.txt", Line: 2, Side: "RIGHT", Body: "reviewed"},
		{Submodule: "lib", Bump: "..new", Path: "lib/b.txt", Line: 1, Side: "RIGHT", Body: "not in lib PR"},
		{Submodule: "lib2", Bump: "..new", Path: "lib2/a.txt", Line: 2, Side: "RIGHT", Body: "first"},
		{Submodule: "lib2", Bump: "..new", Path: "lib2/a.txt", Line: 3, Side: "RIGHT", Body: "second"},
		{Submodule: "lib3", Path: "lib3/a.txt", Line: 2, Side: "RIGHT", Body: "folded by choice"},
		{Path: "a.go", Line: 1, Side: "RIGHT", Body: "parent"},
	}
	req := model.ReviewSubmitRequest{Verdict: "COMMENT", Body: "summary", Submodules: []model.SubmoduleReviewChoice{
		{Path: "lib", Mode: "review", Verdict: "APPROVE"},
		{Path: "lib2", Mode: "comments"},
		{Path: "lib3", Mode: "fold"},
	}}

	_, err := app.SubmitReview(shown(t, app, req))
	require.ErrorContains(t, err, "so2/sr2#6")
	_, err = app.SubmitReview(shown(t, app, req))
	require.Error(t, err, "the parent review fails after the last submodule comment went through")
	res, err := app.SubmitReview(shown(t, app, req))
	require.NoError(t, err)

	assert.Equal(t, map[string]int{
		"/repos/so/sr/pulls/5/reviews APPROVE ":            1,
		"/repos/so2/sr2/pulls/6/comments <nil> first":      1,
		"/repos/so2/sr2/pulls/6/comments <nil> second":     1,
		"/repos/o/r/pulls/1/reviews COMMENT " + parentBody: 1,
	}, posted)
	assert.Equal(t, 1, strings.Count(parentBody, "- `lib/b.txt` (submodule lib) L1 — not in lib PR"))
	assert.Equal(t, 1, strings.Count(parentBody, "- `lib3/a.txt` (submodule lib3) L2 — folded by choice"))
	assert.Equal(t, 1, strings.Count(parentBody, "**Comments on submodule files**"))
	assert.Equal(t, model.ReviewSubmitResult{
		ReviewID: 7, HTMLURL: "https://x", FoldedComments: 2,
		SubmoduleReviewURLs: []string{"https://x"}, SubmoduleCommentURLs: []string{"https://x", "https://x"},
	}, res)
	assert.Empty(t, app.pendingComments)
}

func TestSubmitReview_SubmoduleFileDraftWithVerdict_SubmitsVerdictAndBodyAfterFileThread(t *testing.T) {
	for _, verdict := range []string{"APPROVE", "REQUEST_CHANGES"} {
		t.Run(verdict, func(t *testing.T) {
			var created, submitted map[string]any
			var thread map[string]any
			fakeGraphQL(t, func(w http.ResponseWriter, r *http.Request) {
				require.NoError(t, json.NewDecoder(r.Body).Decode(&thread))
				_, _ = w.Write([]byte(`{"data":{}}`))
			})
			app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					_, _ = w.Write([]byte(`[]`)) // no pending review before the create
					return
				}
				var body map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				switch r.URL.Path {
				case "/repos/so/sr/pulls/5/reviews":
					created = body
					_, _ = w.Write([]byte(`{"id":10,"node_id":"PRR_10"}`))
				case "/repos/so/sr/pulls/5/reviews/10/events":
					submitted = body
					_, _ = w.Write([]byte(`{"id":10,"html_url":"https://sub/review"}`))
				default:
					w.WriteHeader(http.StatusTeapot)
				}
			})
			app.pendingComments = []model.DraftCommentDTO{{Submodule: "lib", Bump: "..new", Path: "lib/a.txt", Body: "whole file"}}

			res, err := app.SubmitReview(shown(t, app, model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{
				{Path: "lib", Mode: "review", Verdict: verdict, Body: "Looks fine."},
			}}))
			require.NoError(t, err)

			assert.NotContains(t, created, "event")
			assert.Equal(t, map[string]any{"reviewID": "PRR_10", "path": "a.txt", "body": "whole file"}, thread["variables"])
			assert.Equal(t, map[string]any{"event": verdict, "body": "Looks fine."}, submitted)
			assert.Equal(t, []string{"https://sub/review"}, res.SubmoduleReviewURLs)
		})
	}
}

func TestSubmitReview_SubmoduleWithoutChoice_FoldsItsDrafts(t *testing.T) {
	got := map[string][]map[string]any{}
	app := subApp(t, recordReviews(t, got))
	app.pendingComments = subDrafts()

	res, err := app.SubmitReview(shown(t, app, model.ReviewSubmitRequest{Verdict: "COMMENT"}))
	require.NoError(t, err)

	assert.Empty(t, got["/repos/so/sr/pulls/5/reviews"])
	assert.Contains(t, got["/repos/o/r/pulls/1/reviews"][0]["body"], "- `lib/a.txt` (submodule lib) L2 — in PR diff")
	assert.Equal(t, 2, res.FoldedComments)
}

func TestLoadSubmodule_PRLookupFails_MarksPRUnknownAndExplicitRetryBypassesBackoff(t *testing.T) {
	lookupFails := true
	app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/contents/lib":
			_, _ = w.Write([]byte(`{"type":"submodule","html_url":"https://github.com/so/sr/tree/new"}`))
		case "/repos/so/sr/compare/old...new":
			_, _ = w.Write([]byte(`{"status":"ahead","files":[{"filename":"a.txt","status":"modified","patch":"@@ -1 +1 @@\n-x\n+y"}]}`))
		case "/repos/so/sr/commits/new/pulls":
			if lookupFails {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			_, _ = w.Write([]byte(`[{"number":5,"state":"open","head":{"sha":"new"},"user":{"login":"me"}}]`))
		case "/repos/so/sr/pulls/5/files":
			_, _ = w.Write([]byte(`[{"filename":"a.txt","patch":"@@ -1 +1 @@\n-x\n+y"}]`))
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	})
	s := app.loadSubmodule(nil, "lib", "old", "new")
	app.prSubmodules = map[string]*prSubmodule{"lib": s}
	pointer := &github.CommitFile{Filename: new("lib"), Patch: new("@@ -1 +1 @@\n-Subproject commit old\n+Subproject commit new")}
	assert.True(t, app.prFileDTO(pointer).SubmodulePRUnknown)
	app.pendingComments = []model.DraftCommentDTO{{Submodule: "lib", Bump: "old..new", Path: "lib/a.txt", Line: 1, Side: "RIGHT", Body: "x"}}

	got, err := app.SubmoduleDrafts(false)
	require.NoError(t, err)
	assert.Equal(t, []model.SubmoduleDraftsDTO{{Path: "lib", Folded: 1, PRUnknown: true}}, got)

	lookupFails = false
	got, err = app.SubmoduleDrafts(false)
	require.NoError(t, err)
	assert.True(t, got[0].PRUnknown, "an automatic reload waits out the backoff")
	fakeGraphQL(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"repository":{"pullRequest":{"files":{"nodes":[{"path":"a.txt","viewerViewedState":"VIEWED"}],"pageInfo":{"hasNextPage":false}}}}}}`))
	})
	app.viewedFiles = map[fileID]bool{}
	got, err = app.SubmoduleDrafts(true)
	require.NoError(t, err)
	assert.Equal(t, []model.SubmoduleDraftsDTO{{Path: "lib", PR: "so/sr#5", ToPR: 1, OwnPR: true}}, got)
	ptr := app.prFileDTO(pointer)
	assert.Equal(t, "so/sr#5", ptr.SubmodulePR)
	assert.False(t, ptr.SubmodulePRUnknown)
	assert.True(t, app.viewedFiles[fileID{"lib", "lib/a.txt", false}], "the found PR's viewed marks are loaded")
}

func TestSubmitReview_BumpGoneSinceDraft_FoldsDraftInsteadOfParentComment(t *testing.T) {
	got := map[string][]map[string]any{}
	app := subApp(t, recordReviews(t, got))
	_, err := app.AddDraftComment(model.DraftCommentDTO{Submodule: "lib", Bump: "..new", Path: "lib/a.txt", Line: 2, Side: "RIGHT", Body: "in PR diff"})
	require.NoError(t, err)
	assert.Equal(t, "lib", app.pendingComments[0].Submodule)

	app.prSubmodules, app.prSubmoduleOf = map[string]*prSubmodule{}, map[*github.CommitFile]*prSubmodule{} // force-push dropped the bump
	res, err := app.SubmitReview(shown(t, app, model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{}}))
	require.NoError(t, err)

	parent := got["/repos/o/r/pulls/1/reviews"][0]
	assert.Empty(t, parent["comments"])
	assert.Contains(t, parent["body"], "- `lib/a.txt` (submodule lib) L2 — in PR diff")
	assert.Equal(t, 1, res.FoldedComments)
}

func TestSubmitReview_EmptyParentReview_ReturnsValidationErrorBeforePosting(t *testing.T) {
	routed := []model.SubmoduleReviewChoice{{Path: "lib", Mode: "review", Verdict: "COMMENT"}}
	cases := map[string]struct {
		verdict string
		choices []model.SubmoduleReviewChoice
		drafts  []model.DraftCommentDTO
	}{
		"request changes, all routed": {"REQUEST_CHANGES", routed, subDrafts()[:1]},
		"comment, nothing at all":     {"COMMENT", nil, nil},
	}
	for name, c := range cases {
		got := map[string][]map[string]any{}
		app := subApp(t, recordReviews(t, got))
		app.pendingComments = c.drafts

		_, err := app.SubmitReview(shown(t, app, model.ReviewSubmitRequest{Verdict: c.verdict, Submodules: c.choices}))
		require.ErrorContains(t, err, "validation:", name)
		assert.Empty(t, got, name)
	}
}

func TestSubmitReview_VerdictOnOwnSubmodulePR_ReturnsValidationError(t *testing.T) {
	got := map[string][]map[string]any{}
	app := subApp(t, recordReviews(t, got))
	app.prSubmodules["lib"].pr.User = &github.User{Login: new("me")}
	app.pendingComments = subDrafts()

	_, err := app.SubmitReview(shown(t, app, model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "review", Verdict: "APPROVE"}}}))
	require.ErrorContains(t, err, "validation: so/sr#5 is your own pull request")
	assert.Empty(t, got)
	drafts, err := app.SubmoduleDrafts(false)
	require.NoError(t, err)
	assert.True(t, drafts[0].OwnPR)
}

func TestSubmitReview_UserPendingReviewOnSubmodulePR_NamesItAndDeletesNothing(t *testing.T) {
	var deleted bool
	app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
		deleted = deleted || r.Method == http.MethodDelete
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"Unprocessable Entity","errors":["User can only have one pending review per pull request"]}`))
	})
	app.pendingComments = subDrafts()

	_, err := app.SubmitReview(shown(t, app, model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "review", Verdict: "COMMENT"}}}))
	require.EqualError(t, err, "github: you already have a pending review on so/sr#5; submit or discard it on GitHub")
	assert.False(t, deleted)
}

func TestDiscardPendingReview_RememberedPendingReviews_DeletesOnlyThoseStillPending(t *testing.T) {
	for name, tc := range map[string]struct {
		status  int
		state   string
		deleted int
		wantErr bool
	}{
		"still pending":   {http.StatusOK, "PENDING", 3, false},
		"submitted since": {http.StatusOK, "COMMENTED", 0, false},
		"gone":            {http.StatusNotFound, "", 0, false},
		"lookup fails":    {http.StatusBadGateway, "", 0, true},
	} {
		t.Run(name, func(t *testing.T) {
			var deleted []string
			app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodDelete {
					deleted = append(deleted, r.URL.Path)
					return
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"state":"` + tc.state + `"}`))
			})
			app.pendingComments = subDrafts()
			app.pendingReviewID = 8
			app.orphans = []orphanReview{{"so", "sr", 5, 9}}
			app.uncertain = map[int64]*uncertainError{1: {pending: &orphanReview{"so", "sr", 5, 10}}}
			app.subReviewURLs, app.subCommentURLs = []string{"r"}, []string{"c"}

			err := app.DiscardPendingReview()

			assert.Len(t, deleted, tc.deleted)
			if tc.wantErr {
				require.ErrorContains(t, err, "github: check pending review")
				assert.ElementsMatch(t, []orphanReview{{"o", "r", 1, 8}, {"so", "sr", 5, 9}, {"so", "sr", 5, 10}}, app.rememberedPending(), "kept for the next discard or submit")
				assert.Equal(t, subDrafts(), app.pendingComments)
				return
			}
			require.NoError(t, err)
			assert.Empty(t, app.rememberedPending())
			assert.Empty(t, app.subReviewURLs)
			assert.Empty(t, app.subCommentURLs)
			assert.Empty(t, app.pendingComments)
		})
	}
}

func TestCheckSubmoduleChoices_ReviewForSubmoduleWithoutPR_ReturnsValidationError(t *testing.T) {
	for name, path := range map[string]string{"bump without an open PR": "lib", "bump gone from the PR": "gone"} {
		for _, verdict := range reviewVerdicts {
			app := subApp(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
			app.prSubmodules["lib"].pr = nil
			app.pendingComments = []model.DraftCommentDTO{{Submodule: "gone", Path: "gone/a.txt", Body: "x"}}

			err := app.checkSubmoduleChoices([]model.SubmoduleReviewChoice{{Path: path, Mode: "review", Verdict: verdict}})

			require.ErrorContains(t, err, "validation: submodule "+path+" has no open pull request", name+" "+verdict)
		}
	}
}

func TestSubmoduleDraftsAndSetFileViewed_NoPRFoundOverAMinuteAgo_LookUpAgainAndFindTheNewPR(t *testing.T) {
	fakeGraphQL(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"data":{}}`)) })
	for _, call := range []string{"SubmoduleDrafts", "SetFileViewed"} {
		t.Run(call, func(t *testing.T) {
			lookups := 0
			app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/repos/so/sr/commits/new/pulls":
					lookups++
					_, _ = w.Write([]byte(`[{"number":5,"node_id":"PR_5","state":"open","head":{"sha":"new"}}]`))
				case "/repos/so/sr/pulls/5/files":
					_, _ = w.Write([]byte(`[{"filename":"a.txt","patch":"@@ -1 +1 @@\n-x\n+y"}]`))
				default:
					w.WriteHeader(http.StatusTeapot)
				}
			})
			s := app.prSubmodules["lib"]
			s.pr, s.prFiles, s.checkedAt = nil, nil, time.Now()
			app.pendingComments = subDrafts()
			look := func() bool {
				t.Helper()
				if call == "SetFileViewed" {
					rerouted, err := app.SetFileViewed("lib", "lib/a.txt", false, true)
					require.NoError(t, err)
					return rerouted
				}
				got, err := app.SubmoduleDrafts(false)
				require.NoError(t, err)
				return got[0].PR != ""
			}

			assert.False(t, look(), "a fresh negative lookup is trusted")
			assert.Equal(t, 0, lookups)
			s.checkedAt = time.Now().Add(-2 * time.Minute)
			assert.True(t, look())
			assert.Equal(t, 1, lookups)
			assert.Equal(t, "so/sr#5", s.prName())
		})
	}
}

func TestSubmitReview_SubmoduleReviewAnswers5xxButWasPosted_RecordsItWithoutReposting(t *testing.T) {
	posts := 0
	now := time.Now().UTC().Format(time.RFC3339)
	app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/repos/so/sr/pulls/5/reviews":
			posts++
			w.WriteHeader(http.StatusBadGateway)
		case r.Method == http.MethodGet && r.URL.Path == "/repos/so/sr/pulls/5/reviews":
			_, _ = w.Write([]byte(`[
				{"id":6,"state":"COMMENTED","user":{"login":"me"},"commit_id":"new","body":"","submitted_at":"2020-01-01T00:00:00Z"},
				{"id":7,"state":"COMMENTED","user":{"login":"me"},"commit_id":"new","body":"","submitted_at":"` + now + `","html_url":"https://sub/review"}]`))
		case r.URL.Path == "/repos/so/sr/pulls/5/reviews/7/comments":
			_, _ = w.Write([]byte(`[{"path":"a.txt","line":2,"side":"RIGHT","body":"in PR diff"}]`))
		case r.URL.Path == "/repos/o/r/pulls/1/reviews":
			_, _ = w.Write([]byte(`{"id":9,"html_url":"https://parent/review"}`))
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	})
	app.pendingComments = subDrafts()

	res, err := app.SubmitReview(shown(t, app, model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "review", Verdict: "COMMENT"}}}))
	require.NoError(t, err)
	assert.Equal(t, 1, posts)
	assert.Equal(t, []string{"https://sub/review"}, res.SubmoduleReviewURLs, "the older look-alike review is not it")
	assert.Equal(t, 1, res.FoldedComments)
}

func TestSubmitReview_SubmoduleCommentConnectionDrops_AdoptsTheCommentGitHubCreated(t *testing.T) {
	posts := 0
	now := time.Now().UTC().Format(time.RFC3339)
	app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/repos/so/sr/pulls/5/comments":
			posts++
			panic(http.ErrAbortHandler) // no response at all
		case r.Method == http.MethodGet && r.URL.Path == "/repos/so/sr/pulls/5/comments":
			assert.NotEmpty(t, r.URL.Query().Get("since"))
			_, _ = w.Write([]byte(`[
				{"user":{"login":"other"},"path":"a.txt","body":"in PR diff","commit_id":"new","line":2,"side":"RIGHT","created_at":"` + now + `","html_url":"https://not/mine"},
				{"user":{"login":"me"},"path":"a.txt","body":"in PR diff","commit_id":"new","line":2,"side":"RIGHT","created_at":"` + now + `","html_url":"https://sub/comment"}]`))
		case r.URL.Path == "/repos/o/r/pulls/1/reviews":
			_, _ = w.Write([]byte(`{"id":9}`))
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	})
	app.pendingComments = subDrafts()

	res, err := app.SubmitReview(shown(t, app, model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "comments"}}}))
	require.NoError(t, err)
	assert.Equal(t, 1, posts)
	assert.Equal(t, []string{"https://sub/comment"}, res.SubmoduleCommentURLs)
}

func TestPostImmediateComment_FoldedCommentAnswers5xxButWasPosted_ReturnsIt(t *testing.T) {
	want := "**Comments on submodule files**\n\n- `lib/b.txt` (submodule lib) L1 — hm"
	app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		body, err := json.Marshal(want)
		require.NoError(t, err)
		_, _ = w.Write([]byte(`[{"id":3,"user":{"login":"me"},"created_at":"` + time.Now().UTC().Format(time.RFC3339) + `","body":` + string(body) + `}]`))
	})

	got, err := app.PostImmediateComment(model.DraftCommentDTO{Submodule: "lib", Bump: "..new", Path: "lib/b.txt", Line: 1, Side: "RIGHT", Body: "hm"})
	require.NoError(t, err)
	assert.Equal(t, int64(3), got.ID)
}

func TestSubmitReview_SubmoduleCommentsWithUserPendingReview_NamesIt(t *testing.T) {
	app := subApp(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"Unprocessable Entity","errors":["User can only have one pending review per pull request"]}`))
	})
	app.pendingComments = subDrafts()

	_, err := app.SubmitReview(shown(t, app, model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "comments"}}}))
	require.EqualError(t, err, "github: you already have a pending review on so/sr#5; submit or discard it on GitHub")
}

// addedSubApp is an App whose PR turns tracked directory lib/ into a submodule (so/sr at "new"):
// the parent PR removes lib/a.txt, and the submodule's tree holds a.txt and b.txt.
func addedSubApp(t *testing.T) *App {
	t.Helper()
	app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/pulls/1/files":
			_, _ = w.Write([]byte(`[
				{"filename":"lib","status":"added","patch":"@@ -0,0 +1 @@\n+Subproject commit new"},
				{"filename":"lib/a.txt","status":"removed","deletions":1,"changes":1,"patch":"@@ -1 +0,0 @@\n-x"}]`))
		case "/repos/o/r/contents/lib":
			_, _ = w.Write([]byte(`{"type":"submodule","html_url":"https://github.com/so/sr/tree/new"}`))
		case "/repos/so/sr/git/trees/new":
			_, _ = w.Write([]byte(`{"tree":[{"path":"a.txt","type":"blob","sha":"a1"},{"path":"b.txt","type":"blob","sha":"b1"}]}`))
		case "/repos/so/sr/git/blobs/b1":
			_, _ = w.Write([]byte("one\ntwo\n"))
		case "/repos/so/sr/git/blobs/a1":
			_, _ = w.Write([]byte("1\n2\n3\n"))
		case "/repos/so/sr/commits/new/pulls":
			_, _ = w.Write([]byte(`[]`))
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	})
	app.prFilesCache, app.prSubmodules, app.prSubmoduleOf = nil, nil, nil
	return app
}

func TestGetPRFiles_AddedSubmoduleOverTrackedDir_ListsBothFilesAtSharedPathWithOwnDiffsAndDrafts(t *testing.T) {
	app := addedSubApp(t)

	files, err := app.GetPRFiles()
	require.NoError(t, err)
	var names []string
	for _, f := range files {
		names = append(names, f.Filename+" "+f.Status+" "+f.Submodule)
	}
	assert.Equal(t, []string{"lib submodule lib", "lib/a.txt added lib", "lib/b.txt added lib", "lib/a.txt removed "}, names)
	assert.Empty(t, files[0].SubmoduleUnavailable)
	assert.True(t, files[2].ViewedLocal)
	assert.True(t, files[2].LinesUnknown)
	assert.False(t, files[3].LinesUnknown)

	sub, err := app.GetFileDiff("lib", "lib/a.txt", false)
	require.NoError(t, err)
	assert.Equal(t, []int{3, 0}, []int{sub.TotalAdditions, sub.TotalDeletions}, "the submodule's a.txt, its content loaded")
	parent, err := app.GetFileDiff("", "lib/a.txt", false)
	require.NoError(t, err)
	assert.Equal(t, []int{0, 1}, []int{parent.TotalAdditions, parent.TotalDeletions}, "the parent's removed lib/a.txt")
	d, err := app.GetFileDiff("lib", "lib/b.txt", false)
	require.NoError(t, err)
	assert.Equal(t, 2, d.TotalAdditions)
	files, err = app.GetPRFiles()
	require.NoError(t, err)
	assert.False(t, files[2].LinesUnknown)
	assert.Equal(t, 2, files[2].Additions)

	_, err = app.AddDraftComment(model.DraftCommentDTO{Path: "lib/a.txt", Line: 1, Side: "LEFT", Body: "parent file"})
	require.NoError(t, err)
	_, err = app.AddDraftComment(model.DraftCommentDTO{Submodule: "lib", Bump: "..new", Path: "lib/a.txt", Line: 1, Side: "RIGHT", Body: "submodule file"})
	require.NoError(t, err)
	assert.Equal(t, []string{"", "lib"}, []string{app.pendingComments[0].Submodule, app.pendingComments[1].Submodule})
}

func TestGetPRFiles_TextFileWhosePatchReadsAsPointerChange_StaysARegularFile(t *testing.T) {
	look := "@@ -0,0 +1 @@\n+Subproject commit 1234567890"
	app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/pulls/1/files":
			_, _ = w.Write([]byte(`[
				{"filename":"lib","status":"modified","patch":"@@ -1 +1 @@\n-Subproject commit old\n+Subproject commit new"},
				{"filename":"x.txt","status":"added","patch":"` + strings.ReplaceAll(look, "\n", `\n`) + `"}]`))
		case "/repos/o/r/contents/lib":
			_, _ = w.Write([]byte(`{"type":"submodule","html_url":"https://github.com/so/sr/tree/new"}`))
		case "/repos/o/r/contents/x.txt", "/repos/so/sr/contents/y.txt":
			_, _ = w.Write([]byte(`{"type":"file"}`))
		case "/repos/so/sr/compare/old...new":
			_, _ = w.Write([]byte(`{"status":"ahead","files":[{"filename":"y.txt","status":"added","patch":"` + strings.ReplaceAll(look, "\n", `\n`) + `"}]}`))
		case "/repos/so/sr/commits/new/pulls":
			_, _ = w.Write([]byte(`[]`))
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	})
	app.prFilesCache, app.prSubmodules, app.prSubmoduleOf = nil, nil, nil

	files, err := app.GetPRFiles()
	require.NoError(t, err)
	var rows []string
	for _, f := range files {
		rows = append(rows, f.Filename+" "+f.Status+" "+f.Repo+" "+f.SubmoduleUnavailable)
	}
	assert.Equal(t, []string{"lib submodule  ", "lib/y.txt added lib ", "x.txt added  "}, rows)
	assert.Len(t, app.prSubmodules, 1, "neither x.txt nor lib/y.txt is a submodule")

	_, err = app.GetFileDiff("", "x.txt", false)
	require.NoError(t, err)
	_, err = app.AddDraftComment(model.DraftCommentDTO{Path: "x.txt", Line: 1, Side: "RIGHT", Body: "parent"})
	require.NoError(t, err)
	_, err = app.AddDraftComment(model.DraftCommentDTO{Submodule: "lib", Path: "lib/y.txt", Line: 1, Side: "RIGHT", Body: "nested"})
	require.NoError(t, err)
	assert.False(t, app.restoredOnPointer(model.DraftCommentDTO{Path: "x.txt", Line: 1, Side: "RIGHT"}))
}

func TestAddDraftComment_SubmoduleNotHoldingThatFile_ReturnsValidationError(t *testing.T) {
	app := addedSubApp(t)
	for _, d := range []model.DraftCommentDTO{
		{Submodule: "nope", Path: "lib/a.txt", Line: 1, Side: "RIGHT", Body: "x"},
		{Submodule: "lib", Bump: "..new", Path: "lib/zzz.txt", Line: 1, Side: "RIGHT", Body: "x"},
	} {
		_, err := app.AddDraftComment(d)
		require.ErrorContains(t, err, "validation:")
		_, err = app.PostImmediateComment(d)
		require.ErrorContains(t, err, "validation:")
	}
	assert.Empty(t, app.pendingComments)
}

func TestAddDraftComment_SubmoduleRangeChangedSinceFormOpened_ReturnsConflict(t *testing.T) {
	app := subApp(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	app.prSubmodules["lib"].rng = "aaa..bbb"
	stale := model.DraftCommentDTO{Submodule: "lib", Range: "aaa..ccc", Path: "lib/a.txt", Line: 1, Side: "RIGHT", Body: "x"}
	_, err := app.AddDraftComment(stale)
	require.ErrorContains(t, err, "conflict:")
	_, err = app.PostImmediateComment(stale)
	require.ErrorContains(t, err, "conflict:")
	assert.Empty(t, app.pendingComments)

	stale.Range = "aaa..bbb"
	_, err = app.AddDraftComment(stale)
	require.NoError(t, err)
}

func TestAddDraftComment_SubmoduleBumpNoLongerChangesFileAndRangeStale_ReturnsConflict(t *testing.T) {
	app := subApp(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	app.prSubmodules["lib"].rng = "aaa..bbb"
	for name, path := range map[string]string{"file gone": "lib/gone.txt", "submodule gone": ""} {
		d := model.DraftCommentDTO{Submodule: "lib", Range: "aaa..ccc", Path: "lib/gone.txt", Line: 1, Side: "RIGHT", Body: "x"}
		if path == "" {
			d.Submodule, d.Path = "gone", "gone/a.txt"
		}
		_, err := app.AddDraftComment(d)
		require.ErrorContains(t, err, "conflict:", name)
	}
}

func TestSubmitReview_ParentHeadMovedBumpUnchanged_SendsDraftToSubmodulePR(t *testing.T) {
	got := map[string][]map[string]any{}
	app := subApp(t, recordReviews(t, got))
	app.pendingComments, app.pendingHeadSHA = subDrafts()[:1], "sha"
	app.prCache.HeadSHA = "moved"

	drafts, err := app.SubmoduleDrafts(false)
	require.NoError(t, err)
	assert.Equal(t, []model.SubmoduleDraftsDTO{{Path: "lib", PR: "so/sr#5", ToPR: 1}}, drafts)
	_, err = app.SubmitReview(shown(t, app, model.ReviewSubmitRequest{Verdict: "COMMENT", Body: "x", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "review", Verdict: "COMMENT"}}}))

	require.NoError(t, err)
	require.Len(t, got["/repos/so/sr/pulls/5/reviews"], 1)
	assert.NotContains(t, got["/repos/o/r/pulls/1/reviews"][0]["body"], "in PR diff")
}

func TestSetFileViewed_SubmodulePRMarkDismissedOnGitHub_ClearsCachedMarkAndReroutes(t *testing.T) {
	fakeGraphQL(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"repository":{"pullRequest":{"files":{"nodes":[],"pageInfo":{"hasNextPage":false}}}}}}`))
	})
	app := subAppRaw(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/so/sr/pulls/5":
			_, _ = w.Write([]byte(`{"number":5,"state":"open","head":{"sha":"new"},"base":{"sha":"retargeted"}}`))
		case "/repos/so/sr/pulls/5/files":
			_, _ = w.Write([]byte(`[{"filename":"a.txt","patch":"@@ -1 +1 @@\n-x\n+y"}]`))
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	})
	s := app.prSubmodules["lib"]
	app.viewedFiles = map[fileID]bool{{"lib", "lib/a.txt", false}: true}
	s.checkedAt = time.Now().Add(-2 * time.Minute)

	rerouted, err := app.SetFileViewed("lib", "lib/b.txt", false, true)

	require.NoError(t, err)
	assert.True(t, rerouted, "a.txt's row has to drop its mark")
	assert.False(t, app.prFileDTO(app.bumpFile(s, "lib/a.txt", false)).Viewed)
}

func TestSetFileViewed_WholeTreeFile_MarkSurvivesLoadingItsContent(t *testing.T) {
	app := addedSubApp(t)
	_, err := app.GetPRFiles()
	require.NoError(t, err)

	require.NoError(t, viewedErr(app.SetFileViewed("lib", "lib/b.txt", false, true)))
	_, err = app.GetFileDiff("lib", "lib/b.txt", false)
	require.NoError(t, err)
	app.loadSubViewed()

	files, err := app.GetPRFiles()
	require.NoError(t, err)
	assert.True(t, files[2].Viewed)
}

func TestGetPRFiles_SubmoduleFilesFailedToLoad_RetriedOnlyOnceBackoffPassed(t *testing.T) {
	compareFails, compares := true, 0
	app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/pulls/1/files":
			_, _ = w.Write([]byte(`[{"filename":"lib","status":"modified","patch":"@@ -1 +1 @@\n-Subproject commit old\n+Subproject commit new"}]`))
		case "/repos/o/r/contents/lib":
			_, _ = w.Write([]byte(`{"type":"submodule","html_url":"https://github.com/so/sr/tree/new"}`))
		case "/repos/so/sr/compare/old...new":
			compares++
			if compareFails {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			_, _ = w.Write([]byte(`{"status":"ahead","files":[{"filename":"a.txt","status":"modified","patch":"@@ -1 +1 @@\n-x\n+y"}]}`))
		case "/repos/so/sr/commits/new/pulls":
			_, _ = w.Write([]byte(`[]`))
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	})
	app.prFilesCache, app.prSubmodules, app.prSubmoduleOf = nil, nil, nil

	files, err := app.GetPRFiles()
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, "failed to load", files[0].SubmoduleUnavailable)

	compareFails = false
	files, err = app.GetPRFiles()
	require.NoError(t, err)
	require.Len(t, files, 1, "not retried within the backoff")
	assert.Equal(t, 1, compares)

	app.prSubmodules["lib"].retryAt = time.Time{}
	files, err = app.GetPRFiles()
	require.NoError(t, err)
	require.Len(t, files, 2)
	assert.Empty(t, files[0].SubmoduleUnavailable)
	assert.Equal(t, "lib/a.txt", files[1].Filename)
}

// binApp is an App whose fake GitHub answers compares and trees of submodule so/sr from bump, keyed by URL path.
func binApp(t *testing.T, bump map[string]string) *App {
	t.Helper()
	return subApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/o/r/contents/lib":
			_, _ = w.Write([]byte(`{"type":"submodule","html_url":"https://github.com/so/sr/tree/new"}`))
		case strings.HasSuffix(r.URL.Path, "/pulls"):
			_, _ = w.Write([]byte(`[]`))
		case bump[r.URL.Path] != "":
			_, _ = w.Write([]byte(bump[r.URL.Path]))
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	})
}

// markBin marks lib/c.bin of the bump from..to viewed, and reports whether the mark holds on the bump from2..to2.
func markBin(t *testing.T, app *App, from, to, from2, to2 string) bool {
	t.Helper()
	s := app.loadSubmodule(nil, "lib", from, to)
	require.False(t, s.failed, s.Unavailable)
	app.prSubmodules["lib"] = s
	for _, f := range s.Files {
		app.prSubmoduleOf[f] = s
	}
	require.NoError(t, viewedErr(app.SetFileViewed("lib", "lib/c.bin", false, true)))
	require.True(t, app.subViewedNow(s, app.bumpFile(s, "lib/c.bin", false)))
	s2 := app.loadSubmodule(nil, "lib", from2, to2)
	require.False(t, s2.failed, s2.Unavailable)
	return app.subViewedNow(s2, app.bumpFile(s2, "lib/c.bin", false))
}

const binFile = `{"status":"ahead","files":[{"filename":"c.bin","status":"modified","changes":1,"sha":"binO"}]}`

func TestSetFileViewed_BackwardsBinaryBumpWhoseNewSideChangesAgain_MarkClears(t *testing.T) {
	app := binApp(t, map[string]string{
		"/repos/so/sr/compare/O...N1": `{"status":"behind"}`,
		"/repos/so/sr/compare/N1...O": binFile,
		"/repos/so/sr/git/trees/N1":   `{"tree":[{"path":"c.bin","type":"blob","sha":"binN1"}]}`,
		"/repos/so/sr/compare/O...N2": `{"status":"behind"}`,
		"/repos/so/sr/compare/N2...O": binFile,
		"/repos/so/sr/git/trees/N2":   `{"tree":[{"path":"c.bin","type":"blob","sha":"binN2"}]}`,
	})

	assert.False(t, markBin(t, app, "O", "N1", "O", "N2"))
	f := app.bumpFile(app.prSubmodules["lib"], "lib/c.bin", false)
	assert.Equal(t, "binN1", f.GetSHA(), "after inversion the SHA is the new side's")
	assert.Equal(t, "binO", app.prSubmodules["lib"].OldSHAs["lib/c.bin"])
}

func TestSetFileViewed_BinaryBumpWhoseOldCommitAdvancesUnrelated_MarkKept(t *testing.T) {
	app := binApp(t, map[string]string{
		"/repos/so/sr/compare/O1...N": `{"status":"ahead","files":[{"filename":"c.bin","status":"modified","changes":1,"sha":"binN"}]}`,
		"/repos/so/sr/git/trees/O1":   `{"tree":[{"path":"c.bin","type":"blob","sha":"binO"}]}`,
		"/repos/so/sr/compare/O2...N": `{"status":"ahead","files":[{"filename":"c.bin","status":"modified","changes":1,"sha":"binN"}]}`,
		"/repos/so/sr/git/trees/O2":   `{"tree":[{"path":"c.bin","type":"blob","sha":"binO"},{"path":"other.txt","type":"blob","sha":"x"}]}`,
	})

	assert.True(t, markBin(t, app, "O1", "N", "O2", "N"))
}

func TestSetFileViewed_BinaryBumpWhoseOldBlobChanges_MarkClears(t *testing.T) {
	app := binApp(t, map[string]string{
		"/repos/so/sr/compare/O1...N": `{"status":"ahead","files":[{"filename":"c.bin","status":"modified","changes":1,"sha":"binN"}]}`,
		"/repos/so/sr/git/trees/O1":   `{"tree":[{"path":"c.bin","type":"blob","sha":"binO1"}]}`,
		"/repos/so/sr/compare/O2...N": `{"status":"ahead","files":[{"filename":"c.bin","status":"modified","changes":1,"sha":"binN"}]}`,
		"/repos/so/sr/git/trees/O2":   `{"tree":[{"path":"c.bin","type":"blob","sha":"binO2"}]}`,
	})

	assert.False(t, markBin(t, app, "O1", "N", "O2", "N"))
}

func TestLoadSubmodule_RemovedSubmodule_ResolvesItAtMergeBaseAndListsOldTree(t *testing.T) {
	app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/o/r/compare/main...sha":
			_, _ = w.Write([]byte(`{"merge_base_commit":{"sha":"mb"},"base_commit":{"sha":"tip"}}`))
		case r.URL.Path == "/repos/o/r/contents/lib" && r.URL.Query().Get("ref") == "mb":
			_, _ = w.Write([]byte(`{"type":"submodule","html_url":"https://github.com/so/sr/tree/old"}`))
		case r.URL.Path == "/repos/so/sr/git/trees/old":
			_, _ = w.Write([]byte(`{"tree":[{"path":"a.txt","type":"blob","sha":"a1"}]}`))
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	})

	app.prCache.BaseBranch = "main"

	s := app.loadSubmodule(nil, "lib", "old", "")
	require.Len(t, s.Files, 1)
	assert.Equal(t, "removed", s.Files[0].GetStatus())
	assert.Equal(t, "old", s.TreeRef)
	assert.Nil(t, s.pr)
}

func TestSubmitReview_SubmoduleReviewSubmitAnswers5xxButWentThrough_KeepsIt(t *testing.T) {
	var deleted bool
	fakeGraphQL(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"data":{}}`)) })
	app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete:
			deleted = true
		case r.URL.Path == "/repos/so/sr/pulls/5/reviews" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && r.URL.Path == "/repos/so/sr/pulls/5/reviews":
			_, _ = w.Write([]byte(`{"id":10,"node_id":"PRR_10"}`))
		case r.URL.Path == "/repos/so/sr/pulls/5/reviews/10/events":
			w.WriteHeader(http.StatusGatewayTimeout)
		case r.URL.Path == "/repos/so/sr/pulls/5/reviews/10":
			_, _ = w.Write([]byte(`{"id":10,"state":"COMMENTED","html_url":"https://sub/review"}`))
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	})
	app.pendingComments = []model.DraftCommentDTO{{Submodule: "lib", Bump: "..new", Path: "lib/a.txt", Body: "whole file"}}

	res, err := app.SubmitReview(shown(t, app, model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "review", Verdict: "COMMENT"}}}))
	require.NoError(t, err)
	assert.False(t, deleted)
	assert.Equal(t, []string{"https://sub/review"}, res.SubmoduleReviewURLs)
}

func TestGetPRFilesAndSubmoduleDrafts_CalledConcurrentlyOnEmptyCache_LoadOnce(t *testing.T) {
	var loads atomic.Int32
	app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/pulls/1/files":
			loads.Add(1)
			_, _ = w.Write([]byte(`[{"filename":"lib","status":"modified","patch":"@@ -1 +1 @@\n-Subproject commit old\n+Subproject commit new"}]`))
		case "/repos/o/r/contents/lib":
			_, _ = w.Write([]byte(`{"type":"submodule","html_url":"https://github.com/so/sr/tree/new"}`))
		case "/repos/so/sr/compare/old...new":
			_, _ = w.Write([]byte(`{"status":"ahead","files":[{"filename":"a.txt","status":"modified","patch":"@@ -1 +1 @@\n-x\n+y"}]}`))
		case "/repos/so/sr/commits/new/pulls":
			_, _ = w.Write([]byte(`[]`))
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	})
	app.prFilesCache, app.prSubmodules, app.prSubmoduleOf = nil, nil, nil
	app.pendingComments = []model.DraftCommentDTO{{Submodule: "lib", Bump: "..new", Path: "lib/a.txt", Line: 1, Side: "RIGHT", Body: "x"}}

	var wg sync.WaitGroup
	errs := make(chan error, 30)
	for range 10 {
		wg.Go(func() {
			_, err := app.GetPRFiles()
			errs <- err
		})
		wg.Go(func() {
			_, err := app.SubmoduleDrafts(false)
			errs <- err
		})
		wg.Go(func() {
			_, err := app.GetFileDiff("lib", "lib/a.txt", false)
			errs <- err
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	assert.Equal(t, int32(1), loads.Load())
}

func TestGetViewedFiles_ParentAndSubmoduleFileShareAPath_ReportsEachMarkOnItsOwnFile(t *testing.T) {
	app := addedSubApp(t)
	_, err := app.GetPRFiles()
	require.NoError(t, err)
	fakeGraphQL(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Variables map[string]any }
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		nodes := `[]`
		if req.Variables["repo"] == "r" {
			nodes = `[{"path":"lib/a.txt","viewerViewedState":"VIEWED"}]`
		}
		_, _ = w.Write([]byte(`{"data":{"repository":{"pullRequest":{"files":{"nodes":` + nodes + `,"pageInfo":{"hasNextPage":false}}}}}}`))
	})
	require.NoError(t, viewedErr(app.SetFileViewed("lib", "lib/b.txt", false, true)))

	got, err := app.GetViewedFiles()
	require.NoError(t, err)
	assert.Equal(t, []model.FileKeyDTO{{Submodule: "lib", Path: "lib/b.txt"}, {Path: "lib/a.txt"}}, got.Files)

	require.NoError(t, viewedErr(app.SetFileViewed("lib", "lib/a.txt", false, true)))
	require.NoError(t, viewedErr(app.SetFileViewed("", "lib/a.txt", false, false)))
	got, err = app.GetViewedFiles()
	require.NoError(t, err)
	assert.Equal(t, []model.FileKeyDTO{{Submodule: "lib", Path: "lib/a.txt"}, {Submodule: "lib", Path: "lib/b.txt"}}, got.Files)
}

func TestPostImmediateComment_SubmodulePRWithUserPendingReview_NamesIt(t *testing.T) {
	app := subApp(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"Unprocessable Entity","errors":["User can only have one pending review per pull request"]}`))
	})

	_, err := app.PostImmediateComment(model.DraftCommentDTO{Submodule: "lib", Bump: "..new", Path: "lib/a.txt", Line: 2, Side: "RIGHT", Body: "x"})
	require.EqualError(t, err, "github: you already have a pending review on so/sr#5; submit or discard it on GitHub")
}

func TestLoadSubViewed_MarksOnUnlistedFiles_DeletedExceptUnderFailedSubmodule(t *testing.T) {
	app := subApp(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	app.prSubmodules["lib2"] = &prSubmodule{path: "lib2", failed: true}
	s := app.prSubmodules["lib"]
	for path, fp := range map[string]string{"lib/b.txt": s.fingerprint(app.bumpFile(s, "lib/b.txt", false)), "lib/gone.txt": "x", "lib2/c.txt": "y"} {
		require.NoError(t, app.queries.UpsertSubmoduleViewed(app.ctx, db.UpsertSubmoduleViewedParams{Owner: "o", Repo: "r", Number: 1, Submodule: strings.Split(path, "/")[0], Path: path, Fingerprint: fp}))
	}

	app.loadSubViewed()

	rows, err := app.queries.ListSubmoduleViewed(app.ctx, db.ListSubmoduleViewedParams{Owner: "o", Repo: "r", Number: 1})
	require.NoError(t, err)
	var paths []string
	for _, r := range rows {
		paths = append(paths, r.Path)
	}
	assert.ElementsMatch(t, []string{"lib/b.txt", "lib2/c.txt"}, paths)
}

func TestLoadSubViewed_IncompleteListing_KeepsMarksOnChangedOrUnlistedFiles(t *testing.T) {
	for name, incomplete := range map[string]func(*prSubmodule){
		"compare cap hit":       func(s *prSubmodule) { s.Unavailable = "too many files" },
		"submodule unavailable": func(s *prSubmodule) { s.Unavailable = "no access" },
		"PR lookup unknown":     func(s *prSubmodule) { s.prUnknown = true },
		"tree truncated":        func(s *prSubmodule) { s.BlobsTruncated = true },
	} {
		t.Run(name, func(t *testing.T) {
			app := subApp(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
			incomplete(app.prSubmodules["lib"])
			for _, path := range []string{"lib/b.txt", "lib/gone.txt"} {
				require.NoError(t, app.queries.UpsertSubmoduleViewed(app.ctx, db.UpsertSubmoduleViewedParams{Owner: "o", Repo: "r", Number: 1, Submodule: "lib", Path: path, Fingerprint: "earlier"}))
			}

			app.loadSubViewed()

			rows, err := app.queries.ListSubmoduleViewed(app.ctx, db.ListSubmoduleViewedParams{Owner: "o", Repo: "r", Number: 1})
			require.NoError(t, err)
			assert.Len(t, rows, 2)
			assert.False(t, app.prFileDTO(app.bumpFile(app.prSubmodules["lib"], "lib/b.txt", false)).Viewed, "a mark on another change still doesn't show")
		})
	}
}

func TestGetPRFiles_SubmodulePRLookupFailed_RetriedAndMovesAppMarksToFoundPR(t *testing.T) {
	lookupFails := true
	var marked []string
	fakeGraphQL(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query     string
			Variables map[string]any
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		if strings.Contains(req.Query, "markFileAsViewed") {
			marked = append(marked, fmt.Sprint(req.Variables["id"], " ", req.Variables["path"]))
		}
		_, _ = w.Write([]byte(`{"data":{}}`))
	})
	app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/pulls/1/files":
			_, _ = w.Write([]byte(`[{"filename":"lib","status":"modified","patch":"@@ -1 +1 @@\n-Subproject commit old\n+Subproject commit new"}]`))
		case "/repos/o/r/contents/lib":
			_, _ = w.Write([]byte(`{"type":"submodule","html_url":"https://github.com/so/sr/tree/new"}`))
		case "/repos/so/sr/compare/old...new":
			_, _ = w.Write([]byte(`{"status":"ahead","files":[{"filename":"a.txt","status":"modified","patch":"@@ -1 +1 @@\n-x\n+y"},{"filename":"b.txt","status":"modified","patch":"@@ -1 +1 @@\n-p\n+q"}]}`))
		case "/repos/so/sr/commits/new/pulls":
			if lookupFails {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			_, _ = w.Write([]byte(`[{"number":5,"node_id":"PR_5","state":"open","head":{"sha":"new"}}]`))
		case "/repos/so/sr/pulls/5/files":
			_, _ = w.Write([]byte(`[{"filename":"a.txt","patch":"@@ -1 +1 @@\n-x\n+y"}]`))
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	})
	app.prFilesCache, app.prSubmodules, app.prSubmoduleOf = nil, nil, nil
	_, err := app.GetPRFiles()
	require.NoError(t, err)
	require.NoError(t, viewedErr(app.SetFileViewed("lib", "lib/a.txt", false, true)))
	require.NoError(t, viewedErr(app.SetFileViewed("lib", "lib/b.txt", false, true)))

	lookupFails = false
	app.prSubmodules["lib"].retryAt = time.Time{} // the backoff has passed
	files, err := app.GetPRFiles()
	require.NoError(t, err)

	assert.Equal(t, "so/sr#5", files[0].SubmodulePR)
	assert.Equal(t, []string{"PR_5 a.txt"}, marked)
	rows, err := app.queries.ListSubmoduleViewed(app.ctx, db.ListSubmoduleViewedParams{Owner: "o", Repo: "r", Number: 1})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "lib/b.txt", rows[0].Path, "b.txt is not in the PR, so its mark stays in the app")
	assert.False(t, files[1].ViewedLocal)
	assert.True(t, files[2].Viewed)
}

func TestMigrateSubViewed_MarkOnAnotherChange_NotSentAndDeletedUnlessListingIncomplete(t *testing.T) {
	for name, tc := range map[string]struct {
		incomplete bool
		kept       bool
	}{
		"complete listing":   {false, false},
		"incomplete listing": {true, true},
	} {
		t.Run(name, func(t *testing.T) {
			marked := 0
			fakeGraphQL(t, func(w http.ResponseWriter, _ *http.Request) {
				marked++
				_, _ = w.Write([]byte(`{"data":{}}`))
			})
			app := subApp(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
			s := app.prSubmodules["lib"]
			s.BlobsTruncated = tc.incomplete
			require.NoError(t, app.queries.UpsertSubmoduleViewed(app.ctx, db.UpsertSubmoduleViewedParams{Owner: "o", Repo: "r", Number: 1, Submodule: "lib", Path: "lib/a.txt", Fingerprint: "earlier"}))
			app.subViewed[fileID{"lib", "lib/a.txt", false}] = "earlier"

			app.migrateSubViewed(s)

			assert.Zero(t, marked, "a mark on another change is not sent to the PR")
			rows, err := app.queries.ListSubmoduleViewed(app.ctx, db.ListSubmoduleViewedParams{Owner: "o", Repo: "r", Number: 1})
			require.NoError(t, err)
			assert.Equal(t, tc.kept, len(rows) == 1)
			_, inCache := app.subViewed[fileID{"lib", "lib/a.txt", false}]
			assert.Equal(t, tc.kept, inCache)
		})
	}
}

func TestSubmitReview_SubmodulePRClosedOrMovedSinceLoad_PostsNothingAndFoldsItsDrafts(t *testing.T) {
	for name, pr := range map[string]string{
		"closed":     `{"state":"closed","head":{"sha":"new"}}`,
		"head moved": `{"state":"open","head":{"sha":"newer"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			app := subAppRaw(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet && r.URL.Path == "/repos/so/sr/pulls/5" {
					_, _ = w.Write([]byte(pr))
					return
				}
				t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusTeapot)
			})
			app.pendingComments = subDrafts()
			app.prSubmodules["lib"].viewedUnknown = true

			_, err := app.SubmitReview(shown(t, app, model.ReviewSubmitRequest{Verdict: "COMMENT", Body: "x", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "review", Verdict: "COMMENT"}}}))

			require.ErrorContains(t, err, "conflict: so/sr#5")
			assert.Equal(t, subDrafts(), app.pendingComments)
			assert.False(t, app.prSubmodules["lib"].viewedUnknown, "no PR left to read Viewed marks from")
			got, err := app.SubmoduleDrafts(false)
			require.NoError(t, err)
			assert.Equal(t, []model.SubmoduleDraftsDTO{{Path: "lib", Folded: 2}}, got)
		})
	}
}

func TestSetFileViewed_SubmodulePRCheckedOverAMinuteAgoAndClosed_KeepsTheMarkInGitura(t *testing.T) {
	var marks, checks int
	fakeGraphQL(t, func(w http.ResponseWriter, _ *http.Request) {
		marks++
		_, _ = w.Write([]byte(`{"data":{}}`))
	})
	app := subAppRaw(t, func(w http.ResponseWriter, r *http.Request) {
		checks++
		_, _ = w.Write([]byte(`{"state":"closed","head":{"sha":"new"}}`))
	})
	s := app.prSubmodules["lib"]
	f := app.bumpFile(s, "lib/a.txt", false)

	s.checkedAt = time.Now()
	rerouted, err := app.SetFileViewed("lib", "lib/a.txt", false, true)
	require.NoError(t, err)
	assert.False(t, rerouted)
	assert.Equal(t, []int{1, 0}, []int{marks, checks}, "a fresh check is trusted")

	s.checkedAt = time.Now().Add(-2 * time.Minute)
	rerouted, err = app.SetFileViewed("lib", "lib/a.txt", false, true)
	require.NoError(t, err)
	assert.True(t, rerouted, "the file list has to show the mark kept in gitura")
	assert.Equal(t, []int{1, 1}, []int{marks, checks})
	assert.Nil(t, s.pr)
	dto := app.prFileDTO(f)
	assert.True(t, dto.Viewed)
	assert.True(t, dto.ViewedLocal)
}

func TestSetFileViewed_FileMissingFromSubmodulePRFilesCheckedOverAMinuteAgo_RereadsThemAndMarksItOnGitHub(t *testing.T) {
	var marks, reads int
	fakeGraphQL(t, func(w http.ResponseWriter, _ *http.Request) {
		marks++
		_, _ = w.Write([]byte(`{"data":{}}`))
	})
	app := subAppRaw(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/so/sr/pulls/5":
			_, _ = w.Write([]byte(`{"number":5,"state":"open","head":{"sha":"new"}}`))
		case "/repos/so/sr/pulls/5/files":
			reads++
			_, _ = w.Write([]byte(`[{"filename":"a.txt"},{"filename":"b.txt"}]`))
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	})
	s := app.prSubmodules["lib"]

	s.checkedAt = time.Now()
	rerouted, err := app.SetFileViewed("lib", "lib/b.txt", false, true)
	require.NoError(t, err)
	assert.False(t, rerouted)
	assert.Equal(t, []int{0, 0}, []int{marks, reads}, "a fresh file list is trusted")
	require.NoError(t, viewedErr(app.SetFileViewed("lib", "lib/b.txt", false, false)))

	s.checkedAt = time.Now().Add(-2 * time.Minute)
	rerouted, err = app.SetFileViewed("lib", "lib/b.txt", false, true)
	require.NoError(t, err)
	assert.True(t, rerouted, "the file list has to show the mark on GitHub")
	assert.Equal(t, []int{1, 1}, []int{marks, reads})
	assert.Empty(t, app.subViewed, "nothing is kept in gitura")
}

func TestSetFileViewed_SubmodulePRRetargetedToIncludeAFileMarkedInGitura_MovesThatMarkToGitHub(t *testing.T) {
	var marks int
	fakeGraphQL(t, func(w http.ResponseWriter, _ *http.Request) {
		marks++
		_, _ = w.Write([]byte(`{"data":{}}`))
	})
	app := subAppRaw(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/so/sr/pulls/5":
			_, _ = w.Write([]byte(`{"number":5,"state":"open","head":{"sha":"new"},"base":{"sha":"retargeted"}}`))
		case "/repos/so/sr/pulls/5/files":
			_, _ = w.Write([]byte(`[{"filename":"a.txt"},{"filename":"b.txt"}]`))
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	})
	app.viewedFiles = map[fileID]bool{}
	s := app.prSubmodules["lib"]
	s.checkedAt = time.Now()
	require.NoError(t, viewedErr(app.SetFileViewed("lib", "lib/b.txt", false, true)))
	require.Contains(t, app.subViewed, fileID{"lib", "lib/b.txt", false})

	s.checkedAt = time.Now().Add(-2 * time.Minute)
	rerouted, err := app.SetFileViewed("lib", "lib/a.txt", false, true)
	require.NoError(t, err)

	assert.True(t, rerouted, "b.txt's row now shows its mark on GitHub")
	assert.Empty(t, app.subViewed)
	dto := app.prFileDTO(app.bumpFile(s, "lib/b.txt", false))
	assert.True(t, dto.Viewed)
	assert.False(t, dto.ViewedLocal)
	assert.Equal(t, 3, marks, "read the PR's marks, move b.txt's, mark a.txt")
}

func TestSubmoduleDrafts_DraftWrittenOnAnotherBumpOfTheSameHead_Folds(t *testing.T) {
	app := subApp(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	_, err := app.AddDraftComment(model.DraftCommentDTO{Submodule: "lib", Path: "lib/a.txt", Line: 2, Side: "RIGHT", Body: "in PR diff"})
	require.NoError(t, err)
	require.Equal(t, "..new", app.pendingComments[0].Bump)

	app.prSubmodules["lib"].from = "other" // the parent PR retargeted: same head, the bump starts elsewhere

	got, err := app.SubmoduleDrafts(false)
	require.NoError(t, err)
	assert.Equal(t, []model.SubmoduleDraftsDTO{{Path: "lib", PR: "so/sr#5", Folded: 1, Outdated: 1}}, got)
}

func TestSetFileViewed_UnmarkOnSubmodulePRWithLeftoverGituraMark_DeletesTheStoredMark(t *testing.T) {
	fakeGraphQL(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"data":{}}`)) })
	app := subApp(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	s := app.prSubmodules["lib"]
	s.checkedAt = time.Now()
	fp := s.fingerprint(app.bumpFile(s, "lib/a.txt", false))
	require.NoError(t, app.queries.UpsertSubmoduleViewed(app.ctx, db.UpsertSubmoduleViewedParams{Owner: "o", Repo: "r", Number: 1, Submodule: "lib", Path: "lib/a.txt", Fingerprint: fp}))
	app.subViewed[fileID{"lib", "lib/a.txt", false}] = fp

	require.NoError(t, viewedErr(app.SetFileViewed("lib", "lib/a.txt", false, false)))

	assert.Empty(t, app.subViewed)
	app.loadSubViewed()
	assert.Empty(t, app.subViewed, "the stored mark is gone, so it can't come back once the PR is")
}

func TestGetViewedFiles_SubmodulePRMarksReadFails_FlagsPointerAndRetriesAfterBackoff(t *testing.T) {
	subFails := true
	fakeGraphQL(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Variables map[string]any }
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		nodes := `[]`
		if req.Variables["repo"] == "sr" {
			if subFails {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			nodes = `[{"path":"a.txt","viewerViewedState":"VIEWED"}]`
		}
		_, _ = w.Write([]byte(`{"data":{"repository":{"pullRequest":{"files":{"nodes":` + nodes + `,"pageInfo":{"hasNextPage":false}}}}}}`))
	})
	app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/pulls/1/files":
			_, _ = w.Write([]byte(`[{"filename":"lib","status":"modified","patch":"@@ -1 +1 @@\n-Subproject commit old\n+Subproject commit new"}]`))
		case "/repos/o/r/contents/lib":
			_, _ = w.Write([]byte(`{"type":"submodule","html_url":"https://github.com/so/sr/tree/new"}`))
		case "/repos/so/sr/compare/old...new":
			_, _ = w.Write([]byte(`{"status":"ahead","files":[{"filename":"a.txt","status":"modified","patch":"@@ -1 +1 @@\n-x\n+y"}]}`))
		case "/repos/so/sr/commits/new/pulls":
			_, _ = w.Write([]byte(`[{"number":5,"node_id":"PR_5","state":"open","head":{"sha":"new"}}]`))
		case "/repos/so/sr/pulls/5/files":
			_, _ = w.Write([]byte(`[{"filename":"a.txt","patch":"@@ -1 +1 @@\n-x\n+y"}]`))
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	})
	app.prFilesCache, app.prSubmodules, app.prSubmoduleOf = nil, nil, nil
	viewedUnknown := func() bool {
		t.Helper()
		files, err := app.GetPRFiles()
		require.NoError(t, err)
		return files[0].SubmoduleViewedUnknown
	}

	got, err := app.GetViewedFiles()
	require.NoError(t, err)
	assert.Empty(t, got.Files)
	assert.Equal(t, []string{"lib"}, got.UnknownSubmodules, "the first read already reports the failure")
	assert.True(t, viewedUnknown())

	subFails = false
	assert.True(t, viewedUnknown(), "not retried within the backoff")
	app.prSubmodules["lib"].retryAt = time.Time{}
	got, err = app.GetViewedFiles()
	require.NoError(t, err)
	assert.Equal(t, []model.FileKeyDTO{{Submodule: "lib", Path: "lib/a.txt"}}, got.Files)
	assert.Empty(t, got.UnknownSubmodules)
	assert.False(t, viewedUnknown())
}

// retargetedSubApp is subApp whose submodule PR #5, loaded at base b1, now reads base b2 with a diff whose
// left side no longer matches the bump's; posts counts what it is sent.
func retargetedSubApp(t *testing.T, posts *int) *App {
	t.Helper()
	app := subAppRaw(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost:
			*posts++
			_, _ = w.Write([]byte(`{"id":7,"html_url":"https://sub/posted"}`))
		case r.URL.Path == "/repos/so/sr/pulls/5":
			_, _ = w.Write([]byte(`{"number":5,"state":"open","head":{"sha":"new"},"base":{"sha":"b2"}}`))
		case r.URL.Path == "/repos/so/sr/pulls/5/files":
			_, _ = w.Write([]byte(`[{"filename":"a.txt","patch":"@@ -1,2 +1,3 @@\n a\n+b\n c"}]`))
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	})
	app.prSubmodules["lib"].pr.Base = &github.PullRequestBranch{SHA: new("b1")}
	return app
}

func TestSubmitReview_SubmodulePRRetargeted_RechecksAnchorsAgainstItsNewDiff(t *testing.T) {
	for name, tc := range map[string]struct {
		draft   model.DraftCommentDTO
		wantErr string
		posts   int
	}{
		"left-side anchor no longer in its diff": {model.DraftCommentDTO{Submodule: "lib", Bump: "..new", Path: "lib/a.txt", Line: 1, Side: "LEFT", Body: "old"}, "conflict:", 0},
		"right-side anchor still in its diff":    {model.DraftCommentDTO{Submodule: "lib", Bump: "..new", Path: "lib/a.txt", Line: 2, Side: "RIGHT", Body: "new"}, "", 1},
	} {
		t.Run(name, func(t *testing.T) {
			posts := 0
			app := retargetedSubApp(t, &posts)
			app.pendingComments = []model.DraftCommentDTO{tc.draft}
			counted, err := app.SubmoduleDrafts(false)
			require.NoError(t, err)
			require.Equal(t, 1, counted[0].ToPR)

			_, err = app.SubmitReview(shown(t, app, model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "comments"}}}))

			assert.Equal(t, tc.posts, posts)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
			counted, err = app.SubmoduleDrafts(false)
			require.NoError(t, err)
			assert.Equal(t, 1, counted[0].Folded, "the dialog's next count matches what would be sent")
		})
	}
}

func TestPostImmediateComment_SubmodulePRRetargeted_RefusesAnchorNoLongerInItsDiff(t *testing.T) {
	posts := 0
	app := retargetedSubApp(t, &posts)

	_, err := app.PostImmediateComment(model.DraftCommentDTO{Submodule: "lib", Bump: "..new", Path: "lib/a.txt", Line: 1, Side: "LEFT", Body: "old"})

	require.ErrorContains(t, err, "conflict:")
	assert.Equal(t, 0, posts)
}

// viewedErr is SetFileViewed's error, for tests that don't check whether the mark was rerouted.
func viewedErr(_ bool, err error) error { return err }

// shown is req as the submit dialog sends it: each choice carries the routing SubmoduleDrafts reported, and a
// submodule with drafts but no choice in req gets a fold one.
func shown(t *testing.T, app *App, req model.ReviewSubmitRequest) model.ReviewSubmitRequest {
	t.Helper()
	app.prMu.Lock()
	defer app.prMu.Unlock()
	require.NoError(t, app.loadPRFiles())
	routes := app.draftRoutes()
	// A choice for a path the PR doesn't bump stays, for the validation it fails.
	choices := slices.DeleteFunc(slices.Clone(req.Submodules), func(c model.SubmoduleReviewChoice) bool {
		return app.prSubmodules[c.Path] != nil || app.hasSubmoduleDrafts(c.Path)
	})
	for path, r := range routes {
		c := choiceFor(req.Submodules, path)
		c.PR, c.ToPR, c.Folded = r.pr, r.toPR, r.folded
		choices = append(choices, c)
	}
	req.Submodules = choices
	return req
}

func TestSubmitReview_RoutingDiffersFromWhatTheDialogShowed_ReturnsConflictWithoutPosting(t *testing.T) {
	routed := func(pr string, toPR, folded int) []model.SubmoduleReviewChoice {
		return []model.SubmoduleReviewChoice{{Path: "lib", Mode: "review", Verdict: "COMMENT", PR: pr, ToPR: toPR, Folded: folded}}
	}
	cases := map[string][]model.SubmoduleReviewChoice{
		"no choice for a submodule with drafts": nil,
		"another PR":                            routed("so/sr#4", 1, 1),
		"other counts":                          routed("so/sr#5", 2, 0),
		"expected to fold, now goes to a PR":    {{Path: "lib", Mode: "fold", Folded: 2}},
		"a submodule without drafts":            append(routed("so/sr#5", 1, 1), model.SubmoduleReviewChoice{Path: "lib", Mode: "fold", Folded: 1}),
	}
	for name, choices := range cases {
		got := map[string][]map[string]any{}
		app := subApp(t, recordReviews(t, got))
		app.pendingComments = subDrafts()

		_, err := app.SubmitReview(model.ReviewSubmitRequest{Verdict: "COMMENT", Body: "x", Submodules: choices})
		require.ErrorContains(t, err, "conflict:", name)
		assert.Empty(t, got, name)
		assert.Len(t, app.pendingComments, 2, name)
	}

	got := map[string][]map[string]any{}
	app := subApp(t, recordReviews(t, got))
	app.pendingComments = subDrafts()
	_, err := app.SubmitReview(model.ReviewSubmitRequest{Verdict: "COMMENT", Body: "x", Submodules: routed("so/sr#5", 1, 1)})
	require.NoError(t, err)
	assert.Len(t, got["/repos/so/sr/pulls/5/reviews"], 1)
}

func TestAddDraftComment_ParentHeadMovedWithOnlySubmoduleDrafts_AddsBothKindsAndPinsOnlyParentDrafts(t *testing.T) {
	app := subApp(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	app.pendingComments, app.pendingHeadSHA = subDrafts()[:1], "sha"
	app.prCache.HeadSHA = "moved"

	got, err := app.AddDraftComment(subDrafts()[1])
	require.NoError(t, err)
	assert.Empty(t, got.HeadSHA, "submodule drafts pin no parent head")
	got, err = app.AddDraftComment(model.DraftCommentDTO{Path: "a.go", Line: 1, Side: "RIGHT", Body: "parent"})
	require.NoError(t, err)
	assert.Equal(t, "moved", got.HeadSHA)

	app.prCache.HeadSHA = "moved again"
	_, err = app.AddDraftComment(model.DraftCommentDTO{Path: "a.go", Line: 2, Side: "RIGHT", Body: "parent"})
	require.ErrorContains(t, err, "older commit")
	_, err = app.AddDraftComment(subDrafts()[0])
	require.NoError(t, err, "a parent head move never blocks a submodule draft")
	assert.Equal(t, "moved", app.reviewCommitSHA())
}

func TestGetPRFiles_BumpComparesANestedGitlinkBump_ExpandsItAsItsOwnSubmodule(t *testing.T) {
	app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/pulls/1/files":
			_, _ = w.Write([]byte(`[{"filename":"lib","status":"modified","patch":"@@ -1 +1 @@\n-Subproject commit old\n+Subproject commit new"}]`))
		case "/repos/o/r/contents/lib":
			_, _ = w.Write([]byte(`{"type":"submodule","html_url":"https://github.com/so/sr/tree/new"}`))
		case "/repos/so/sr/compare/old...new":
			_, _ = w.Write([]byte(`{"status":"ahead","files":[{"filename":"a.txt","status":"modified","patch":"@@ -1 +1 @@\n-x\n+y"},
				{"filename":"deps/core","status":"modified","patch":"@@ -1 +1 @@\n-Subproject commit cold\n+Subproject commit cnew"}]}`))
		case "/repos/so/sr/contents/deps/core":
			assert.Equal(t, "new", r.URL.Query().Get("ref"), "resolved in the outer submodule at its new commit")
			_, _ = w.Write([]byte(`{"type":"submodule","html_url":"https://github.com/no/nr/tree/cnew"}`))
		case "/repos/no/nr/compare/cold...cnew":
			_, _ = w.Write([]byte(`{"status":"ahead","files":[{"filename":"x.txt","status":"modified","patch":"@@ -1 +1 @@\n-p\n+q"}]}`))
		case "/repos/so/sr/commits/new/pulls":
			_, _ = w.Write([]byte(`[]`))
		case "/repos/no/nr/commits/cnew/pulls":
			_, _ = w.Write([]byte(`[{"number":7,"state":"open","head":{"sha":"cnew"},"html_url":"https://github.com/no/nr/pull/7"}]`))
		case "/repos/no/nr/pulls/7/files":
			_, _ = w.Write([]byte(`[{"filename":"x.txt","patch":"@@ -1 +1 @@\n-p\n+q"}]`))
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	})
	app.prFilesCache, app.prSubmodules, app.prSubmoduleOf = nil, nil, nil

	files, err := app.GetPRFiles()
	require.NoError(t, err)
	var rows []string
	for _, f := range files {
		rows = append(rows, f.Filename+" "+f.Status+" "+f.Repo+" "+f.Submodule+" "+f.SubmodulePR)
	}
	assert.Equal(t, []string{
		"lib submodule  lib ",
		"lib/a.txt modified lib lib ",
		"lib/deps/core submodule lib lib/deps/core no/nr#7",
		"lib/deps/core/x.txt modified lib/deps/core lib/deps/core ",
	}, rows)
	d, err := app.GetFileDiff("lib", "lib/deps/core", true)
	require.NoError(t, err, "the nested pointer row is a file of the submodule holding it")
	assert.NotEmpty(t, d.Hunks)

	_, err = app.AddDraftComment(model.DraftCommentDTO{Submodule: "lib/deps/core", Range: "cold..cnew", Path: "lib/deps/core/x.txt", Line: 1, Side: "RIGHT", Body: "inner"})
	require.NoError(t, err)
	_, err = app.AddDraftComment(model.DraftCommentDTO{Submodule: "lib", Path: "lib/deps/core", Pointer: true, Line: 1, Side: "RIGHT", Body: "on the nested pointer"})
	require.NoError(t, err)
	assert.Equal(t, []string{"lib/deps/core", "lib"}, []string{app.pendingComments[0].Submodule, app.pendingComments[1].Submodule})
	drafts, err := app.SubmoduleDrafts(false)
	require.NoError(t, err)
	assert.Equal(t, []model.SubmoduleDraftsDTO{
		{Path: "lib/deps/core", PR: "no/nr#7", PRURL: "https://github.com/no/nr/pull/7", ToPR: 1},
		{Path: "lib", Folded: 1},
	}, drafts)
}

func TestLoadSubViewed_OuterSubmoduleFailsToLoad_KeepsNestedMarksUntilItLoadsAgain(t *testing.T) {
	var compareFails atomic.Bool
	app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/pulls/1/files":
			_, _ = w.Write([]byte(`[{"filename":"lib","status":"modified","patch":"@@ -1 +1 @@\n-Subproject commit old\n+Subproject commit new"}]`))
		case "/repos/o/r/contents/lib":
			_, _ = w.Write([]byte(`{"type":"submodule","html_url":"https://github.com/so/sr/tree/new"}`))
		case "/repos/so/sr/compare/old...new":
			if compareFails.Load() {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			_, _ = w.Write([]byte(`{"status":"ahead","files":[{"filename":"deps/core","status":"modified","patch":"@@ -1 +1 @@\n-Subproject commit cold\n+Subproject commit cnew"}]}`))
		case "/repos/so/sr/contents/deps/core":
			_, _ = w.Write([]byte(`{"type":"submodule","html_url":"https://github.com/no/nr/tree/cnew"}`))
		case "/repos/no/nr/compare/cold...cnew":
			_, _ = w.Write([]byte(`{"status":"ahead","files":[{"filename":"x.txt","status":"modified","patch":"@@ -1 +1 @@\n-p\n+q"}]}`))
		case "/repos/so/sr/commits/new/pulls", "/repos/no/nr/commits/cnew/pulls":
			_, _ = w.Write([]byte(`[]`))
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	})
	app.prFilesCache, app.prSubmodules, app.prSubmoduleOf = nil, nil, nil
	_, err := app.GetPRFiles()
	require.NoError(t, err)
	require.NoError(t, viewedErr(app.SetFileViewed("lib/deps/core", "lib/deps/core/x.txt", false, true)))

	compareFails.Store(true)
	app.prFilesCache, app.prSubmodules, app.prSubmoduleOf = nil, nil, nil
	files, err := app.GetPRFiles()
	require.NoError(t, err)
	require.Len(t, files, 1)
	rows, err := app.queries.ListSubmoduleViewed(app.ctx, db.ListSubmoduleViewedParams{Owner: "o", Repo: "r", Number: 1})
	require.NoError(t, err)
	assert.Len(t, rows, 1, "a nested mark is kept while the submodule holding it can't be listed")

	compareFails.Store(false)
	app.prSubmodules["lib"].retryAt = time.Time{}
	files, err = app.GetPRFiles()
	require.NoError(t, err)
	viewed := map[string]bool{}
	for _, f := range files {
		viewed[f.Filename] = f.Viewed
	}
	assert.Equal(t, map[string]bool{"lib": false, "lib/deps/core": false, "lib/deps/core/x.txt": true}, viewed)
}

func TestGetPRFiles_RemovesOwnSubmoduleUnderAddedOne_PointerStaysInParentRepo(t *testing.T) {
	app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/o/r/pulls/1/files":
			_, _ = w.Write([]byte(`[
				{"filename":"lib","status":"added","patch":"@@ -0,0 +1 @@\n+Subproject commit new"},
				{"filename":"lib/vendor","status":"removed","patch":"@@ -1 +0,0 @@\n-Subproject commit vold"}]`))
		case r.URL.Path == "/repos/o/r/compare/main...sha":
			_, _ = w.Write([]byte(`{"merge_base_commit":{"sha":"mb"}}`))
		case r.URL.Path == "/repos/o/r/contents/lib":
			_, _ = w.Write([]byte(`{"type":"submodule","html_url":"https://github.com/so/sr/tree/new"}`))
		case r.URL.Path == "/repos/o/r/contents/lib/vendor" && r.URL.Query().Get("ref") == "mb":
			_, _ = w.Write([]byte(`{"type":"submodule","html_url":"https://github.com/vo/vr/tree/vold"}`))
		case r.URL.Path == "/repos/so/sr/git/trees/new":
			_, _ = w.Write([]byte(`{"tree":[{"path":"a.txt","type":"blob","sha":"a1"}]}`))
		case r.URL.Path == "/repos/vo/vr/git/trees/vold":
			_, _ = w.Write([]byte(`{"tree":[{"path":"v.txt","type":"blob","sha":"v1"}]}`))
		case r.URL.Path == "/repos/so/sr/commits/new/pulls":
			_, _ = w.Write([]byte(`[]`))
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	})
	app.prCache.BaseBranch = "main"
	app.prFilesCache, app.prSubmodules, app.prSubmoduleOf = nil, nil, nil

	files, err := app.GetPRFiles()
	require.NoError(t, err)
	var rows []string
	for _, f := range files {
		rows = append(rows, f.Filename+" "+f.Status+" "+f.Repo)
	}
	assert.Equal(t, []string{"lib submodule ", "lib/a.txt added lib", "lib/vendor submodule ", "lib/vendor/v.txt removed lib/vendor"}, rows)
	_, err = app.GetFileDiff("", "lib/vendor", true)
	require.NoError(t, err)
	_, err = app.GetFileDiff("lib", "lib/vendor", true)
	require.ErrorContains(t, err, "notfound:", "lib doesn't hold vendor")
}

func TestLoadSubViewed_ParentFileListTruncated_KeepsMarksOfUnlistedSubmodules(t *testing.T) {
	for name, tc := range map[string]struct {
		listed int
		kept   []string
	}{
		"under the cap": {listed: 2, kept: nil},
		"at the cap":    {listed: prFileListCap, kept: []string{"gone/x.txt"}},
	} {
		t.Run(name, func(t *testing.T) {
			listing := make([]map[string]string, tc.listed)
			for i := range listing {
				listing[i] = map[string]string{"filename": fmt.Sprintf("f%d.txt", i), "status": "modified"}
			}
			app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/repos/o/r/pulls/1/files" {
					w.WriteHeader(http.StatusTeapot)
					return
				}
				page, err := strconv.Atoi(cmp.Or(r.URL.Query().Get("page"), "1"))
				require.NoError(t, err)
				if end := min(page*100, len(listing)); end < len(listing) {
					w.Header().Set("Link", fmt.Sprintf(`<%s?page=%d>; rel="next"`, r.URL.Path, page+1))
					require.NoError(t, json.NewEncoder(w).Encode(listing[(page-1)*100:end]))
					return
				}
				require.NoError(t, json.NewEncoder(w).Encode(listing[(page-1)*100:]))
			})
			app.prFilesCache, app.prSubmodules, app.prSubmoduleOf = nil, nil, nil
			require.NoError(t, app.queries.UpsertSubmoduleViewed(app.ctx, db.UpsertSubmoduleViewedParams{Owner: "o", Repo: "r", Number: 1, Submodule: "gone", Path: "gone/x.txt", Fingerprint: "fp"}))

			_, err := app.GetPRFiles()
			require.NoError(t, err)

			rows, err := app.queries.ListSubmoduleViewed(app.ctx, db.ListSubmoduleViewedParams{Owner: "o", Repo: "r", Number: 1})
			require.NoError(t, err)
			var kept []string
			for _, r := range rows {
				kept = append(kept, r.Path)
			}
			assert.Equal(t, tc.kept, kept)
		})
	}
}

func TestGetPRFiles_SubmoduleReplacedByFileAtItsPath_ListsBothWithOwnDiffsSharingGitHubsMark(t *testing.T) {
	app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/o/r/pulls/1/files":
			_, _ = w.Write([]byte(`[
				{"filename":"lib","status":"removed","deletions":1,"patch":"@@ -1 +0,0 @@\n-Subproject commit old"},
				{"filename":"lib","status":"added","additions":1,"patch":"@@ -0,0 +1 @@\n+text"}]`))
		case r.URL.Path == "/repos/o/r/compare/main...sha":
			_, _ = w.Write([]byte(`{"merge_base_commit":{"sha":"mb"}}`))
		case r.URL.Path == "/repos/o/r/contents/lib" && r.URL.Query().Get("ref") == "mb":
			_, _ = w.Write([]byte(`{"type":"submodule","html_url":"https://github.com/so/sr/tree/old"}`))
		case r.URL.Path == "/repos/so/sr/git/trees/old":
			_, _ = w.Write([]byte(`{"tree":[{"path":"a.txt","type":"blob","sha":"a1"}]}`))
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	})
	app.prCache.BaseBranch = "main"
	app.prFilesCache, app.prSubmodules, app.prSubmoduleOf = nil, nil, nil

	files, err := app.GetPRFiles()
	require.NoError(t, err)
	var rows []string
	for _, f := range files {
		rows = append(rows, f.Filename+" "+f.Status+" "+f.Repo)
	}
	assert.Equal(t, []string{"lib submodule ", "lib/a.txt removed lib", "lib added "}, rows)

	pointer, err := app.GetFileDiff("", "lib", true)
	require.NoError(t, err)
	file, err := app.GetFileDiff("", "lib", false)
	require.NoError(t, err)
	assert.Equal(t, []int{0, 1}, []int{pointer.TotalAdditions, pointer.TotalDeletions})
	assert.Equal(t, []int{1, 0}, []int{file.TotalAdditions, file.TotalDeletions})

	app.viewedFiles = map[fileID]bool{{path: "lib"}: true}
	viewed, err := app.GetViewedFiles()
	require.NoError(t, err)
	assert.Equal(t, []model.FileKeyDTO{{Path: "lib", Pointer: true}, {Path: "lib"}}, viewed.Files, "GitHub keeps one mark per path")
}

const (
	twinPointerPatch = "@@ -1 +1 @@\n-Subproject commit va\n+Subproject commit vb"
	twinFilePatch    = "@@ -0,0 +1,3 @@\n+x\n+y\n+z"
)

// twinApp is subApp with lib's bump turning nested submodule vendor into a file there, and so/sr#5 listing prFiles.
func twinApp(t *testing.T, prFiles []*github.CommitFile) *App {
	t.Helper()
	app := subApp(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	lib := app.prSubmodules["lib"]
	lib.Files = []*github.CommitFile{
		{Filename: new("lib/vendor"), Status: new("modified"), Patch: new(twinPointerPatch)},
		{Filename: new("lib/vendor"), Status: new("added"), Patch: new(twinFilePatch)},
	}
	lib.setPR(lib.pr, prFiles)
	lib.checkedAt = time.Now()
	app.prSubmodules["lib/vendor"] = &prSubmodule{in: lib, path: "lib/vendor", from: "va", to: "vb"}
	app.prFilesCache, app.prSubmoduleOf = lib.Files, map[*github.CommitFile]*prSubmodule{}
	for _, f := range lib.Files {
		app.prSubmoduleOf[f] = lib
	}
	return app
}

func TestGetFileDiffAndSetFileViewed_NestedPointerAndFileShareAPath_EachReachesItsOwnRow(t *testing.T) {
	app := twinApp(t, nil)

	ptr, err := app.GetFileDiff("lib", "lib/vendor", true)
	require.NoError(t, err)
	file, err := app.GetFileDiff("lib", "lib/vendor", false)
	require.NoError(t, err)
	assert.Equal(t, []string{"@@ -1 +1 @@", "@@ -0,0 +1,3 @@"}, []string{ptr.Hunks[0].Header, file.Hunks[0].Header})

	for _, pointer := range []bool{true, false} {
		require.NoError(t, viewedErr(app.SetFileViewed("lib", "lib/vendor", pointer, true)))
		require.NoError(t, viewedErr(app.SetFileViewed("lib", "lib/vendor", !pointer, false)))
		files, err := app.GetPRFiles()
		require.NoError(t, err)
		viewed := map[string]bool{}
		for _, f := range files {
			viewed[f.Status] = f.Viewed
		}
		assert.Equal(t, map[string]bool{"submodule": pointer, "added": !pointer}, viewed)
	}
}

func TestSubmoduleDrafts_SubmodulePRListsPointerFileOrBothAtOnePath_SendsOnlyDraftsOfAListedKind(t *testing.T) {
	ptr := &github.CommitFile{Filename: new("vendor"), Patch: new(twinPointerPatch)}
	file := &github.CommitFile{Filename: new("vendor"), Patch: new(twinFilePatch)}
	for name, tc := range map[string]struct {
		prFiles      []*github.CommitFile
		toPR, folded int
	}{
		"neither":      {nil, 0, 2},
		"file only":    {[]*github.CommitFile{file}, 1, 1},
		"pointer only": {[]*github.CommitFile{ptr}, 1, 1},
		"both":         {[]*github.CommitFile{ptr, file}, 2, 0},
	} {
		t.Run(name, func(t *testing.T) {
			app := twinApp(t, tc.prFiles)
			app.pendingComments = []model.DraftCommentDTO{
				{Submodule: "lib", Bump: "..new", Path: "lib/vendor", Pointer: true, Line: 1, Side: "RIGHT", Body: "pointer"},
				{Submodule: "lib", Bump: "..new", Path: "lib/vendor", Line: 3, Side: "RIGHT", Body: "file"},
			}

			drafts, err := app.SubmoduleDrafts(false)
			require.NoError(t, err)
			require.Len(t, drafts, 1)
			assert.Equal(t, []int{tc.toPR, tc.folded}, []int{drafts[0].ToPR, drafts[0].Folded})
		})
	}
}

func TestSubmoduleDrafts_FoldOnlySubmodulePRRetargetedToIncludeTheFile_RoutesDraftThere(t *testing.T) {
	var writes []string
	app := subAppRaw(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writes = append(writes, r.Method+" "+r.URL.Path)
			_, _ = w.Write([]byte(`{"id":7}`))
			return
		}
		switch r.URL.Path {
		case "/repos/so/sr/pulls/5":
			_, _ = w.Write([]byte(`{"number":5,"state":"open","head":{"sha":"new"},"base":{"sha":"b2"}}`))
		case "/repos/so/sr/pulls/5/files":
			_, _ = w.Write([]byte(`[{"filename":"a.txt","patch":"` + strings.ReplaceAll(subPatch, "\n", `\n`) + `"},{"filename":"b.txt","patch":"@@ -1 +1 @@\n-x\n+y"}]`))
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	})
	app.prSubmodules["lib"].pr.Base = &github.PullRequestBranch{SHA: new("b1")}
	app.pendingComments = []model.DraftCommentDTO{{Submodule: "lib", Bump: "..new", Path: "lib/b.txt", Line: 1, Side: "RIGHT", Body: "b"}}

	got, err := app.SubmoduleDrafts(false)

	require.NoError(t, err)
	assert.Equal(t, []model.SubmoduleDraftsDTO{{Path: "lib", PR: "so/sr#5", ToPR: 1, Rerouted: true}}, got)

	app.orphans = []orphanReview{{"so", "sr", 5, 10}}
	_, err = app.SubmitReview(shown(t, app, model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "review", Verdict: "COMMENT"}}}))
	require.NoError(t, err)
	assert.Equal(t, []string{"DELETE /repos/so/sr/pulls/5/reviews/10", "POST /repos/so/sr/pulls/5/reviews"}, writes,
		"the orphan goes before the draft is posted to the retargeted PR")
}
