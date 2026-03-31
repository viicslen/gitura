package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// resolveGraphQLURL is the GitHub GraphQL API endpoint used for resolve/unresolve
// mutations. It is a package-level variable to allow overriding in tests.
var resolveGraphQLURL = "https://api.github.com/graphql"

const resolveThreadMutation = `
mutation($threadID: ID!) {
  resolveReviewThread(input: {threadId: $threadID}) {
    thread { id isResolved }
  }
}
`

const unresolveThreadMutation = `
mutation($threadID: ID!) {
  unresolveReviewThread(input: {threadId: $threadID}) {
    thread { id isResolved }
  }
}
`

// graphQLMutationResponse is a generic wrapper for mutation responses.
type graphQLMutationResponse struct {
	Errors []graphQLError `json:"errors"`
}

// ResolveThread marks a review thread as resolved via the GitHub GraphQL API.
// threadNodeID is the GraphQL global ID (e.g. "PRRT_kwDO...").
// Returns an "auth:" prefixed error on HTTP 401 and a "github:" prefixed error
// on all other API or HTTP failures.
func ResolveThread(ctx context.Context, httpClient *http.Client, threadNodeID string) error {
	return runThreadMutation(ctx, httpClient, resolveThreadMutation, threadNodeID)
}

// UnresolveThread marks a review thread as unresolved via the GitHub GraphQL API.
// threadNodeID is the GraphQL global ID (e.g. "PRRT_kwDO...").
// Returns an "auth:" prefixed error on HTTP 401 and a "github:" prefixed error
// on all other API or HTTP failures.
func UnresolveThread(ctx context.Context, httpClient *http.Client, threadNodeID string) error {
	return runThreadMutation(ctx, httpClient, unresolveThreadMutation, threadNodeID)
}

// runThreadMutation executes a resolve or unresolve GraphQL mutation.
func runThreadMutation(ctx context.Context, httpClient *http.Client, mutation, threadNodeID string) error {
	payload := map[string]interface{}{
		"query": mutation,
		"variables": map[string]string{
			"threadID": threadNodeID,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("github: marshal GraphQL mutation: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resolveGraphQLURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("github: create GraphQL mutation request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("github: GraphQL mutation request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("auth: GitHub API returned 401 — token invalid or missing scope")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("github: GraphQL mutation returned HTTP %d", resp.StatusCode)
	}

	var gqlResp graphQLMutationResponse
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return fmt.Errorf("github: decode GraphQL mutation response: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		msgs := make([]string, len(gqlResp.Errors))
		for i, e := range gqlResp.Errors {
			msgs[i] = e.Message
		}
		return fmt.Errorf("github: GraphQL errors: %s", strings.Join(msgs, "; "))
	}

	return nil
}
