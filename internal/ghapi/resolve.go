package ghapi

import (
	"context"
	"net/http"
)

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

// ResolveThread marks a review thread as resolved via the GitHub GraphQL API.
// threadNodeID is the GraphQL global ID (e.g. "PRRT_kwDO...").
// Returns a "github:" prefixed error on API or HTTP failures.
func ResolveThread(ctx context.Context, httpClient *http.Client, threadNodeID string) error {
	return runThreadMutation(ctx, httpClient, resolveThreadMutation, threadNodeID)
}

// UnresolveThread marks a review thread as unresolved via the GitHub GraphQL API.
// threadNodeID is the GraphQL global ID (e.g. "PRRT_kwDO...").
// Returns a "github:" prefixed error on API or HTTP failures.
func UnresolveThread(ctx context.Context, httpClient *http.Client, threadNodeID string) error {
	return runThreadMutation(ctx, httpClient, unresolveThreadMutation, threadNodeID)
}

// runThreadMutation executes a resolve or unresolve GraphQL mutation.
func runThreadMutation(ctx context.Context, httpClient *http.Client, mutation, threadNodeID string) error {
	return runGraphQL(ctx, httpClient, mutation, map[string]any{"threadID": threadNodeID}, nil)
}
