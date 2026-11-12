package ghapi

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v93/github"

	"gitura/internal/model"
)

// FetchIssueComments fetches all general pull request comments (issue comments).
// Errors are prefixed with "github:".
func FetchIssueComments(ctx context.Context, client *github.Client, owner, repo string, number int) ([]model.IssueCommentDTO, error) {
	opts := &github.IssueListCommentsOptions{PerPage: 100}
	var all []*github.IssueComment
	for {
		comments, resp, err := client.Issues.ListComments(ctx, owner, repo, number, opts)
		if err != nil {
			return nil, fmt.Errorf("github: list PR comments %s/%s#%d: %w", owner, repo, number, err)
		}
		all = append(all, comments...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	result := make([]model.IssueCommentDTO, 0, len(all))
	for _, c := range all {
		result = append(result, model.IssueCommentDTO{
			ID:           c.GetID(),
			Body:         c.GetBody(),
			AuthorLogin:  c.GetUser().GetLogin(),
			AuthorAvatar: c.GetUser().GetAvatarURL(),
			CreatedAt:    c.GetCreatedAt().UTC().Format(time.RFC3339),
			UpdatedAt:    c.GetUpdatedAt().UTC().Format(time.RFC3339),
			HTMLURL:      c.GetHTMLURL(),
		})
	}
	return result, nil
}
