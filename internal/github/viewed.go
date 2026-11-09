package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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

// postGraphQL sends a GraphQL request and returns the response once its HTTP status is OK.
func postGraphQL(ctx context.Context, httpClient *http.Client, query string, vars map[string]any) (*http.Response, error) {
	body, err := json.Marshal(map[string]any{"query": query, "variables": vars})
	if err != nil {
		return nil, fmt.Errorf("github: marshal GraphQL request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubGraphQLURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("github: create GraphQL request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: GraphQL request failed: %w", err)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("auth: GitHub API returned 401 — token invalid or missing scope")
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("github: GraphQL request returned HTTP %d", resp.StatusCode)
	}
	return resp, nil
}

// runGraphQL posts a GraphQL request and decodes its "data" into out (when non-nil).
func runGraphQL(ctx context.Context, httpClient *http.Client, query string, vars map[string]any, out any) error {
	resp, err := postGraphQL(ctx, httpClient, query, vars)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	var gqlResp struct {
		Data   json.RawMessage `json:"data"`
		Errors []graphQLError  `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return fmt.Errorf("github: decode GraphQL response: %w", err)
	}
	if len(gqlResp.Errors) > 0 {
		msgs := make([]string, len(gqlResp.Errors))
		for i, e := range gqlResp.Errors {
			msgs[i] = e.Message
		}
		return fmt.Errorf("github: GraphQL errors: %s", strings.Join(msgs, "; "))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(gqlResp.Data, out); err != nil {
		return fmt.Errorf("github: decode GraphQL data: %w", err)
	}
	return nil
}
