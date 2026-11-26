package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v93/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitura/internal/model"
)

func stamp(d time.Duration) string {
	return time.Now().Add(d).UTC().Format(time.RFC3339)
}

func TestSubmitReview_SubmoduleReviewAnswers5xxWithOnlyALookAlike_KeepsDraftsAndReturnsError(t *testing.T) {
	const review = `{"id":7,"user":{"login":"me"},"commit_id":"new","body":"","html_url":"https://sub/review",`
	const onLine2 = `[{"path":"a.txt","line":2,"side":"RIGHT","body":"in PR diff"}]`
	for name, tc := range map[string]struct{ reviews, comments string }{
		"comment on another line": {`[` + review + `"state":"COMMENTED","submitted_at":"` + stamp(0) + `"}]`,
			`[{"path":"a.txt","line":3,"side":"RIGHT","body":"in PR diff"}]`},
		"comment on another side": {`[` + review + `"state":"COMMENTED","submitted_at":"` + stamp(0) + `"}]`,
			`[{"path":"a.txt","line":2,"side":"LEFT","body":"in PR diff"}]`},
		"comment starting a range": {`[` + review + `"state":"COMMENTED","submitted_at":"` + stamp(0) + `"}]`,
			`[{"path":"a.txt","start_line":1,"line":2,"side":"RIGHT","body":"in PR diff"}]`},
		"submitted before the attempt": {`[` + review + `"state":"COMMENTED","submitted_at":"` + stamp(-10*time.Minute) + `"}]`, onLine2},
		"another verdict":              {`[` + review + `"state":"APPROVED","submitted_at":"` + stamp(0) + `"}]`, onLine2},
		"reviews list fails":           {"", onLine2},
		"review comments list fails":   {`[` + review + `"state":"COMMENTED","submitted_at":"` + stamp(0) + `"}]`, ""},
	} {
		t.Run(name, func(t *testing.T) {
			posts := 0
			app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/repos/so/sr/pulls/5/reviews":
					posts++
					w.WriteHeader(http.StatusBadGateway)
				case r.URL.Path == "/repos/so/sr/pulls/5/reviews" && tc.reviews != "":
					_, _ = w.Write([]byte(tc.reviews))
				case r.URL.Path == "/repos/so/sr/pulls/5/reviews/7/comments" && tc.comments != "":
					_, _ = w.Write([]byte(tc.comments))
				default:
					w.WriteHeader(http.StatusInternalServerError)
				}
			})
			app.pendingComments = subDrafts()

			res, err := app.SubmitReview(shown(t, app, model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "review", Verdict: "COMMENT"}}}))
			require.ErrorContains(t, err, "so/sr#5")
			assert.Empty(t, res.SubmoduleReviewURLs)
			assert.Equal(t, 1, posts)
			assert.Equal(t, subDrafts(), app.pendingComments)
			assert.Empty(t, app.subReviewURLs)
		})
	}
}

func TestSubmitReview_SubmoduleCommentAnswers5xxWithOnlyALookAlike_KeepsDraftAndReturnsError(t *testing.T) {
	const mine = `{"user":{"login":"me"},"path":"a.txt","body":"in PR diff","commit_id":"new","side":"RIGHT","html_url":"https://sub/comment",`
	for name, tc := range map[string]struct {
		comments string
		login    string
	}{
		"comment on another line":        {`[` + mine + `"line":3,"created_at":"` + stamp(0) + `"}]`, "me"},
		"comment created before attempt": {`[` + mine + `"line":2,"created_at":"` + stamp(-10*time.Minute) + `"}]`, "me"},
		"viewer login fails":             {`[` + mine + `"line":2,"created_at":"` + stamp(0) + `"}]`, ""},
		"comments list fails":            {"", "me"},
	} {
		t.Run(name, func(t *testing.T) {
			posts := 0
			app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/repos/so/sr/pulls/5/comments":
					posts++
					w.WriteHeader(http.StatusServiceUnavailable)
				case r.URL.Path == "/repos/so/sr/pulls/5/comments" && tc.comments != "":
					_, _ = w.Write([]byte(tc.comments))
				default:
					w.WriteHeader(http.StatusInternalServerError)
				}
			})
			app.login = tc.login
			app.pendingComments = subDrafts()

			_, err := app.SubmitReview(shown(t, app, model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "comments"}}}))
			require.ErrorContains(t, err, "so/sr#5")
			assert.Equal(t, 1, posts)
			assert.Equal(t, subDrafts(), app.pendingComments)
			assert.Empty(t, app.subCommentURLs)
		})
	}
}

func TestSubmitReview_RangeAnswers5xxWithLookAlikeStartingOnTheOtherSide_KeepsDraftAndReturnsError(t *testing.T) {
	const lookAlike = `"path":"a.txt","start_line":1,"start_side":"RIGHT","line":2,"side":"RIGHT","body":"range"`
	for _, mode := range []string{"review", "comments"} {
		t.Run(mode, func(t *testing.T) {
			app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPost:
					w.WriteHeader(http.StatusBadGateway)
				case r.URL.Path == "/repos/so/sr/pulls/5/reviews":
					_, _ = w.Write([]byte(`[{"id":7,"user":{"login":"me"},"state":"COMMENTED","commit_id":"new","body":"","submitted_at":"` + stamp(0) + `"}]`))
				case r.URL.Path == "/repos/so/sr/pulls/5/reviews/7/comments":
					_, _ = w.Write([]byte(`[{` + lookAlike + `}]`))
				case r.URL.Path == "/repos/so/sr/pulls/5/comments":
					_, _ = w.Write([]byte(`[{"id":100,"user":{"login":"me"},"commit_id":"new","created_at":"` + stamp(0) + `",` + lookAlike + `}]`))
				default:
					w.WriteHeader(http.StatusTeapot)
				}
			})
			drafts := []model.DraftCommentDTO{{Submodule: "lib", Bump: "..new", Path: "lib/a.txt", StartLine: 1, StartSide: "LEFT", Line: 2, Side: "RIGHT", Body: "range"}}
			app.pendingComments = slices.Clone(drafts)

			_, err := app.SubmitReview(shown(t, app, model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: mode, Verdict: "COMMENT"}}}))

			require.ErrorContains(t, err, "so/sr#5")
			assert.Equal(t, drafts, app.pendingComments)
		})
	}
}

