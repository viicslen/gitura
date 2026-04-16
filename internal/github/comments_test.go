package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixtureDir returns the path to tests/fixtures/graphql relative to this file.
func fixtureDir() string {
	_, file, _, _ := runtime.Caller(0)
	// file is .../internal/github/comments_test.go
	// walk up to repo root
	dir := filepath.Dir(filepath.Dir(filepath.Dir(file)))
	return filepath.Join(dir, "tests", "fixtures", "graphql")
}

// loadFixture reads a fixture JSON file by name.
func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixtureDir(), name))
	require.NoError(t, err, "fixture %s not found", name)
	return data
}

// TestFetchReviewThreads_SinglePage_ReturnsOneThread verifies that a single-page
// GraphQL response is correctly decoded and mapped to CommentThreadDTO.
func TestFetchReviewThreads_SinglePage_ReturnsOneThread(t *testing.T) {
	body := loadFixture(t, "review_threads_single_page.json")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	// Override the GraphQL URL to point at the test server.
	origURL := githubGraphQLURL
	githubGraphQLURL = srv.URL
	defer func() { githubGraphQLURL = origURL }()

	client := srv.Client()
	threads, err := FetchReviewThreads(context.Background(), client, "owner", "repo", 7, nil)
	require.NoError(t, err)
	require.Len(t, threads, 1)

	th := threads[0]
	assert.Equal(t, "PRRT_kwDOA1B2Mc4ADeF1", th.NodeID)
	assert.False(t, th.Resolved)
	assert.Equal(t, "internal/foo/bar.go", th.Path)
	assert.Equal(t, 42, th.Line)
	require.Len(t, th.Comments, 1)
	assert.Equal(t, int64(1001), th.Comments[0].ID)
	assert.Equal(t, "reviewer1", th.Comments[0].AuthorLogin)
	assert.False(t, th.Comments[0].IsSuggestion)
}

// TestFetchReviewThreads_MultiPage_PaginatesAndMerges verifies that pagination
// is handled correctly and all threads from both pages are returned.
func TestFetchReviewThreads_MultiPage_PaginatesAndMerges(t *testing.T) {
	page1 := loadFixture(t, "review_threads_page1.json")
	page2 := loadFixture(t, "review_threads_page2.json")
	callCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		callCount++
		if callCount == 1 {
			_, _ = w.Write(page1)
		} else {
			_, _ = w.Write(page2)
		}
	}))
	defer srv.Close()

	origURL := githubGraphQLURL
	githubGraphQLURL = srv.URL
	defer func() { githubGraphQLURL = origURL }()

	threads, err := FetchReviewThreads(context.Background(), srv.Client(), "owner", "repo", 8, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, callCount, "expected exactly 2 GraphQL requests for 2 pages")
	assert.Len(t, threads, 2)
	assert.Equal(t, int64(2001), threads[0].RootID)
	assert.Equal(t, int64(3001), threads[1].RootID)
	assert.True(t, threads[1].Resolved)
}

// TestFetchReviewThreads_Empty_ReturnsEmptySlice verifies no threads returns
// an empty (non-nil) slice.
func TestFetchReviewThreads_Empty_ReturnsEmptySlice(t *testing.T) {
	body := loadFixture(t, "review_threads_empty.json")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	origURL := githubGraphQLURL
	githubGraphQLURL = srv.URL
	defer func() { githubGraphQLURL = origURL }()

	threads, err := FetchReviewThreads(context.Background(), srv.Client(), "owner", "repo", 1, nil)
	require.NoError(t, err)
	assert.NotNil(t, threads)
	assert.Empty(t, threads)
}

// TestFetchReviewThreads_GraphQLErrors_ReturnsError verifies that a GraphQL
// errors array in the response body surfaces as a "github:" prefixed error.
func TestFetchReviewThreads_GraphQLErrors_ReturnsError(t *testing.T) {
	body := loadFixture(t, "review_threads_graphql_error.json")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	origURL := githubGraphQLURL
	githubGraphQLURL = srv.URL
	defer func() { githubGraphQLURL = origURL }()

	_, err := FetchReviewThreads(context.Background(), srv.Client(), "owner", "nonexistent", 1, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "github:")
}

// TestFetchReviewThreads_HTTP401_ReturnsAuthError verifies HTTP 401 returns an
// "auth:" prefixed error.
func TestFetchReviewThreads_HTTP401_ReturnsAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	origURL := githubGraphQLURL
	githubGraphQLURL = srv.URL
	defer func() { githubGraphQLURL = origURL }()

	_, err := FetchReviewThreads(context.Background(), srv.Client(), "owner", "repo", 1, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth:")
}

