// Package github provides a factory for creating authenticated GitHub API clients
// and helpers for querying the GitHub Search API.
package github

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/go-github/v67/github"

	"gitura/internal/logger"
	"gitura/internal/model"
)

// repoFromURL extracts the owner and repository name from a GitHub API
// repository URL of the form https://api.github.com/repos/owner/repo.
// Returns empty strings when the URL cannot be parsed.
func repoFromURL(repoURL string) (owner, repo string) {
	trimmed := strings.TrimPrefix(repoURL, "https://api.github.com/repos/")
	parts := strings.SplitN(trimmed, "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", ""
	}
	return parts[0], parts[1]
}

// runQuery executes a paginated GitHub Search Issues query and returns all
// results. Returns the issues, whether any page had incomplete_results, and
// any error encountered.
func runQuery(
	ctx context.Context,
	client *github.Client,
	query string,
) ([]*github.Issue, bool, error) {
	opts := &github.SearchOptions{
		Sort:  "updated",
		Order: "desc",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	var all []*github.Issue
	incomplete := false

	for {
		result, resp, err := client.Search.Issues(ctx, query, opts)
		if err != nil {
			return all, incomplete, err
		}
		if result.GetIncompleteResults() {
			incomplete = true
		}
		all = append(all, result.Issues...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return all, incomplete, nil
}

// issueToItem converts a *github.Issue from the Search API to a PRListItem,
// tagging the involvement types relative to the authenticated user's login.
// Returns (item, true) on success; (zero, false) when required fields are missing.
func issueToItem(issue *github.Issue, login string) (model.PRListItem, bool) {
	owner, repo := repoFromURL(issue.GetRepositoryURL())
	if owner == "" || repo == "" {
		return model.PRListItem{}, false
	}
	if issue.GetTitle() == "" || issue.GetHTMLURL() == "" {
		return model.PRListItem{}, false
	}

	// Derive involvement tags so the frontend can apply involvement toggles locally.
	// Since the query already uses is:pr, all results are pull requests — we do not
	// need to check PullRequestLinks. The reviewer tag is a best-effort approximation:
	// the Search API does not expose requested_reviewers, so anyone involved who is
	// neither the author nor an assignee is tagged as reviewer.
	isAuthor := strings.EqualFold(issue.GetUser().GetLogin(), login)

	isAssignee := false
	for _, a := range issue.Assignees {
		if strings.EqualFold(a.GetLogin(), login) {
			isAssignee = true
			break
		}
	}

	isReviewer := !isAuthor && !isAssignee

	return model.PRListItem{
		Number:      issue.GetNumber(),
		Title:       issue.GetTitle(),
		Owner:       owner,
		Repo:        repo,
		AuthorLogin: issue.GetUser().GetLogin(),
		CreatedAt:   issue.GetCreatedAt().Time.UTC().Format(time.RFC3339),
		UpdatedAt:   issue.GetUpdatedAt().Time.UTC().Format(time.RFC3339),
		HTMLURL:     issue.GetHTMLURL(),
		IsDraft:     issue.GetDraft(),
		IsAuthor:    isAuthor,
		IsAssignee:  isAssignee,
		IsReviewer:  isReviewer,
	}, true
}

// SearchOpenPRs fetches all open pull requests involving the authenticated user
// using a single GitHub Search query with the involves: qualifier. Results are
// tagged with IsAuthor, IsAssignee, IsReviewer for client-side filtering.
//
// On success, Result.Items is sorted by UpdatedAt descending and contains all
// PRs up to the GitHub Search API limit (1,000 per query).
// On rate limit exhaustion, Result.RateLimitReset is set (RFC3339) and
// Result.Error is non-empty — the caller should surface this to the user.
func SearchOpenPRs(
	ctx context.Context,
	client *github.Client,
	login string,
	filters model.PRListFilters,
) (model.PRListResult, error) {
	// Build query: involves: covers author, assignee, review-requested, and mentioned.
	// All other filter fields (repo, org, author, date) are applied client-side.
	parts := []string{"is:pr", "is:open", "archived:false", fmt.Sprintf("involves:%s", login)}
	if !filters.IncludeDrafts {
		parts = append(parts, "draft:false")
	}
	query := strings.Join(parts, " ")

	logger.L.Debug("SearchOpenPRs query", "query", query, "login", login)

	issues, incomplete, err := runQuery(ctx, client, query)
	if err != nil {
		// Check for primary rate limit.
		var rateLimitErr *github.RateLimitError
		if errors.As(err, &rateLimitErr) {
			resetAt := rateLimitErr.Rate.Reset.Time.UTC().Format(time.RFC3339)
			return model.PRListResult{
				RateLimitReset: resetAt,
				Error:          "rate limit exceeded",
			}, nil
		}
		// Check for secondary / abuse rate limit.
		var abuseErr *github.AbuseRateLimitError
		if errors.As(err, &abuseErr) {
			retryAfter := ""
			if d := abuseErr.GetRetryAfter(); d > 0 {
				retryAfter = time.Now().UTC().Add(d).Format(time.RFC3339)
			}
			return model.PRListResult{
				RateLimitReset: retryAfter,
				Error:          "secondary rate limit exceeded",
			}, nil
		}
		return model.PRListResult{Error: fmt.Sprintf("search error: %v", err)}, nil
	}

	// Sort by UpdatedAt descending.
	sort.Slice(issues, func(i, j int) bool {
		return issues[i].GetUpdatedAt().Time.After(issues[j].GetUpdatedAt().Time)
	})

	// Convert to DTOs, tagging involvement types.
	items := make([]model.PRListItem, 0, len(issues))
	for _, issue := range issues {
		item, ok := issueToItem(issue, login)
		if !ok {
			continue
		}
		logger.L.Debug("SearchOpenPRs item", "number", item.Number, "title", item.Title, "is_author", item.IsAuthor, "is_assignee", item.IsAssignee, "is_reviewer", item.IsReviewer)
		items = append(items, item)
	}

	logger.L.Debug("SearchOpenPRs complete", "total_issues", len(issues), "items", len(items), "incomplete", incomplete)

	return model.PRListResult{
		Items:             items,
		IncompleteResults: incomplete,
	}, nil
}