func TestPostImmediateComment_FoldedCommentAnswers5xxWithOnlyAnOlderLookAlike_ReturnsError(t *testing.T) {
	body, err := json.Marshal("**Comments on submodule files**\n\n- `lib/b.txt` (submodule lib) L1 — hm")
	require.NoError(t, err)
	posts := 0
	app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts++
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`[{"id":3,"user":{"login":"me"},"created_at":"` + stamp(-10*time.Minute) + `","body":` + string(body) + `}]`))
	})

	_, err = app.PostImmediateComment(model.DraftCommentDTO{Submodule: "lib", Bump: "..new", Path: "lib/b.txt", Line: 1, Side: "RIGHT", Body: "hm"})
	require.ErrorContains(t, err, "github: post comment")
	assert.Equal(t, 1, posts)
}

func TestSubmitReview_TwoIdenticalDraftsSecondAnswers5xx_DoesNotAdoptTheFirstOnesComment(t *testing.T) {
	posts := 0
	app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/repos/so/sr/pulls/5/comments":
			posts++
			if posts == 2 {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			_, _ = w.Write([]byte(`{"id":100,"html_url":"https://sub/first"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/repos/so/sr/pulls/5/comments":
			_, _ = w.Write([]byte(`[{"id":100,"user":{"login":"me"},"path":"a.txt","body":"same","commit_id":"new","line":2,"side":"RIGHT","created_at":"` + stamp(0) + `","html_url":"https://sub/first"}]`))
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	})
	same := model.DraftCommentDTO{Submodule: "lib", Bump: "..new", Path: "lib/a.txt", Line: 2, Side: "RIGHT", Body: "same"}
	app.pendingComments = []model.DraftCommentDTO{same, same}

	_, err := app.SubmitReview(shown(t, app, model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "comments"}}}))
	require.ErrorContains(t, err, "so/sr#5")
	assert.Equal(t, 2, posts)
	assert.Equal(t, []model.DraftCommentDTO{same}, app.pendingComments)
	assert.Equal(t, []string{"https://sub/first"}, app.subCommentURLs)
}

func TestSubmitReview_LookupAfter5xxFails_NextSubmitLooksAgainBeforeResending(t *testing.T) {
	const posted = `[{"id":100,"user":{"login":"me"},"path":"a.txt","body":"in PR diff","commit_id":"new","line":2,"side":"RIGHT","html_url":"https://sub/first","created_at":"`
	for name, tc := range map[string]struct {
		second string
		posts  int
		url    string
	}{
		"it went through":    {posted + stamp(0) + `"}]`, 1, "https://sub/first"},
		"it did not":         {`[]`, 2, "https://sub/resent"},
		"lookup fails again": {"", 1, ""},
	} {
		t.Run(name, func(t *testing.T) {
			posts, lookups := 0, 0
			app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/repos/so/sr/pulls/5/comments":
					posts++
					if posts == 1 {
						w.WriteHeader(http.StatusBadGateway)
						return
					}
					_, _ = w.Write([]byte(`{"id":101,"html_url":"https://sub/resent"}`))
				case r.Method == http.MethodGet && r.URL.Path == "/repos/so/sr/pulls/5/comments":
					lookups++
					if lookups == 1 || tc.second == "" {
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					_, _ = w.Write([]byte(tc.second))
				default:
					w.WriteHeader(http.StatusTeapot)
				}
			})
			app.pendingComments = subDrafts()[:1]
			req := model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "comments"}}}

			_, err := app.SubmitReview(shown(t, app, req))
			require.ErrorContains(t, err, "so/sr#5")
			res, err := app.SubmitReview(shown(t, app, req))

			assert.Equal(t, tc.posts, posts)
			if tc.url == "" {
				require.ErrorContains(t, err, "earlier attempt")
				assert.Equal(t, subDrafts()[:1], app.pendingComments)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, []string{tc.url}, res.SubmoduleCommentURLs)
			assert.Empty(t, app.uncertain)
		})
	}
}

func TestSubmitReview_UncertainCommentThenAnotherChoice_ConflictsWithoutReadingOrSending(t *testing.T) {
	var after []string
	failed := false
	app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
		if failed {
			after = append(after, r.Method+" "+r.URL.Path)
		}
		w.WriteHeader(http.StatusBadGateway)
	})
	app.pendingComments = subDrafts()[:1]
	_, err := app.SubmitReview(shown(t, app, model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "comments"}}}))
	require.ErrorContains(t, err, "so/sr#5")
	require.Len(t, app.uncertain, 1)
	failed = true

	for _, c := range []model.SubmoduleReviewChoice{{Path: "lib", Mode: "fold"}, {Path: "lib", Mode: "review", Verdict: "COMMENT"}} {
		_, err = app.SubmitReview(shown(t, app, model.ReviewSubmitRequest{Verdict: "COMMENT", Body: "x", Submodules: []model.SubmoduleReviewChoice{c}}))
		require.ErrorContains(t, err, "conflict: this comment's earlier submit", c.Mode)
	}

	assert.Empty(t, after, "nothing is read or sent")
	assert.Equal(t, subDrafts()[:1], app.pendingComments)
	got, err := app.SubmoduleDrafts(false)
	require.NoError(t, err)
	assert.Equal(t, "comments", got[0].InFlight, "the dialog shows the choice in flight")
}