// TestFetchReviewThreads_ProgressCallback_CalledPerPage verifies the progress
// callback is called once per page.
func TestFetchReviewThreads_ProgressCallback_CalledPerPage(t *testing.T) {
	page1 := loadFixture(t, "review_threads_page1.json")
	page2 := loadFixture(t, "review_threads_page2.json")
	callCount := 0
	progressCalls := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		callCount++
		if callCount == 1 {
			_, _ = w.Write(page1)
		} else {
			_, _ = w.Write(page2)
		}
	}))
	defer srv.Close()

	origURL := githubGraphQLURL
	githubGraphQLURL = srv.URL
	defer func() { githubGraphQLURL = origURL }()

	_, err := FetchReviewThreads(context.Background(), srv.Client(), "owner", "repo", 8,
		func(_, total int) {
			progressCalls++
			assert.Equal(t, -1, total)
		})
	require.NoError(t, err)
	assert.Equal(t, 2, progressCalls, "progress callback must be called once per page")
}

// TestFetchReviewThreads_SuggestionDetection_SetsFlagFromBody verifies that
// IsSuggestion is true for comments containing a suggestion fenced block.
func TestFetchReviewThreads_SuggestionDetection_SetsFlagFromBody(t *testing.T) {
	body := `{
		"data": {
			"repository": {
				"pullRequest": {
					"reviewThreads": {
						"pageInfo": {"hasNextPage": false, "endCursor": ""},
						"nodes": [{
							"id": "PRRT_sug1",
							"isResolved": false,
							"comments": {
								"nodes": [{
									"databaseId": 5001,
									"body": "Please fix:\n` + "```" + `suggestion\nfixed line\n` + "```" + `",
									"author": {"login": "rev", "avatarUrl": ""},
									"path": "main.go",
									"line": 1,
									"originalLine": 1,
									"diffHunk": "@@ -1 +1 @@\n-old\n+new",
									"createdAt": "2025-03-01T00:00:00Z",
									"url": "https://github.com/o/r/pull/1",
									"replyTo": null
								}]
							}
						}]
					}
				}
			}
		}
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	origURL := githubGraphQLURL
	githubGraphQLURL = srv.URL
	defer func() { githubGraphQLURL = origURL }()

	threads, err := FetchReviewThreads(context.Background(), srv.Client(), "o", "r", 1, nil)
	require.NoError(t, err)
	require.Len(t, threads, 1)
	assert.True(t, threads[0].Comments[0].IsSuggestion)
}

// TestFetchReviewThreads_HTTP500_ReturnsGithubError verifies a non-401, non-200
// response returns a "github:" prefixed error.
func TestFetchReviewThreads_HTTP500_ReturnsGithubError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	origURL := githubGraphQLURL
	githubGraphQLURL = srv.URL
	defer func() { githubGraphQLURL = origURL }()

	_, err := FetchReviewThreads(context.Background(), srv.Client(), "owner", "repo", 1, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "github:")
}

// TestMapGraphQLThread_EmptyComments_ReturnsEmptyThreadDTO verifies that a
// thread node with no comments still produces a valid (empty) DTO.
func TestMapGraphQLThread_EmptyComments_ReturnsEmptyThreadDTO(t *testing.T) {
	node := graphQLThread{
		ID:         "PRRT_empty",
		IsResolved: true,
		Comments:   graphQLCommentConnection{Nodes: []graphQLComment{}},
	}
	dto := mapGraphQLThread(node)
	assert.Equal(t, "PRRT_empty", dto.NodeID)
	assert.True(t, dto.Resolved)
	assert.Empty(t, dto.Comments)
}

// TestMapGraphQLThread_OriginalLine_FallsBackCorrectly verifies that when Line
// is nil but OriginalLine is set, the OriginalLine value is used.
func TestMapGraphQLThread_OriginalLine_FallsBackCorrectly(t *testing.T) {
	origLine := 17
	node := graphQLThread{
		ID:         "PRRT_origline",
		IsResolved: false,
		Comments: graphQLCommentConnection{
			Nodes: []graphQLComment{
				{
					DatabaseID:   42,
					Body:         "comment",
					Author:       graphQLActor{Login: "dev"},
					Path:         "src/foo.go",
					Line:         nil,
					OriginalLine: &origLine,
				},
			},
		},
	}
	dto := mapGraphQLThread(node)
	assert.Equal(t, 17, dto.Line)
	assert.Equal(t, "src/foo.go", dto.Path)
}

// TestMapGraphQLThread_ReplyTo_SetsInReplyToID verifies that a comment with a
// non-nil replyTo sets the InReplyToID field correctly.
func TestMapGraphQLThread_ReplyTo_SetsInReplyToID(t *testing.T) {
	node := graphQLThread{
		ID: "PRRT_reply",
		Comments: graphQLCommentConnection{
			Nodes: []graphQLComment{
				{
					DatabaseID: 100,
					Body:       "root comment",
					Author:     graphQLActor{Login: "alice"},
				},
				{
					DatabaseID: 101,
					Body:       "reply comment",
					Author:     graphQLActor{Login: "bob"},
					ReplyTo:    &graphQLReplyTo{DatabaseID: 100},
				},
			},
		},
	}
	dto := mapGraphQLThread(node)
	require.Len(t, dto.Comments, 2)
	assert.Equal(t, int64(0), dto.Comments[0].InReplyToID)
	assert.Equal(t, int64(100), dto.Comments[1].InReplyToID)
}
