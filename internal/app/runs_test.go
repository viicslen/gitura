package app

import (
	"testing"

	"gitura/internal/model"
)

func TestRunVars_ThreadComment_FillsPRAndCommentValues(t *testing.T) {
	a := New("")
	a.prCache = &model.PullRequestSummary{Number: 7, Owner: "o", Repo: "r", HeadBranch: "feat", BaseBranch: "main", AuthorLogin: "alice"}
	a.threads = []model.CommentThreadDTO{{
		RootID: 1, Path: "a.go", Line: 12,
		Comments: []model.ReviewCommentDTO{{ID: 1, AuthorLogin: "bob"}, {ID: 2, AuthorLogin: "carol"}},
	}}

	got := a.runVars(model.RunContext{ThreadRootID: 1, CommentID: 2})
	want := map[string]string{
		"pr_number": "7", "repo": "o/r", "head_branch": "feat", "base_branch": "main",
		"author": "alice", "comment_author": "carol", "file": "a.go", "line": "12", "pr_title": "", "pr_url": "",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %q, want %q", k, got[k], v)
		}
	}

	if got := New("").runVars(model.RunContext{}); got["pr_number"] != "" || len(got) != len(want) {
		t.Errorf("no PR: got %v, want every variable empty", got)
	}
}