func TestSubmitReview_FileReviewSubmitAndLookupFail_NextSubmitReadsThatReviewFirst(t *testing.T) {
	dests := map[string]struct {
		prefix string
		draft  model.DraftCommentDTO
		req    model.ReviewSubmitRequest
	}{
		"parent PR":    {"/repos/o/r/pulls/1", model.DraftCommentDTO{Path: "a.go", Side: "RIGHT", Body: "file"}, model.ReviewSubmitRequest{Verdict: "COMMENT"}},
		"submodule PR": {"/repos/so/sr/pulls/5", model.DraftCommentDTO{Submodule: "lib", Bump: "..new", Path: "lib/a.txt", Body: "file"}, model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "review", Verdict: "COMMENT"}}}},
	}
	states := map[string]struct {
		status                    int
		review                    string
		creates, submits, deletes int
		wantErr                   string
	}{
		"it was submitted":                        {status: http.StatusOK, review: `{"id":10,"state":"COMMENTED","html_url":"https://x/review"}`, creates: 1, submits: 1},
		"it is still pending":                     {status: http.StatusOK, review: `{"id":10,"state":"PENDING"}`, creates: 2, submits: 2, deletes: 1},
		"it is gone":                              {status: http.StatusNotFound, creates: 2, submits: 2},
		"lookup fails again":                      {status: http.StatusInternalServerError, creates: 1, submits: 1, wantErr: "earlier attempt"},
		"it reads back empty":                     {status: http.StatusOK, review: `{}`, creates: 1, submits: 1, wantErr: "earlier attempt"},
		"it reads back as another review":         {status: http.StatusOK, review: `{"id":11,"state":"COMMENTED"}`, creates: 1, submits: 1, wantErr: "earlier attempt"},
		"it reads back as another pending review": {status: http.StatusOK, review: `{"id":11,"state":"PENDING"}`, creates: 1, submits: 1, wantErr: "earlier attempt"},
	}
	for dname, dest := range dests {
		for sname, tc := range states {
			t.Run(dname+"/"+sname, func(t *testing.T) {
				fakeGraphQL(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"data":{}}`)) })
				creates, submits, lookups, deletes := 0, 0, 0, 0
				app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
					switch {
					case r.Method == http.MethodDelete:
						deletes++
					case r.URL.Path == dest.prefix+"/reviews" && r.Method == http.MethodGet:
						_, _ = w.Write([]byte(`[]`))
					case r.URL.Path == dest.prefix+"/reviews":
						creates++
						_, _ = w.Write([]byte(`{"id":10,"node_id":"PRR_10"}`))
					case r.URL.Path == dest.prefix+"/reviews/10/events":
						if submits++; submits == 1 {
							w.WriteHeader(http.StatusBadGateway)
							return
						}
						_, _ = w.Write([]byte(`{"id":10,"state":"COMMENTED","html_url":"https://x/review"}`))
					case r.URL.Path == dest.prefix+"/reviews/10":
						if lookups++; lookups == 1 {
							w.WriteHeader(http.StatusInternalServerError)
							return
						}
						w.WriteHeader(tc.status)
						_, _ = w.Write([]byte(tc.review))
					default:
						w.WriteHeader(http.StatusTeapot)
					}
				})
				app.pendingComments = []model.DraftCommentDTO{dest.draft}

				_, err := app.SubmitReview(shown(t, app, dest.req))
				require.Error(t, err)
				res, err := app.SubmitReview(shown(t, app, dest.req))

				assert.Equal(t, []int{tc.creates, tc.submits, tc.deletes}, []int{creates, submits, deletes}, "creates, submits, deletes: only one read still pending is deleted, to be rebuilt")
				if tc.wantErr != "" {
					require.ErrorContains(t, err, tc.wantErr)
					assert.Len(t, app.pendingComments, 1)
					return
				}
				require.NoError(t, err)
				assert.Contains(t, append(res.SubmoduleReviewURLs, res.HTMLURL), "https://x/review")
				assert.Empty(t, app.pendingComments)
			})
		}
	}
}

func TestUpdateAndDeleteDraftComment_LastSendUncertain_RejectedUntilASubmitSettlesIt(t *testing.T) {
	const posted = `[{"id":100,"user":{"login":"me"},"path":"a.txt","body":"in PR diff","commit_id":"new","line":2,"side":"RIGHT","html_url":"https://sub/first","created_at":"`
	posts, lookups := 0, 0
	app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/repos/so/sr/pulls/5/comments":
			posts++
			w.WriteHeader(http.StatusBadGateway)
		case r.Method == http.MethodGet && r.URL.Path == "/repos/so/sr/pulls/5/comments":
			if lookups++; lookups == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_, _ = w.Write([]byte(posted + stamp(0) + `"}]`))
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	})
	_, err := app.AddDraftComment(subDrafts()[0])
	require.NoError(t, err)
	req := model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "comments"}}}
	_, err = app.SubmitReview(shown(t, app, req))
	require.ErrorContains(t, err, "so/sr#5")

	_, err = app.UpdateDraftComment(0, "in PR diff", "edited")
	require.ErrorContains(t, err, "conflict: this comment's earlier submit")
	_, err = app.DeleteDraftComment(0, "in PR diff")
	require.ErrorContains(t, err, "conflict: this comment's earlier submit")
	res, err := app.SubmitReview(shown(t, app, req))

	require.NoError(t, err)
	assert.Equal(t, 1, posts, "the draft is not posted a second time")
	assert.Equal(t, []string{"https://sub/first"}, res.SubmoduleCommentURLs)
	assert.Empty(t, app.pendingComments)
}

