package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchViewedFiles_PaginatesAndKeepsOnlyViewed(t *testing.T) {
	pages := []string{
		`{"data":{"repository":{"pullRequest":{"files":{"nodes":[{"path":"a.go","viewerViewedState":"VIEWED"},{"path":"b.go","viewerViewedState":"DISMISSED"}],"pageInfo":{"hasNextPage":true,"endCursor":"c1"}}}}}}`,
		`{"data":{"repository":{"pullRequest":{"files":{"nodes":[{"path":"c.go","viewerViewedState":"VIEWED"},{"path":"d.go","viewerViewedState":"UNVIEWED"}],"pageInfo":{"hasNextPage":false,"endCursor":"c2"}}}}}}`,
	}
	var afters []any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Variables map[string]any `json:"variables"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		afters = append(afters, req.Variables["after"])
		_, _ = w.Write([]byte(pages[len(afters)-1]))
	}))
	defer srv.Close()
	orig := githubGraphQLURL
	githubGraphQLURL = srv.URL
	defer func() { githubGraphQLURL = orig }()

	viewed, err := FetchViewedFiles(context.Background(), srv.Client(), "o", "r", 1)
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"a.go": true, "c.go": true}, viewed)
	assert.Equal(t, []any{nil, "c1"}, afters)
}

func TestSetFileViewed_PicksMutationAndSurfacesErrors(t *testing.T) {
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query string `json:"query"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		query = req.Query
		_, _ = w.Write([]byte(`{"errors":[{"message":"nope"}]}`))
	}))
	defer srv.Close()
	orig := githubGraphQLURL
	githubGraphQLURL = srv.URL
	defer func() { githubGraphQLURL = orig }()

	err := SetFileViewed(context.Background(), srv.Client(), "PR_1", "a.go", false)
	assert.Contains(t, query, "unmarkFileAsViewed")
	assert.ErrorContains(t, err, "github: GraphQL errors: nope")
}
