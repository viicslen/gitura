package ghapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitura/internal/model"
)

func TestAddBranchRefs_ManyItems_BatchesAndFillsRefs(t *testing.T) {
	items := make([]model.PRListItem, nodesPerQuery+1)
	for i := range items {
		items[i].NodeID = fmt.Sprintf("PR_%d", i)
	}
	items = append(items, model.PRListItem{Number: 999})

	var batches []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Variables struct {
				IDs []string `json:"ids"`
			} `json:"variables"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		batches = append(batches, len(req.Variables.IDs))
		nodes := []any{nil}
		for _, id := range req.Variables.IDs {
			repo := any(map[string]any{"nameWithOwner": "o/r"})
			if id == "PR_1" {
				repo = nil
			}
			nodes = append(nodes, map[string]any{"id": id, "baseRefName": "base-" + id, "headRefName": "head-" + id, "headRepository": repo})
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"nodes": nodes}}))
	}))
	defer srv.Close()
	orig := githubGraphQLURL
	githubGraphQLURL = srv.URL
	defer func() { githubGraphQLURL = orig }()

	require.NoError(t, AddBranchRefs(context.Background(), srv.Client(), items))
	assert.Equal(t, []int{nodesPerQuery, 1}, batches)
	assert.Equal(t, "base-PR_0", items[0].BaseRef)
	assert.Equal(t, "head-PR_0", items[0].HeadRef)
	assert.Equal(t, "o/r", items[0].HeadRepo)
	assert.Empty(t, items[1].HeadRepo)
	assert.Equal(t, "head-PR_100", items[nodesPerQuery].HeadRef)
	assert.Empty(t, items[len(items)-1].BaseRef)
}

func TestAddBranchRefs_PartialGraphQLError_FillsResolvedNodesAndReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte(`{"data":{"nodes":[{"id":"PR_1","baseRefName":"main","headRefName":"feat1","headRepository":{"nameWithOwner":"o/r"}},null]},"errors":[{"message":"nope"}]}`))
		assert.NoError(t, err)
	}))
	defer srv.Close()
	orig := githubGraphQLURL
	githubGraphQLURL = srv.URL
	defer func() { githubGraphQLURL = orig }()

	items := []model.PRListItem{{NodeID: "PR_1"}, {NodeID: "PR_2"}}
	err := AddBranchRefs(context.Background(), srv.Client(), items)
	assert.ErrorContains(t, err, "nope")
	assert.Equal(t, "feat1", items[0].HeadRef)
	assert.Empty(t, items[1].HeadRef)
}

func TestAddBranchRefs_EmptyResponse_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte(`{}`))
		assert.NoError(t, err)
	}))
	defer srv.Close()
	orig := githubGraphQLURL
	githubGraphQLURL = srv.URL
	defer func() { githubGraphQLURL = orig }()

	err := AddBranchRefs(context.Background(), srv.Client(), []model.PRListItem{{NodeID: "PR_1"}})
	assert.ErrorContains(t, err, "decode GraphQL data")
}