func TestSubmitReview_UncertainSubmoduleFileReviewStillPending_RebuiltOnlyWithTheSameVerdictAndText(t *testing.T) {
	review := model.SubmoduleReviewChoice{Path: "lib", Mode: "review", Verdict: "COMMENT", Body: "old text"}
	for name, tc := range map[string]struct {
		choice                    model.SubmoduleReviewChoice
		closed                    bool
		creates, submits, deletes int
		wantErr                   string
	}{
		"review again":         {choice: review, creates: 2, submits: 2, deletes: 1},
		"another verdict":      {choice: model.SubmoduleReviewChoice{Path: "lib", Mode: "review", Verdict: "APPROVE", Body: "old text"}, creates: 1, submits: 1, wantErr: "conflict: this comment's earlier submit"},
		"other text":           {choice: model.SubmoduleReviewChoice{Path: "lib", Mode: "review", Verdict: "COMMENT", Body: "new text"}, creates: 1, submits: 1, wantErr: "conflict: this comment's earlier submit"},
		"single comments now":  {choice: model.SubmoduleReviewChoice{Path: "lib", Mode: "comments"}, creates: 1, submits: 1, wantErr: "conflict: this comment's earlier submit"},
		"fold now":             {choice: model.SubmoduleReviewChoice{Path: "lib", Mode: "fold"}, creates: 1, submits: 1, wantErr: "conflict: this comment's earlier submit"},
		"PR closed since then": {choice: review, closed: true, creates: 1, submits: 1, deletes: 1, wantErr: "conflict: so/sr#5"},
	} {
		t.Run(name, func(t *testing.T) {
			fakeGraphQL(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"data":{}}`)) })
			var submitted []map[string]any
			creates, deletes, lookups, closed := 0, 0, 0, false
			app := subAppRaw(t, func(w http.ResponseWriter, r *http.Request) {
				p := r.URL.Path
				switch {
				case r.Method == http.MethodGet && p == "/repos/so/sr/pulls/5":
					_, _ = w.Write([]byte(`{"state":"` + map[bool]string{false: "open", true: "closed"}[closed] + `","head":{"sha":"new"}}`))
				case r.Method == http.MethodDelete:
					deletes++
				case r.Method == http.MethodGet && p == "/repos/so/sr/pulls/5/reviews":
					_, _ = w.Write([]byte(`[]`))
				case p == "/repos/so/sr/pulls/5/reviews":
					creates++
					_, _ = w.Write([]byte(`{"id":10,"node_id":"PRR_10"}`))
				case p == "/repos/so/sr/pulls/5/reviews/10/events":
					var body map[string]any
					if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					if submitted = append(submitted, body); len(submitted) == 1 {
						w.WriteHeader(http.StatusBadGateway)
						return
					}
					_, _ = w.Write([]byte(`{"id":10,"state":"APPROVED","html_url":"https://x/review"}`))
				case p == "/repos/so/sr/pulls/5/reviews/10":
					if lookups++; lookups == 1 {
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					_, _ = w.Write([]byte(`{"id":10,"state":"PENDING"}`))
				default:
					w.WriteHeader(http.StatusTeapot)
				}
			})
			app.pendingComments = []model.DraftCommentDTO{{Submodule: "lib", Bump: "..new", Path: "lib/a.txt", Body: "file"}}
			first := model.SubmoduleReviewChoice{Path: "lib", Mode: "review", Verdict: "COMMENT", Body: "old text"}
			_, err := app.SubmitReview(shown(t, app, model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{first}}))
			require.Error(t, err)
			drafts, err := app.SubmoduleDrafts(false)
			require.NoError(t, err)
			assert.Equal(t, &model.InFlightReviewDTO{Verdict: "COMMENT", Body: "old text"}, drafts[0].InFlightReview, "the dialog shows what was sent")

			closed = tc.closed
			res, err := app.SubmitReview(shown(t, app, model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{tc.choice}}))

			assert.Equal(t, []int{tc.creates, tc.submits, tc.deletes}, []int{creates, len(submitted), deletes}, "creates, submits, deletes")
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				assert.Len(t, app.pendingComments, 1)
				assert.Empty(t, app.orphans, "the pending review on the closed PR is deleted, not left blocking one on github.com")
				return
			}
			require.NoError(t, err)
			assert.Empty(t, app.pendingComments)
			assert.Equal(t, map[string]any{"event": "COMMENT", "body": "old text"}, submitted[1])
			assert.Equal(t, []string{"https://x/review"}, res.SubmoduleReviewURLs)
		})
	}
}

func TestSubmitReview_ParentFileReviewStillPendingAfterUncertainSubmit_OtherVerdictRefusedThenRebuiltWithTheSame(t *testing.T) {
	fakeGraphQL(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"data":{}}`)) })
	var submitted []map[string]any
	deleted, lookups := 0, 0
	app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case r.Method == http.MethodDelete && p == "/repos/o/r/pulls/1/reviews/10":
			deleted++
		case r.Method == http.MethodGet && p == "/repos/o/r/pulls/1/reviews":
			_, _ = w.Write([]byte(`[]`))
		case p == "/repos/o/r/pulls/1/reviews":
			_, _ = w.Write([]byte(`{"id":10,"node_id":"PRR_10"}`))
		case p == "/repos/o/r/pulls/1/reviews/10/events":
			var body map[string]any
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if submitted = append(submitted, body); len(submitted) == 1 {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			_, _ = w.Write([]byte(`{"id":10,"state":"APPROVED","html_url":"https://x/review"}`))
		case p == "/repos/o/r/pulls/1/reviews/10":
			if lookups++; lookups == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_, _ = w.Write([]byte(`{"id":10,"state":"PENDING"}`))
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	})
	app.pendingComments = []model.DraftCommentDTO{{Path: "a.go", Side: "RIGHT", Body: "file"}}
	first := model.ReviewSubmitRequest{Verdict: "COMMENT", Body: "first"}
	_, err := app.SubmitReview(first)
	require.Error(t, err)
	state, err := app.SyncPendingReview()
	require.NoError(t, err)
	assert.Equal(t, &model.InFlightReviewDTO{Verdict: "COMMENT", Body: "first"}, state.InFlightReview, "the dialog shows what was sent")

	for _, other := range []model.ReviewSubmitRequest{{Verdict: "APPROVE", Body: "first"}, {Verdict: "COMMENT", Body: "second"}} {
		_, err = app.SubmitReview(other)
		require.ErrorContains(t, err, "conflict: this comment's earlier submit")
	}
	require.Len(t, submitted, 1, "a refused verdict or text sends nothing")
	res, err := app.SubmitReview(first)

	require.NoError(t, err)
	assert.Equal(t, 1, deleted)
	require.Len(t, submitted, 2)
	assert.Equal(t, map[string]any{"event": "COMMENT", "body": "first"}, submitted[1])
	assert.Equal(t, "https://x/review", res.HTMLURL)
	assert.Nil(t, app.pendingReviewState().InFlightReview, "settled, everything unlocks")
}

