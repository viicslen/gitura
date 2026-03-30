package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/google/go-github/v67/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitura/internal/model"
)

// ── httptest helpers ────────────────────────────────────────────────────────

// searchResponse builds a minimal GitHub Search API response body.
func searchResponse(items []map[string]interface{}, total int, incomplete bool) string {
	body := map[string]interface{}{
		"total_count":        total,
		"incomplete_results": incomplete,
		"items":              items,
	}
	b, _ := json.Marshal(body)
	return string(b)
}

// makeIssueJSON builds a minimal issue JSON object for search results.
// assignees and requestedReviewers are optional — pass nil to omit.
func makeIssueJSON(
	number int,
	title, htmlURL, repoURL, authorLogin string,
	isDraft bool,
	updatedAt time.Time,
	assignees []string,
	requestedReviewers []string,
) map[string]interface{} {
	assigneeList := make([]map[string]interface{}, 0, len(assignees))
	for _, a := range assignees {
		assigneeList = append(assigneeList, map[string]interface{}{"login": a})
	}
	reviewerList := make([]map[string]interface{}, 0, len(requestedReviewers))
	for _, r := range requestedReviewers {
		reviewerList = append(reviewerList, map[string]interface{}{"login": r})
	}
	return map[string]interface{}{
		"number":              number,
		"title":               title,
		"html_url":            htmlURL,
		"repository_url":      repoURL,
		"user":                map[string]interface{}{"login": authorLogin},
		"draft":               isDraft,
		"created_at":          updatedAt.Add(-24 * time.Hour).Format(time.RFC3339),
		"updated_at":          updatedAt.Format(time.RFC3339),
		"assignees":           assigneeList,
		"requested_reviewers": reviewerList,
		"pull_request":        map[string]interface{}{"url": "https://api.github.com/repos/owner/repo/pulls/1"},
	}
}

// newTestClient returns a *github.Client pointing at the given httptest server.
func newTestClient(srv *httptest.Server) *github.Client {
	client := github.NewClient(nil)
	parsed, _ := url.Parse(srv.URL + "/")
	client.BaseURL = parsed
	return client
}

// ── runQuery tests ───────────────────────────────────────────────────────────

// TestRunQuery_SinglePage_ReturnsAllIssues verifies a single-page response is
// collected correctly.
func TestRunQuery_SinglePage_ReturnsAllIssues(t *testing.T) {
	now := time.Now().UTC()
	item := makeIssueJSON(1, "Fix it", "https://github.com/o/r/pull/1",
		"https://api.github.com/repos/o/r", "alice", false, now, nil, nil)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, searchResponse([]map[string]interface{}{item}, 1, false))
	}))
	defer srv.Close()

	client := newTestClient(srv)
	issues, incomplete, err := runQuery(context.Background(), client, "is:pr is:open")
	require.NoError(t, err)
	assert.False(t, incomplete)
	assert.Len(t, issues, 1)
	assert.Equal(t, 1, issues[0].GetNumber())
}

// TestRunQuery_IncompleteResults_SetsFlag verifies that incomplete_results=true
// from the API is surfaced.
func TestRunQuery_IncompleteResults_SetsFlag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, searchResponse([]map[string]interface{}{}, 0, true))
	}))
	defer srv.Close()

	client := newTestClient(srv)
	_, incomplete, err := runQuery(context.Background(), client, "is:pr is:open")
	require.NoError(t, err)
	assert.True(t, incomplete)
}

