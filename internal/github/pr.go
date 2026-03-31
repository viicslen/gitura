package github

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/go-github/v67/github"

	"gitura/internal/model"
)

// FetchPR fetches a single pull request from GitHub using the REST API.
// It returns a populated PullRequestSummary on success.
// On HTTP 404 the error is prefixed with "notfound:".
// On any other API error the error is prefixed with "github:".
func FetchPR(ctx context.Context, client *github.Client, owner, repo string, number int) (*model.PullRequestSummary, error) {
	pr, resp, err := client.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("notfound: pull request %s/%s#%d not found", owner, repo, number)
		}
		return nil, fmt.Errorf("github: fetch PR %s/%s#%d: %w", owner, repo, number, err)
	}

	summary := &model.PullRequestSummary{
		ID:         pr.GetID(),
		Number:     pr.GetNumber(),
		Title:      pr.GetTitle(),
		State:      pr.GetState(),
		IsDraft:    pr.GetDraft(),
		Body:       pr.GetBody(),
		HeadBranch: pr.GetHead().GetRef(),
		BaseBranch: pr.GetBase().GetRef(),
		HeadSHA:    pr.GetHead().GetSHA(),
		NodeID:     pr.GetNodeID(),
		HTMLURL:    pr.GetHTMLURL(),
		Owner:      owner,
		Repo:       repo,
	}
	return summary, nil
}
