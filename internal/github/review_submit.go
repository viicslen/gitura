package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/go-github/v67/github"

	"gitura/internal/model"
)

// githubRESTBaseURL is the GitHub REST API base URL.
// It is a package-level variable to allow overriding in tests.
var githubRESTBaseURL = "https://api.github.com"

// addCommentRequest is the JSON body sent to the "add comment to pending review"
// endpoint, which is not implemented in go-github/v67.
type addCommentRequest struct {
	Path      string `json:"path"`
	Body      string `json:"body"`
	Line      int    `json:"line"`
	Side      string `json:"side"`
	StartLine int    `json:"start_line,omitempty"`
	StartSide string `json:"start_side,omitempty"`
}

// CreatePendingReview creates a new empty pending review on the given pull
// request and returns the review ID. Callers should store this ID and use it
// with AddCommentToPendingReview and SubmitReview.
// Errors are prefixed with "github:".
func CreatePendingReview(ctx context.Context, client *github.Client, owner, repo string, number int) (int64, error) {
	review, _, err := client.PullRequests.CreateReview(ctx, owner, repo, number, &github.PullRequestReviewRequest{
		// Omitting Event creates a PENDING review.
	})
	if err != nil {
		return 0, fmt.Errorf("github: create pending review for %s/%s#%d: %w", owner, repo, number, err)
	}
	return review.GetID(), nil
}

// AddCommentToPendingReview adds a single inline comment to an existing pending
// review using a raw HTTP call. go-github/v67 does not implement the
// POST .../reviews/{review_id}/comments endpoint.
//
// httpClient must be an authenticated client (e.g. obtained from
// oauth2.NewClient). Errors are prefixed with "github:".
func AddCommentToPendingReview(
	ctx context.Context,
	httpClient *http.Client,
	owner, repo string,
	number int,
	reviewID int64,
	comment model.DraftCommentDTO,
) error {
	body := addCommentRequest{
		Path: comment.Path,
		Body: comment.Body,
		Line: comment.Line,
		Side: comment.Side,
	}
	if comment.StartLine > 0 {
		body.StartLine = comment.StartLine
		body.StartSide = comment.StartSide
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("github: marshal add-comment request: %w", err)
	}

	rawURL := fmt.Sprintf(
		"%s/repos/%s/%s/pulls/%d/reviews/%d/comments",
		githubRESTBaseURL, owner, repo, number, reviewID,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("github: create add-comment request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("github: add comment to pending review: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("github: add comment returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// SubmitReview submits a pending review with the given verdict.
// req.Verdict must be one of "APPROVE", "REQUEST_CHANGES", or "COMMENT".
// Errors are prefixed with "github:".
func SubmitReview(
	ctx context.Context,
	client *github.Client,
	owner, repo string,
	number int,
	reviewID int64,
	req model.ReviewSubmitDTO,
) (model.ReviewSubmitResult, error) {
	review, _, err := client.PullRequests.SubmitReview(ctx, owner, repo, number, reviewID, &github.PullRequestReviewRequest{
		Body:  &req.Body,
		Event: &req.Verdict,
	})
	if err != nil {
		return model.ReviewSubmitResult{}, fmt.Errorf(
			"github: submit review %d for %s/%s#%d: %w", reviewID, owner, repo, number, err)
	}
	return model.ReviewSubmitResult{
		ReviewID: review.GetID(),
		HTMLURL:  review.GetHTMLURL(),
	}, nil
}

// DeletePendingReview deletes a pending review and all its draft comments.
// Errors are prefixed with "github:".
func DeletePendingReview(ctx context.Context, client *github.Client, owner, repo string, number int, reviewID int64) error {
	_, _, err := client.PullRequests.DeletePendingReview(ctx, owner, repo, number, reviewID)
	if err != nil {
		return fmt.Errorf("github: delete pending review %d for %s/%s#%d: %w", reviewID, owner, repo, number, err)
	}
	return nil
}