// TestRunQuery_RateLimited_ReturnsError verifies 403 with rate limit header is
// returned as an error.
func TestRunQuery_RateLimited_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-RateLimit-Limit", "30")
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(60*time.Second).Unix()))
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"message":"API rate limit exceeded","documentation_url":"https://docs.github.com/rest/overview/resources-in-the-rest-api#rate-limiting"}`)
	}))
	defer srv.Close()

	client := newTestClient(srv)
	_, _, err := runQuery(context.Background(), client, "is:pr is:open")
	require.Error(t, err)
}

// ── issueToItem tests ────────────────────────────────────────────────────────

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }
func boolPtr(b bool) *bool    { return &b }

func makeTimestamp(t time.Time) *github.Timestamp {
	ts := github.Timestamp{Time: t}
	return &ts
}

func makeIssue(
	number int,
	title, htmlURL, repoURL, authorLogin string,
	isDraft bool,
	updatedAt time.Time,
) *github.Issue {
	return &github.Issue{
		Number:        intPtr(number),
		Title:         strPtr(title),
		HTMLURL:       strPtr(htmlURL),
		RepositoryURL: strPtr(repoURL),
		User:          &github.User{Login: strPtr(authorLogin)},
		Draft:         boolPtr(isDraft),
		CreatedAt:     makeTimestamp(updatedAt.Add(-24 * time.Hour)),
		UpdatedAt:     makeTimestamp(updatedAt),
		PullRequestLinks: &github.PullRequestLinks{
			URL: strPtr("https://api.github.com/repos/owner/repo/pulls/1"),
		},
	}
}

// TestIssueToItem_ValidIssue_SetsAuthorTag verifies the IsAuthor tag is set
// when the login matches the PR author.
func TestIssueToItem_ValidIssue_SetsAuthorTag(t *testing.T) {
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	issue := makeIssue(42, "Fix the bug", "https://github.com/owner/repo/pull/42",
		"https://api.github.com/repos/owner/repo", "alice", false, now)

	item, ok := issueToItem(issue, "alice")
	require.True(t, ok)
	assert.Equal(t, 42, item.Number)
	assert.Equal(t, "Fix the bug", item.Title)
	assert.Equal(t, "owner", item.Owner)
	assert.Equal(t, "repo", item.Repo)
	assert.Equal(t, "alice", item.AuthorLogin)
	assert.Equal(t, "https://github.com/owner/repo/pull/42", item.HTMLURL)
	assert.False(t, item.IsDraft)
	assert.Equal(t, now.UTC().Format(time.RFC3339), item.UpdatedAt)
	assert.True(t, item.IsAuthor)
	assert.False(t, item.IsAssignee)
}

// TestIssueToItem_DraftIssue_SetsDraftTrue verifies draft status is propagated.
func TestIssueToItem_DraftIssue_SetsDraftTrue(t *testing.T) {
	issue := makeIssue(1, "WIP", "https://github.com/o/r/pull/1",
		"https://api.github.com/repos/o/r", "bob", true, time.Now())

	item, ok := issueToItem(issue, "bob")
	require.True(t, ok)
	assert.True(t, item.IsDraft)
}

// TestIssueToItem_AssigneeTag_SetWhenLoginInAssignees verifies the IsAssignee tag.
func TestIssueToItem_AssigneeTag_SetWhenLoginInAssignees(t *testing.T) {
	issue := makeIssue(1, "T", "https://github.com/o/r/pull/1",
		"https://api.github.com/repos/o/r", "alice", false, time.Now())
	issue.Assignees = []*github.User{{Login: strPtr("bob")}}

	item, ok := issueToItem(issue, "bob")
	require.True(t, ok)
	assert.False(t, item.IsAuthor)
	assert.True(t, item.IsAssignee)
}

// TestIssueToItem_ReviewerTag_SetWhenNeitherAuthorNorAssignee verifies that a
// PR where the user is neither author nor assignee gets IsReviewer=true
// (best-effort approximation since Search API doesn't expose requested_reviewers).
func TestIssueToItem_ReviewerTag_SetWhenNeitherAuthorNorAssignee(t *testing.T) {
	issue := makeIssue(1, "T", "https://github.com/o/r/pull/1",
		"https://api.github.com/repos/o/r", "alice", false, time.Now())

	item, ok := issueToItem(issue, "charlie")
	require.True(t, ok)
	assert.False(t, item.IsAuthor)
	assert.False(t, item.IsAssignee)
	assert.True(t, item.IsReviewer)
}

// TestIssueToItem_MissingRepoURL_ReturnsFalse verifies required field guard.
func TestIssueToItem_MissingRepoURL_ReturnsFalse(t *testing.T) {
	issue := makeIssue(1, "Title", "https://github.com/o/r/pull/1", "", "bob", false, time.Now())
	_, ok := issueToItem(issue, "bob")
	assert.False(t, ok)
}

// TestIssueToItem_MissingTitle_ReturnsFalse verifies required field guard.
func TestIssueToItem_MissingTitle_ReturnsFalse(t *testing.T) {
	issue := makeIssue(1, "", "https://github.com/o/r/pull/1",
		"https://api.github.com/repos/o/r", "bob", false, time.Now())
	_, ok := issueToItem(issue, "bob")
	assert.False(t, ok)
}

// TestIssueToItem_MissingHTMLURL_ReturnsFalse verifies required field guard.
func TestIssueToItem_MissingHTMLURL_ReturnsFalse(t *testing.T) {
	issue := makeIssue(1, "Title", "", "https://api.github.com/repos/o/r", "bob", false, time.Now())
	_, ok := issueToItem(issue, "bob")
	assert.False(t, ok)
}

// ── SearchOpenPRs integration-style tests ───────────────────────────────────

// TestSearchOpenPRs_SingleQuery_UsesInvolvesQualifier verifies that exactly one
// query is issued and it contains the involves: qualifier.
func TestSearchOpenPRs_SingleQuery_UsesInvolvesQualifier(t *testing.T) {
	callCount := 0
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		capturedQuery = r.URL.Query().Get("q")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, searchResponse([]map[string]interface{}{}, 0, false))
	}))
	defer srv.Close()

	client := newTestClient(srv)
	filters := model.PRListFilters{
		IncludeAuthor:   true,
		IncludeAssignee: true,
		IncludeReviewer: true,
	}
	_, err := SearchOpenPRs(context.Background(), client, "alice", filters)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount, "expected exactly 1 query (involves: replaces 3-query fan-out)")
	assert.Contains(t, capturedQuery, "involves:alice", "involves: qualifier must not quote the login")
	assert.NotContains(t, capturedQuery, `involves:"alice"`, "login must not be quoted")
}

// TestSearchOpenPRs_FieldMapping_MapsIssueToItem verifies that all PRListItem
// fields are correctly mapped from the GitHub API response.
func TestSearchOpenPRs_FieldMapping_MapsIssueToItem(t *testing.T) {
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	pr := makeIssueJSON(42, "My PR", "https://github.com/org/repo/pull/42",
		"https://api.github.com/repos/org/repo", "bob", true, now, nil, nil)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, searchResponse([]map[string]interface{}{pr}, 1, false))
	}))
	defer srv.Close()

	client := newTestClient(srv)
	filters := model.PRListFilters{IncludeAuthor: true}
	result, err := SearchOpenPRs(context.Background(), client, "bob", filters)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)

	item := result.Items[0]
	assert.Equal(t, 42, item.Number)
	assert.Equal(t, "My PR", item.Title)
	assert.Equal(t, "org", item.Owner)
	assert.Equal(t, "repo", item.Repo)
	assert.Equal(t, "bob", item.AuthorLogin)
	assert.Equal(t, "https://github.com/org/repo/pull/42", item.HTMLURL)
	assert.True(t, item.IsDraft)
	assert.Equal(t, now.Format(time.RFC3339), item.UpdatedAt)
	assert.True(t, item.IsAuthor, "bob authored this PR")
}

// TestSearchOpenPRs_InvolvementTags_AssigneeAndReviewer verifies that assignee
// and reviewer tags are set correctly on returned items.
func TestSearchOpenPRs_InvolvementTags_AssigneeAndReviewer(t *testing.T) {
	now := time.Now().UTC()
	// PR authored by alice, assigned to bob
	prAssigned := makeIssueJSON(1, "Assigned", "https://github.com/o/r/pull/1",
		"https://api.github.com/repos/o/r", "alice", false, now, []string{"bob"}, nil)
	// PR authored by alice, bob is neither author nor assignee → reviewer tag
	prReviewer := makeIssueJSON(2, "Reviewer", "https://github.com/o/r/pull/2",
		"https://api.github.com/repos/o/r", "alice", false, now.Add(-time.Hour), nil, nil)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, searchResponse([]map[string]interface{}{prAssigned, prReviewer}, 2, false))
	}))
	defer srv.Close()

	client := newTestClient(srv)
	filters := model.PRListFilters{IncludeAuthor: true, IncludeAssignee: true, IncludeReviewer: true}
	result, err := SearchOpenPRs(context.Background(), client, "bob", filters)
	require.NoError(t, err)
	require.Len(t, result.Items, 2)

	assigned := result.Items[0] // newer
	assert.True(t, assigned.IsAssignee)
	assert.False(t, assigned.IsAuthor)

	reviewer := result.Items[1] // older
	assert.True(t, reviewer.IsReviewer)
	assert.False(t, reviewer.IsAuthor)
	assert.False(t, reviewer.IsAssignee)
}

// TestSearchOpenPRs_RateLimit_ReturnsMappedResult verifies that a rate limit
// error from the API is translated into PRListResult.Error + RateLimitReset.
func TestSearchOpenPRs_RateLimit_ReturnsMappedResult(t *testing.T) {
	resetTime := time.Now().Add(60 * time.Second).Unix()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-RateLimit-Limit", "30")
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", resetTime))
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"message":"API rate limit exceeded","documentation_url":"https://docs.github.com/rest/overview/resources-in-the-rest-api#rate-limiting"}`)
	}))
	defer srv.Close()

	client := newTestClient(srv)
	filters := model.PRListFilters{IncludeAuthor: true}
	result, err := SearchOpenPRs(context.Background(), client, "alice", filters)
	require.NoError(t, err)
	assert.Equal(t, "rate limit exceeded", result.Error)
	assert.NotEmpty(t, result.RateLimitReset)
}

