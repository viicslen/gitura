package ghapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// githubGraphQLURL is a variable so tests can point it at a fake server.
var githubGraphQLURL = "https://api.github.com/graphql"

type graphQLError struct {
	Message string `json:"message"`
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
		return nil, fmt.Errorf("auth: GitHub API returned 401 — token invalid or missing 'repo' scope")
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
