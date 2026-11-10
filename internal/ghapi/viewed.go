package ghapi

import (
	"context"
	"fmt"
	"net/http"
)

const viewedFilesQuery = `
query($owner: String!, $repo: String!, $number: Int!, $after: String) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $number) {
      files(first: 100, after: $after) {
        nodes { path viewerViewedState }
        pageInfo { hasNextPage endCursor }
      }
    }
  }
}`

// FetchViewedFiles returns the paths the authenticated user has marked as viewed on the PR.
// Files changed since they were marked (state DISMISSED) count as not viewed, as on github.com.
func FetchViewedFiles(ctx context.Context, httpClient *http.Client, owner, repo string, number int) (map[string]bool, error) {
	viewed := map[string]bool{}
	var after *string
	for {
		var data struct {
			Repository struct {
				PullRequest struct {
					Files struct {
						Nodes []struct {
							Path              string `json:"path"`
							ViewerViewedState string `json:"viewerViewedState"`
						} `json:"nodes"`
						PageInfo struct {
							HasNextPage bool   `json:"hasNextPage"`
							EndCursor   string `json:"endCursor"`
						} `json:"pageInfo"`
					} `json:"files"`
				} `json:"pullRequest"`
			} `json:"repository"`
		}
		vars := map[string]any{"owner": owner, "repo": repo, "number": number, "after": after}
		if err := runGraphQL(ctx, httpClient, viewedFilesQuery, vars, &data); err != nil {
			return nil, err
		}
		files := data.Repository.PullRequest.Files
		for _, f := range files.Nodes {
			if f.ViewerViewedState == "VIEWED" {
				viewed[f.Path] = true
			}
		}
		if !files.PageInfo.HasNextPage {
			return viewed, nil
		}
		after = &files.PageInfo.EndCursor
	}
}

// SetFileViewed marks or unmarks path as viewed by the authenticated user on the PR.
func SetFileViewed(ctx context.Context, httpClient *http.Client, prNodeID, path string, viewed bool) error {
	mutation := "markFileAsViewed"
	if !viewed {
		mutation = "unmarkFileAsViewed"
	}
	query := fmt.Sprintf(`mutation($id: ID!, $path: String!) { %s(input: {pullRequestId: $id, path: $path}) { clientMutationId } }`, mutation)
	return runGraphQL(ctx, httpClient, query, map[string]any{"id": prNodeID, "path": path}, nil)
}
