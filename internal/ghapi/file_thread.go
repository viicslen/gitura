package ghapi

import (
	"context"
	"net/http"
)

const addFileReviewThreadMutation = `
mutation($reviewID: ID!, $path: String!, $body: String!) {
  addPullRequestReviewThread(input: {pullRequestReviewId: $reviewID, path: $path, body: $body, subjectType: FILE}) {
    thread { id }
  }
}
`

// AddFileReviewThread adds a comment on the whole file at path to the pending review reviewNodeID.
// Returns a "github:" prefixed error on API or HTTP failures.
func AddFileReviewThread(ctx context.Context, httpClient *http.Client, reviewNodeID, path, body string) error {
	return runGraphQL(ctx, httpClient, addFileReviewThreadMutation,
		map[string]any{"reviewID": reviewNodeID, "path": path, "body": body}, nil)
}