func TestSubmitReview_RetryWhileSecondSubmodulePRClosed_WritesNothingAndConflicts(t *testing.T) {
	fakeGraphQL(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"data":{}}`)) })
	var writes []string
	retry, lookups := false, 0
	app := subAppRaw(t, func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if retry && r.Method != http.MethodGet {
			writes = append(writes, r.Method+" "+p)
		}
		switch {
		case p == "/repos/so/sr/pulls/5":
			_, _ = w.Write([]byte(`{"state":"open","head":{"sha":"new"}}`))
		case p == "/repos/so/sr2/pulls/6":
			_, _ = w.Write([]byte(`{"state":"` + map[bool]string{false: "open", true: "closed"}[retry] + `","head":{"sha":"new"}}`))
		case r.Method == http.MethodGet && p == "/repos/so/sr/pulls/5/reviews":
			_, _ = w.Write([]byte(`[]`))
		case p == "/repos/so/sr/pulls/5/reviews":
			_, _ = w.Write([]byte(`{"id":10,"node_id":"PRR_10"}`))
		case p == "/repos/so/sr/pulls/5/reviews/10/events":
			w.WriteHeader(http.StatusBadGateway)
		case p == "/repos/so/sr/pulls/5/reviews/10":
			if lookups++; lookups == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_, _ = w.Write([]byte(`{"id":10,"state":"PENDING"}`))
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	})
	addSub(app, "lib2", "so", "sr2", 6)
	app.pendingComments = []model.DraftCommentDTO{{Submodule: "lib", Bump: "..new", Path: "lib/a.txt", Body: "one"}, {Submodule: "lib2", Bump: "..new", Path: "lib2/a.txt", Body: "two"}}
	req := model.ReviewSubmitRequest{Verdict: "COMMENT", Body: "x", Submodules: []model.SubmoduleReviewChoice{
		{Path: "lib", Mode: "review", Verdict: "COMMENT"}, {Path: "lib2", Mode: "review", Verdict: "COMMENT"},
	}}
	_, err := app.SubmitReview(shown(t, app, req))
	require.ErrorContains(t, err, "so/sr#5")

	retry = true
	_, err = app.SubmitReview(shown(t, app, req))

	require.ErrorContains(t, err, "conflict: so/sr2#6")
	assert.Empty(t, writes, "the still-pending review on so/sr#5 is not published before the check of so/sr2#6")
	assert.Len(t, app.pendingComments, 2)
}

func TestSubmitReview_OwnPendingReviewOnRetargetedSubmodulePR_DeletedFirstOrKeptWhenThatFails(t *testing.T) {
	for name, deleteStatus := range map[string]int{"delete works": http.StatusOK, "delete fails": http.StatusBadGateway} {
		t.Run(name, func(t *testing.T) {
			var writes []string
			app := subAppRaw(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodDelete:
					writes = append(writes, r.URL.Path)
					w.WriteHeader(deleteStatus)
				case r.URL.Path == "/repos/so/sr/pulls/5":
					_, _ = w.Write([]byte(`{"number":5,"state":"open","head":{"sha":"new"},"base":{"sha":"b2"}}`))
				case r.URL.Path == "/repos/so/sr/pulls/5/files":
					_, _ = w.Write([]byte(`[{"filename":"a.txt","patch":"` + strings.ReplaceAll(subPatch, "\n", `\n`) + `"}]`))
				default:
					writes = append(writes, r.Method+" "+r.URL.Path)
					w.WriteHeader(http.StatusTeapot)
				}
			})
			app.prSubmodules["lib"].pr.Base = &github.PullRequestBranch{SHA: new("b1")}
			app.orphans = []orphanReview{{"so", "sr", 5, 10}}
			app.pendingComments = subDrafts()[:1]

			_, err := app.SubmitReview(shown(t, app, model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "review", Verdict: "COMMENT"}}}))

			require.ErrorContains(t, err, "conflict: so/sr#5")
			assert.Equal(t, []string{"/repos/so/sr/pulls/5/reviews/10"}, writes, "only our pending review is deleted")
			if deleteStatus == http.StatusOK {
				assert.Empty(t, app.orphans)
				return
			}
			assert.ErrorContains(t, err, "could not be deleted")
			assert.Equal(t, []orphanReview{{"so", "sr", 5, 10}}, app.orphans)
		})
	}
}

func TestSubmitReview_UncertainSubmoduleCommentThenParentHeadMoved_FoldAllowedAfterSettlingIt(t *testing.T) {
	const posted = `[{"id":100,"user":{"login":"me"},"path":"a.txt","body":"in PR diff","commit_id":"new","line":2,"side":"RIGHT","html_url":"https://sub/first","created_at":"`
	for name, tc := range map[string]struct {
		lookup     string
		wantFolded bool
	}{
		"it went through": {posted + stamp(0) + `"}]`, false},
		"it did not":      {`[]`, true},
	} {
		t.Run(name, func(t *testing.T) {
			var parentBodies []string
			commentPosts, lookups := 0, 0
			app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/repos/so/sr/pulls/5/comments":
					commentPosts++
					w.WriteHeader(http.StatusBadGateway)
				case r.Method == http.MethodGet && r.URL.Path == "/repos/so/sr/pulls/5/comments":
					if lookups++; lookups == 1 {
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					_, _ = w.Write([]byte(tc.lookup))
				case r.Method == http.MethodPost && r.URL.Path == "/repos/o/r/pulls/1/reviews":
					var body map[string]any
					if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					parentBodies = append(parentBodies, body["body"].(string))
					_, _ = w.Write([]byte(`{"id":9,"html_url":"https://parent/review"}`))
				default:
					w.WriteHeader(http.StatusTeapot)
				}
			})
			app.pendingComments, app.pendingHeadSHA = subDrafts()[:1], "sha"
			_, err := app.SubmitReview(shown(t, app, model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "comments"}}}))
			require.ErrorContains(t, err, "so/sr#5")

			app.prCache.HeadSHA, app.prSubmodules["lib"].from = "moved", "other"
			got, err := app.SubmoduleDrafts(false)
			require.NoError(t, err)
			assert.Equal(t, []model.SubmoduleDraftsDTO{{Path: "lib", PR: "so/sr#5", Folded: 1, Outdated: 1}}, got, "no longer held: the draft can't go to so/sr#5")
			res, err := app.SubmitReview(shown(t, app, model.ReviewSubmitRequest{Verdict: "COMMENT", Body: "x", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "fold"}}}))

			require.NoError(t, err)
			assert.Equal(t, 1, commentPosts)
			require.Len(t, parentBodies, 1)
			assert.Equal(t, tc.wantFolded, strings.Contains(parentBodies[0], "in PR diff"), "folded only when the earlier send did not go through")
			if !tc.wantFolded {
				assert.Equal(t, []string{"https://sub/first"}, res.SubmoduleCommentURLs)
			}
			assert.Empty(t, app.pendingComments)
		})
	}
}