// TestSearchOpenPRs_SortedByUpdatedAtDesc verifies results are sorted newest-first.
func TestSearchOpenPRs_SortedByUpdatedAtDesc(t *testing.T) {
	older := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

	prOld := makeIssueJSON(1, "Old PR", "https://github.com/o/r/pull/1",
		"https://api.github.com/repos/o/r", "alice", false, older, nil, nil)
	prNew := makeIssueJSON(2, "New PR", "https://github.com/o/r/pull/2",
		"https://api.github.com/repos/o/r", "alice", false, newer, nil, nil)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, searchResponse([]map[string]interface{}{prOld, prNew}, 2, false))
	}))
	defer srv.Close()

	client := newTestClient(srv)
	filters := model.PRListFilters{IncludeAuthor: true}
	result, err := SearchOpenPRs(context.Background(), client, "alice", filters)
	require.NoError(t, err)
	require.Len(t, result.Items, 2)
	assert.Equal(t, 2, result.Items[0].Number, "newer PR should be first")
	assert.Equal(t, 1, result.Items[1].Number, "older PR should be second")
}

// TestSearchOpenPRs_GenericError_ReturnsMappedResult verifies that a non-rate-limit
// HTTP error is translated into PRListResult.Error.
func TestSearchOpenPRs_GenericError_ReturnsMappedResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"message":"Internal Server Error"}`)
	}))
	defer srv.Close()

	client := newTestClient(srv)
	filters := model.PRListFilters{IncludeAuthor: true}
	result, err := SearchOpenPRs(context.Background(), client, "alice", filters)
	require.NoError(t, err)
	assert.Contains(t, result.Error, "search error")
}

// TestSearchOpenPRs_IncludeDrafts_OmitsDraftFalseQualifier verifies that when
// IncludeDrafts is true, the draft:false qualifier is not added to the query.
func TestSearchOpenPRs_IncludeDrafts_OmitsDraftFalseQualifier(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query().Get("q")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, searchResponse([]map[string]interface{}{}, 0, false))
	}))
	defer srv.Close()

	client := newTestClient(srv)
	filters := model.PRListFilters{IncludeAuthor: true, IncludeDrafts: true}
	_, err := SearchOpenPRs(context.Background(), client, "alice", filters)
	require.NoError(t, err)
	assert.NotContains(t, capturedQuery, "draft:false")
}

// TestSearchOpenPRs_ExcludeDrafts_AddsDraftFalseQualifier verifies the default
// draft:false qualifier is present when IncludeDrafts is false.
func TestSearchOpenPRs_ExcludeDrafts_AddsDraftFalseQualifier(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query().Get("q")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, searchResponse([]map[string]interface{}{}, 0, false))
	}))
	defer srv.Close()

	client := newTestClient(srv)
	filters := model.PRListFilters{IncludeAuthor: true, IncludeDrafts: false}
	_, err := SearchOpenPRs(context.Background(), client, "alice", filters)
	require.NoError(t, err)
	assert.Contains(t, capturedQuery, "draft:false")
}

// TestSearchOpenPRs_AbuseLimitRetryAfter_ReturnsMappedResult verifies that an
// abuse/secondary rate limit error is translated into PRListResult.Error.
func TestSearchOpenPRs_AbuseLimitRetryAfter_ReturnsMappedResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"message":"You have exceeded a secondary rate limit","documentation_url":"https://docs.github.com/rest/overview/rate-limits-for-the-rest-api#about-secondary-rate-limits"}`)
	}))
	defer srv.Close()

	client := newTestClient(srv)
	filters := model.PRListFilters{IncludeAuthor: true}
	result, err := SearchOpenPRs(context.Background(), client, "alice", filters)
	require.NoError(t, err)
	assert.Contains(t, result.Error, "rate limit")
}

