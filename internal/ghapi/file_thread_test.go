package ghapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitura/internal/model"
)

func TestAddFileReviewThread_Success_SendsFileSubjectForReview(t *testing.T) {
	var got struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		_, _ = w.Write([]byte(`{"data":{"addPullRequestReviewThread":{"thread":{"id":"PRRT_1"}}}}`))
	}))
	defer srv.Close()
	origURL := githubGraphQLURL
	githubGraphQLURL = srv.URL
	defer func() { githubGraphQLURL = origURL }()

	require.NoError(t, AddFileReviewThread(context.Background(), srv.Client(), "PRR_1", "a.go", "split this"))
	assert.Contains(t, got.Query, "subjectType: FILE")
	assert.Equal(t, map[string]any{"reviewID": "PRR_1", "path": "a.go", "body": "split this"}, got.Variables)
}

func TestFetchPendingReviewDrafts_MixedThreads_ReturnsOwnLineRangeAndFileDrafts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"repository":{"pullRequest":{"reviewThreads":{
			"pageInfo":{"hasNextPage":false},
			"nodes":[
				{"diffSide":"RIGHT","originalLine":9,"originalStartLine":7,"startDiffSide":"RIGHT","subjectType":"LINE",
				 "comments":{"nodes":[{"body":"range","path":"a.go","pullRequestReview":{"databaseId":7}}]}},
				{"diffSide":"RIGHT","originalLine":1,"subjectType":"FILE",
				 "comments":{"nodes":[{"body":"file","path":"a.go","pullRequestReview":{"databaseId":7}}]}},
				{"diffSide":"LEFT","originalLine":3,"subjectType":"LINE",
				 "comments":{"nodes":[{"body":"someone else","path":"b.go","pullRequestReview":{"databaseId":8}}]}}
			]}}}}}`))
	}))
	defer srv.Close()
	origURL := githubGraphQLURL
	githubGraphQLURL = srv.URL
	defer func() { githubGraphQLURL = origURL }()

	drafts, err := FetchPendingReviewDrafts(context.Background(), srv.Client(), "o", "r", 1, 7)
	require.NoError(t, err)
	assert.Equal(t, []model.DraftCommentDTO{
		{Path: "a.go", Body: "range", Side: "RIGHT", Line: 9, StartLine: 7, StartSide: "RIGHT"},
		{Path: "a.go", Body: "file", Side: "RIGHT"},
	}, drafts)
}

func TestFetchPendingReviewDrafts_ReviewRepliesToThread_ReturnsConflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"repository":{"pullRequest":{"reviewThreads":{
			"pageInfo":{"hasNextPage":false},
			"nodes":[
				{"diffSide":"RIGHT","originalLine":9,"subjectType":"LINE",
				 "comments":{"nodes":[{"body":"own","path":"a.go","pullRequestReview":{"databaseId":7}}]}},
				{"diffSide":"RIGHT","originalLine":3,"subjectType":"LINE",
				 "comments":{"nodes":[{"body":"theirs","pullRequestReview":{"databaseId":8}},{"body":"reply","pullRequestReview":{"databaseId":7}}]}}
			]}}}}}`))
	}))
	defer srv.Close()
	origURL := githubGraphQLURL
	githubGraphQLURL = srv.URL
	defer func() { githubGraphQLURL = origURL }()

	drafts, err := FetchPendingReviewDrafts(context.Background(), srv.Client(), "o", "r", 1, 7)
	require.ErrorContains(t, err, "conflict:")
	assert.Nil(t, drafts)
}

func TestFetchPendingReviewDrafts_ThreadCommentsHaveMorePages_ReturnsUnheldComments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"repository":{"pullRequest":{"reviewThreads":{
			"pageInfo":{"hasNextPage":false},
			"nodes":[
				{"diffSide":"RIGHT","originalLine":9,"subjectType":"LINE",
				 "comments":{"pageInfo":{"hasNextPage":true,"endCursor":"c100"},
				  "nodes":[{"body":"own","path":"a.go","pullRequestReview":{"databaseId":7}}]}}
			]}}}}}`))
	}))
	defer srv.Close()
	origURL := githubGraphQLURL
	githubGraphQLURL = srv.URL
	defer func() { githubGraphQLURL = origURL }()

	drafts, err := FetchPendingReviewDrafts(context.Background(), srv.Client(), "o", "r", 1, 7)
	require.ErrorIs(t, err, ErrUnheldComments)
	assert.Nil(t, drafts)
}

func TestMapGraphQLThread_FileSubject_HasNoLine(t *testing.T) {
	one := 1
	dto := mapGraphQLThread(graphQLThread{
		SubjectType: "FILE",
		Comments:    graphQLCommentConnection{Nodes: []graphQLComment{{Path: "a.go", OriginalLine: &one}}},
	})
	assert.Zero(t, dto.Line)
	assert.False(t, dto.Outdated)
}