func TestSubmitReview_ParentReviewAnswers5xxAndLookupFails_FrozenThenSettledBeforeResending(t *testing.T) {
	const comments = `[{"path":"a.go","line":3,"side":"RIGHT","body":"parent"}]`
	published := `[{"id":9,"user":{"login":"me"},"state":"COMMENTED","commit_id":"sha","body":"x","html_url":"https://parent/first","submitted_at":"` + stamp(0) + `"}]`
	for name, tc := range map[string]struct {
		reviews string
		posts   int
		url     string
	}{
		"it went through": {published, 1, "https://parent/first"},
		"it did not":      {`[]`, 2, "https://parent/resent"},
	} {
		t.Run(name, func(t *testing.T) {
			posts, lookups := 0, 0
			app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/repos/o/r/pulls/1/reviews":
					if posts++; posts == 1 {
						w.WriteHeader(http.StatusBadGateway)
						return
					}
					_, _ = w.Write([]byte(`{"id":10,"html_url":"https://parent/resent"}`))
				case r.Method == http.MethodGet && r.URL.Path == "/repos/o/r/pulls/1/reviews":
					if lookups++; lookups == 1 {
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					_, _ = w.Write([]byte(tc.reviews))
				case r.URL.Path == "/repos/o/r/pulls/1/reviews/9/comments":
					_, _ = w.Write([]byte(comments))
				default:
					w.WriteHeader(http.StatusTeapot)
				}
			})
			app.pendingComments = []model.DraftCommentDTO{{Path: "a.go", Line: 3, Side: "RIGHT", Body: "parent"}}
			req := model.ReviewSubmitRequest{Verdict: "COMMENT", Body: "x"}
			_, err := app.SubmitReview(req)
			require.ErrorContains(t, err, "github: submit review")
			_, err = app.UpdateDraftComment(0, "parent", "edited")
			require.ErrorContains(t, err, "conflict: this comment's earlier submit")

			res, err := app.SubmitReview(req)

			require.NoError(t, err)
			assert.Equal(t, tc.posts, posts)
			assert.Equal(t, tc.url, res.HTMLURL)
			assert.Empty(t, app.pendingComments)
		})
	}
}

// pendingCreateApp is subApp whose first pending review create on so/sr#5 answers 502 yet leaves pending review
// 9, and whose first look for it after that fails; gql answers GraphQL, deleted records the reviews deleted.
func pendingCreateApp(t *testing.T, gql http.HandlerFunc, deleted *[]string, creates *int) *App {
	t.Helper()
	fakeGraphQL(t, gql)
	left, lists := false, 0
	return subApp(t, func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case r.Method == http.MethodPost && p == "/repos/so/sr/pulls/5/reviews":
			if *creates++; *creates == 1 {
				left = true
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			_, _ = w.Write([]byte(`{"id":10,"node_id":"PRR_10"}`))
		case r.Method == http.MethodGet && p == "/repos/so/sr/pulls/5/reviews":
			if lists++; lists == 2 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if !left {
				_, _ = w.Write([]byte(`[]`))
				return
			}
			_, _ = w.Write([]byte(`[{"id":9,"state":"PENDING","commit_id":"new"}]`))
		case p == "/repos/so/sr/pulls/5/reviews/9/comments":
			_, _ = w.Write([]byte(`[{"path":"a.txt","body":"in PR diff"}]`))
		case p == "/repos/so/sr/pulls/5/reviews/9":
			if r.Method == http.MethodDelete {
				*deleted, left = append(*deleted, p), false
			}
			_, _ = w.Write([]byte(`{"id":9,"state":"PENDING"}`))
		case p == "/repos/so/sr/pulls/5/reviews/10/events":
			_, _ = w.Write([]byte(`{"id":10,"state":"COMMENTED","html_url":"https://sub/review"}`))
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	})
}

// pendingThreadOn answers GraphQL with pending review 9's thread on line of a.txt, in fileReviewDrafts' words.
func pendingThreadOn(line int) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `{"data":{"repository":{"pullRequest":{"reviewThreads":{"pageInfo":{"hasNextPage":false},"nodes":[
			{"diffSide":"RIGHT","originalLine":%d,"subjectType":"LINE",
			 "comments":{"nodes":[{"body":"in PR diff","path":"a.txt","pullRequestReview":{"databaseId":9}}]}}]}}}}}`, line)
	}
}

// fileReviewDrafts are a line and a file-level draft on so/sr#5's a.txt, which go there as a pending review.
func fileReviewDrafts() []model.DraftCommentDTO {
	return []model.DraftCommentDTO{
		{ID: 1, Submodule: "lib", Bump: "..new", Path: "lib/a.txt", Line: 2, Side: "RIGHT", Body: "in PR diff"},
		{ID: 2, Submodule: "lib", Bump: "..new", Path: "lib/a.txt", Body: "whole file"},
	}
}

func TestSubmitReview_PendingSubmoduleCreateAnd5xxLookupFail_NextSubmitDeletesWhatItLeftAndRebuilds(t *testing.T) {
	var deleted []string
	creates := 0
	app := pendingCreateApp(t, pendingThreadOn(2), &deleted, &creates)
	app.pendingComments = fileReviewDrafts()
	req := model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "review", Verdict: "COMMENT"}}}
	_, err := app.SubmitReview(shown(t, app, req))
	require.ErrorContains(t, err, "so/sr#5")
	require.NotEmpty(t, app.uncertain, "the attempt is kept")

	res, err := app.SubmitReview(shown(t, app, req))

	require.NoError(t, err)
	assert.Equal(t, []string{"/repos/so/sr/pulls/5/reviews/9"}, deleted)
	assert.Equal(t, 2, creates)
	assert.Equal(t, []string{"https://sub/review"}, res.SubmoduleReviewURLs)
	assert.Empty(t, app.orphans)
}

