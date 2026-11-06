package github

import (
	"context"
	"fmt"
	"sort"
	"time"

	gh "github.com/google/go-github/v67/github"

	"gitura/internal/model"
)

// ListPRComments fetches all general pull request comments (issue comments).
// Errors are prefixed with "github:".
func ListPRComments(ctx context.Context, client *gh.Client, owner, repo string, number int) ([]model.PRCommentDTO, error) {
	opts := &gh.IssueListCommentsOptions{ListOptions: gh.ListOptions{PerPage: 100}}
	var all []*gh.IssueComment
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

	result := make([]model.PRCommentDTO, 0, len(all))
	for _, c := range all {
		result = append(result, model.PRCommentDTO{
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

// ListPRReviews fetches all submitted pull request reviews.
// Pending reviews are excluded. Errors are prefixed with "github:".
func ListPRReviews(ctx context.Context, client *gh.Client, owner, repo string, number int) ([]model.ReviewOverviewDTO, error) {
	opts := &gh.ListOptions{PerPage: 100}
	var all []*gh.PullRequestReview
	for {
		reviews, resp, err := client.PullRequests.ListReviews(ctx, owner, repo, number, opts)
		if err != nil {
			return nil, fmt.Errorf("github: list PR reviews %s/%s#%d: %w", owner, repo, number, err)
		}
		all = append(all, reviews...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	result := make([]model.ReviewOverviewDTO, 0, len(all))
	for _, r := range all {
		if r.GetState() == "PENDING" {
			continue
		}
		result = append(result, model.ReviewOverviewDTO{
			ReviewID:     r.GetID(),
			State:        r.GetState(),
			Body:         r.GetBody(),
			AuthorLogin:  r.GetUser().GetLogin(),
			AuthorAvatar: r.GetUser().GetAvatarURL(),
			SubmittedAt:  submittedAtRFC3339(r),
			HTMLURL:      r.GetHTMLURL(),
		})
	}

	sort.SliceStable(result, func(i, j int) bool {
		return result[i].SubmittedAt < result[j].SubmittedAt
	})

	return result, nil
}

func submittedAtRFC3339(review *gh.PullRequestReview) string {
	if review.SubmittedAt == nil || review.SubmittedAt.IsZero() {
		return ""
	}
	return review.SubmittedAt.UTC().Format(time.RFC3339)
}

// DecorateReviewsWithThreadStatus annotates reviews with thread resolution state
// derived from the currently loaded review threads.
func DecorateReviewsWithThreadStatus(reviews []model.ReviewOverviewDTO, threads []model.CommentThreadDTO) []model.ReviewOverviewDTO {
	stats := make(map[int64]struct{ total, resolved int })
	for _, thread := range threads {
		if thread.ReviewID <= 0 {
			continue
		}
		entry := stats[thread.ReviewID]
		entry.total++
		if thread.Resolved {
			entry.resolved++
		}
		stats[thread.ReviewID] = entry
	}

	decorated := make([]model.ReviewOverviewDTO, len(reviews))
	copy(decorated, reviews)
	for i := range decorated {
		entry := stats[decorated[i].ReviewID]
		decorated[i].HasThreads = entry.total > 0
		decorated[i].ResolvedThreadCount = entry.resolved
		decorated[i].TotalThreadCount = entry.total
		decorated[i].AllThreadsResolved = entry.total > 0 && entry.resolved == entry.total
	}
	return decorated
}
