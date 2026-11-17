package ghapi

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v93/github"

	"gitura/internal/model"
)

// FetchCommits fetches the commits of a pull request, oldest first.
// Errors are prefixed with "github:".
func FetchCommits(ctx context.Context, client *github.Client, owner, repo string, number int) ([]model.CommitDTO, error) {
	opts := &github.ListOptions{PerPage: 100}
	var all []*github.RepositoryCommit
	for {
		commits, resp, err := client.PullRequests.ListCommits(ctx, owner, repo, number, opts)
		if err != nil {
			return nil, fmt.Errorf("github: list PR commits %s/%s#%d: %w", owner, repo, number, err)
		}
		all = append(all, commits...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	result := make([]model.CommitDTO, 0, len(all))
	for _, c := range all {
		login := c.GetAuthor().GetLogin()
		if login == "" {
			login = c.GetCommit().GetAuthor().GetName()
		}
		headline, _, _ := strings.Cut(c.GetCommit().GetMessage(), "\n")
		result = append(result, model.CommitDTO{
			SHA:          c.GetSHA(),
			Headline:     headline,
			AuthorLogin:  login,
			AuthorAvatar: c.GetAuthor().GetAvatarURL(),
			// The committer date moves on rebase and amend, so it tracks when the commit reached the branch.
			CommittedAt: c.GetCommit().GetCommitter().GetDate().UTC().Format(time.RFC3339),
			HTMLURL:     c.GetHTMLURL(),
		})
	}
	return result, nil
}