func TestDiscardPendingReview_PendingSubmoduleCreateAnd5xxLookupFail_DeletesWhatItLeft(t *testing.T) {
	var deleted []string
	creates := 0
	app := pendingCreateApp(t, pendingThreadOn(2), &deleted, &creates)
	app.pendingComments = fileReviewDrafts()
	req := model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "review", Verdict: "COMMENT"}}}
	_, err := app.SubmitReview(shown(t, app, req))
	require.ErrorContains(t, err, "so/sr#5")

	require.NoError(t, app.DiscardPendingReview())

	assert.Equal(t, []string{"/repos/so/sr/pulls/5/reviews/9"}, deleted)
	assert.Empty(t, app.pendingComments)
	assert.Empty(t, app.rememberedPending())
}

func TestDiscardPendingReview_PendingReviewWithTheSameWordsOnAnotherLine_LeftAlone(t *testing.T) {
	var deleted []string
	creates := 0
	app := pendingCreateApp(t, pendingThreadOn(3), &deleted, &creates)
	app.pendingComments = fileReviewDrafts()
	req := model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "review", Verdict: "COMMENT"}}}
	_, err := app.SubmitReview(shown(t, app, req))
	require.ErrorContains(t, err, "so/sr#5")

	require.NoError(t, app.DiscardPendingReview())

	assert.Empty(t, deleted)
	assert.Empty(t, app.rememberedPending())
}

func TestDiscardPendingReview_LeftPendingReviewsThreadsUnreadable_KeepsItUnresolved(t *testing.T) {
	var deleted []string
	creates := 0
	app := pendingCreateApp(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) }, &deleted, &creates)
	app.pendingComments = fileReviewDrafts()
	req := model.ReviewSubmitRequest{Verdict: "COMMENT", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "review", Verdict: "COMMENT"}}}
	_, err := app.SubmitReview(shown(t, app, req))
	require.ErrorContains(t, err, "so/sr#5")

	require.ErrorContains(t, app.DiscardPendingReview(), "earlier attempt")
	_, err = app.SubmitReview(shown(t, app, req))

	require.ErrorContains(t, err, "earlier attempt")
	assert.Empty(t, deleted)
	assert.Equal(t, 1, creates)
	assert.NotEmpty(t, app.uncertain)
	assert.Equal(t, fileReviewDrafts(), app.pendingComments)
}

func TestSubmitReview_SubmoduleCommentAnswers2xxWithTruncatedBody_LooksItUpInsteadOfFailing(t *testing.T) {
	posts := 0
	app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/repos/so/sr/pulls/5/comments":
			posts++
			w.Header().Set("Content-Length", "100")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":100,`))
		case r.Method == http.MethodGet && r.URL.Path == "/repos/so/sr/pulls/5/comments":
			_, _ = w.Write([]byte(`[{"id":100,"user":{"login":"me"},"path":"a.txt","body":"in PR diff","commit_id":"new","line":2,"side":"RIGHT","created_at":"` + stamp(0) + `","html_url":"https://sub/comment"}]`))
		case r.URL.Path == "/repos/o/r/pulls/1/reviews":
			_, _ = w.Write([]byte(`{"id":9,"html_url":"https://parent/review"}`))
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

func TestSubmitReview_ParentReviewWithoutDraftsAnswers5xxAndLookupFails_LookedUpBeforeResending(t *testing.T) {
	published := `[{"id":9,"user":{"login":"me"},"state":"APPROVED","commit_id":"sha","body":"LGTM","html_url":"https://parent/first","submitted_at":"` + stamp(0) + `"}]`
	for name, tc := range map[string]struct {
		reviews string
		posts   int
		url     string
	}{
		"it went through":    {published, 1, "https://parent/first"},
		"it did not":         {`[]`, 2, "https://parent/resent"},
		"lookup fails again": {"", 1, ""},
	} {
		t.Run(name, func(t *testing.T) {
			posts, lookups := 0, 0
			app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/repos/so/sr/pulls/5/comments":
					_, _ = w.Write([]byte(`{"id":100,"html_url":"https://sub/comment"}`))
				case r.Method == http.MethodPost && r.URL.Path == "/repos/o/r/pulls/1/reviews":
					if posts++; posts == 1 {
						w.WriteHeader(http.StatusBadGateway)
						return
					}
					_, _ = w.Write([]byte(`{"id":10,"html_url":"https://parent/resent"}`))
				case r.Method == http.MethodGet && r.URL.Path == "/repos/o/r/pulls/1/reviews":
					if lookups++; lookups == 1 || tc.reviews == "" {
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					_, _ = w.Write([]byte(tc.reviews))
				case r.URL.Path == "/repos/o/r/pulls/1/reviews/9/comments":
					_, _ = w.Write([]byte(`[]`))
				default:
					w.WriteHeader(http.StatusTeapot)
				}
			})
			app.pendingComments = subDrafts()[:1]
			req := model.ReviewSubmitRequest{Verdict: "APPROVE", Body: "LGTM", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "comments"}}}
			_, err := app.SubmitReview(shown(t, app, req))
			require.ErrorContains(t, err, "github: submit review")
			require.Empty(t, app.pendingComments, "every draft went to the submodule PR")

			res, err := app.SubmitReview(shown(t, app, req))

			assert.Equal(t, tc.posts, posts)
			if tc.url == "" {
				require.ErrorContains(t, err, "earlier attempt")
				assert.NotNil(t, app.parentAttempt)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.url, res.HTMLURL)
			assert.Equal(t, []string{"https://sub/comment"}, res.SubmoduleCommentURLs)
			assert.Nil(t, app.parentAttempt)
		})
	}
}

