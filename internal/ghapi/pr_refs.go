package ghapi

import (
	"context"
	"errors"
	"net/http"

	"gitura/internal/model"
)

const branchRefsQuery = `
query($ids: [ID!]!) {
  nodes(ids: $ids) {
    ... on PullRequest { id baseRefName headRefName headRepository { nameWithOwner } }
  }
}`

// nodesPerQuery is the most IDs GitHub accepts in one nodes(ids:) lookup.
const nodesPerQuery = 100

// AddBranchRefs fills BaseRef, HeadRef and HeadRepo on items, which the Search API does not return.
// Items without a NodeID, or that GitHub would not resolve, are left as they are; the error lists what failed.
func AddBranchRefs(ctx context.Context, httpClient *http.Client, items []model.PRListItem) error {
	var errs []error
	byID := make(map[string]*model.PRListItem, len(items))
	ids := make([]string, 0, len(items))
	for i := range items {
		if id := items[i].NodeID; id != "" {
			byID[id] = &items[i]
			ids = append(ids, id)
		}
	}
	for start := 0; start < len(ids); start += nodesPerQuery {
		var data struct {
			Nodes []*struct {
				ID             string `json:"id"`
				BaseRefName    string `json:"baseRefName"`
				HeadRefName    string `json:"headRefName"`
				HeadRepository *struct {
					NameWithOwner string `json:"nameWithOwner"`
				} `json:"headRepository"`
			} `json:"nodes"`
		}
		vars := map[string]any{"ids": ids[start:min(start+nodesPerQuery, len(ids))]}
		if err := runGraphQL(ctx, httpClient, branchRefsQuery, vars, &data); err != nil {
			errs = append(errs, err)
		}
		for _, n := range data.Nodes {
			if n == nil || byID[n.ID] == nil {
				continue
			}
			item := byID[n.ID]
			item.BaseRef, item.HeadRef = n.BaseRefName, n.HeadRefName
			if n.HeadRepository != nil {
				item.HeadRepo = n.HeadRepository.NameWithOwner
			}
		}
	}
	return errors.Join(errs...)
}