// TestSearchOpenPRs_NoFiltersAppliedServerSide_RepoNotInQuery verifies that
// repo/org/author filters are NOT sent in the API query (client-side only).
func TestSearchOpenPRs_NoFiltersAppliedServerSide_RepoNotInQuery(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query().Get("q")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, searchResponse([]map[string]interface{}{}, 0, false))
	}))
	defer srv.Close()

	client := newTestClient(srv)
	// Even with repo/org/author set, they must NOT appear in the API query.
	filters := model.PRListFilters{
		IncludeAuthor: true,
		Repo:          "org/myrepo",
		Org:           "myorg",
		Author:        "someuser",
	}
	_, err := SearchOpenPRs(context.Background(), client, "alice", filters)
	require.NoError(t, err)
	assert.NotContains(t, capturedQuery, "repo:")
	assert.NotContains(t, capturedQuery, "org:")
	assert.NotContains(t, capturedQuery, "author:")
}

// TestRepoFromURL_ValidURL_ReturnsOwnerAndRepo verifies happy-path extraction.
func TestRepoFromURL_ValidURL_ReturnsOwnerAndRepo(t *testing.T) {
	owner, repo := repoFromURL("https://api.github.com/repos/octocat/Hello-World")
	assert.Equal(t, "octocat", owner)
	assert.Equal(t, "Hello-World", repo)
}

// TestRepoFromURL_EmptyURL_ReturnsBothEmpty verifies empty string returns empty strings.
func TestRepoFromURL_EmptyURL_ReturnsBothEmpty(t *testing.T) {
	owner, repo := repoFromURL("")
	assert.Equal(t, "", owner)
	assert.Equal(t, "", repo)
}

// TestRepoFromURL_OnlyOwner_ReturnsBothEmpty verifies only-owner URL returns empty strings.
func TestRepoFromURL_OnlyOwner_ReturnsBothEmpty(t *testing.T) {
	owner, repo := repoFromURL("https://api.github.com/repos/octocat")
	assert.Equal(t, "", owner)
	assert.Equal(t, "", repo)
}

// TestRepoFromURL_NonAPIURL_DoesNotPanic verifies that a non-API URL doesn't panic.
func TestRepoFromURL_NonAPIURL_DoesNotPanic(t *testing.T) {
	owner, repo := repoFromURL("https://github.com/octocat/Hello-World")
	_ = owner
	_ = repo
}