func TestSubmitReview_ParentReviewWithFoldedDraftsConfirmedOnRetry_ReportsTheFoldedCount(t *testing.T) {
	var sentBody json.RawMessage
	posts, lookups := 0, 0
	app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/repos/o/r/pulls/1/reviews":
			posts++
			var body struct{ Body json.RawMessage }
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			sentBody = body.Body
			w.WriteHeader(http.StatusBadGateway)
		case r.Method == http.MethodGet && r.URL.Path == "/repos/o/r/pulls/1/reviews":
			if lookups++; lookups == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_, _ = w.Write([]byte(`[{"id":9,"user":{"login":"me"},"state":"COMMENTED","commit_id":"sha","body":` + string(sentBody) + `,"html_url":"https://parent/first","submitted_at":"` + stamp(0) + `"}]`))
		case r.URL.Path == "/repos/o/r/pulls/1/reviews/9/comments":
			_, _ = w.Write([]byte(`[]`))
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	})
	app.pendingComments = subDrafts()[1:]
	req := model.ReviewSubmitRequest{Verdict: "COMMENT", Body: "x", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "fold"}}}
	_, err := app.SubmitReview(shown(t, app, req))
	require.ErrorContains(t, err, "github: submit review")

	res, err := app.SubmitReview(shown(t, app, req))

	require.NoError(t, err)
	assert.Equal(t, 1, posts)
	assert.Equal(t, "https://parent/first", res.HTMLURL)
	assert.Equal(t, 1, res.FoldedComments)
}

func TestDiscardPendingReview_ParentReviewWithoutDraftsUncertain_ForgetsTheAttempt(t *testing.T) {
	app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/repos/so/sr/pulls/5/comments":
			_, _ = w.Write([]byte(`{"id":100,"html_url":"https://sub/comment"}`))
		default:
			w.WriteHeader(http.StatusBadGateway)
		}
	})
	app.pendingComments = subDrafts()[:1]
	_, err := app.SubmitReview(shown(t, app, model.ReviewSubmitRequest{Verdict: "APPROVE", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "comments"}}}))
	require.ErrorContains(t, err, "github: submit review")
	require.NotNil(t, app.parentAttempt)

	require.NoError(t, app.DiscardPendingReview())

	assert.Nil(t, app.parentAttempt)
}

func TestSubmitReview_ParentReviewAnswers2xxWithoutAReview_LooksItUpInsteadOfTrustingIt(t *testing.T) {
	published := `[{"id":9,"user":{"login":"me"},"state":"APPROVED","commit_id":"sha","body":"LGTM","html_url":"https://parent/first","submitted_at":"` + stamp(0) + `"}]`
	for name, answer := range map[string]string{"empty": "", "null": "null", "no ID": "{}"} {
		t.Run(name, func(t *testing.T) {
			posts, lookups := 0, 0
			app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/repos/so/sr/pulls/5/comments":
					_, _ = w.Write([]byte(`{"id":100,"html_url":"https://sub/comment"}`))
				case r.Method == http.MethodPost && r.URL.Path == "/repos/o/r/pulls/1/reviews":
					posts++
					_, _ = w.Write([]byte(answer))
				case r.Method == http.MethodGet && r.URL.Path == "/repos/o/r/pulls/1/reviews":
					lookups++
					_, _ = w.Write([]byte(published))
				case r.URL.Path == "/repos/o/r/pulls/1/reviews/9/comments":
					_, _ = w.Write([]byte(`[]`))
				default:
					w.WriteHeader(http.StatusTeapot)
				}
			})
			app.pendingComments = subDrafts()[:1]
			req := model.ReviewSubmitRequest{Verdict: "APPROVE", Body: "LGTM", Submodules: []model.SubmoduleReviewChoice{{Path: "lib", Mode: "comments"}}}

			res, err := app.SubmitReview(shown(t, app, req))

			require.NoError(t, err)
			assert.Equal(t, []int{1, 1}, []int{posts, lookups}, "posts, lookups")
			assert.Equal(t, "https://parent/first", res.HTMLURL)
		})
	}
}

func TestSubmitReview_FileReviewSubmitAnswerUnclear_TrustsOnlyAReadBackOfThatReviewSubmitted(t *testing.T) {
	submitted := `{"id":10,"state":"COMMENTED","html_url":"https://x/review"}`
	for name, tc := range map[string]struct {
		status           int
		answer, readBack string
		wantURL          string
	}{
		"2xx without a review, read back submitted": {http.StatusOK, `{}`, submitted, "https://x/review"},
		"2xx null, read back submitted":             {http.StatusOK, `null`, submitted, "https://x/review"},
		"5xx, read back empty":                      {http.StatusBadGateway, "", `{}`, ""},
		"5xx, read back as another review":          {http.StatusBadGateway, "", `{"id":11,"state":"COMMENTED"}`, ""},
		"5xx, read back as another pending review":  {http.StatusBadGateway, "", `{"id":11,"state":"PENDING"}`, ""},
	} {
		t.Run(name, func(t *testing.T) {
			fakeGraphQL(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"data":{}}`)) })
			app := subApp(t, func(w http.ResponseWriter, r *http.Request) {
				switch p := r.URL.Path; {
				case r.Method == http.MethodGet && p == "/repos/o/r/pulls/1/reviews":
					_, _ = w.Write([]byte(`[]`))
				case p == "/repos/o/r/pulls/1/reviews":
					_, _ = w.Write([]byte(`{"id":10,"node_id":"PRR_10"}`))
				case p == "/repos/o/r/pulls/1/reviews/10/events":
					w.WriteHeader(tc.status)
					_, _ = w.Write([]byte(tc.answer))
				case p == "/repos/o/r/pulls/1/reviews/10":
					_, _ = w.Write([]byte(tc.readBack))
				default:
					w.WriteHeader(http.StatusTeapot)
				}
			})
			app.pendingComments = []model.DraftCommentDTO{{Path: "a.go", Side: "RIGHT", Body: "file"}}

			res, err := app.SubmitReview(model.ReviewSubmitRequest{Verdict: "COMMENT"})

			if tc.wantURL == "" {
				require.Error(t, err)
				assert.NotNil(t, app.parentAttempt, "the outcome stays unknown")
				assert.Len(t, app.pendingComments, 1)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantURL, res.HTMLURL)
		})
	}
}
