package app

import (
	"fmt"
	"strings"

	"gitura/internal/ghapi"
	"gitura/internal/logger"
	"gitura/internal/model"
)

// LoadPR fetches PR metadata and all review threads, caches them in memory,
// and returns a PullRequestSummary with CommentCount and UnresolvedCount computed
// from the filtered thread list.
//
// Progress events ("pr:load-progress") are emitted during the paginated GraphQL fetch.
// Ignored-commenter threads are filtered out before caching.
//
// Error prefixes: "auth:" — no client; "notfound:" — PR absent; "github:" — API error.
func (a *App) LoadPR(owner, repo string, number int) (model.PullRequestSummary, error) {
	logger.L.Info("LoadPR called", "owner", owner, "repo", repo, "number", number)

	if a.ghClient == nil {
		return model.PullRequestSummary{}, fmt.Errorf("auth: not authenticated")
	}
	if err := a.loadConfig(); err != nil {
		logger.L.Warn("LoadPR: failed to load ignored commenters", "err", err)
	}

	pr, err := ghapi.FetchPR(a.ctx, a.ghClient, owner, repo, number)
	if err != nil {
		logger.L.Error("FetchPR failed", "err", err)
		return model.PullRequestSummary{}, err
	}

	httpClient := ghapi.NewHTTPClient(a.authToken)
	threads, err := ghapi.FetchReviewThreads(
		a.ctx, httpClient, owner, repo, number,
		func(loaded, total int) {
			a.emit("pr:load-progress", model.LoadProgress{Loaded: loaded, Total: total})
		},
	)
	if err != nil {
		logger.L.Error("FetchReviewThreads failed", "err", err)
		return model.PullRequestSummary{}, err
	}

	// Build ignored-login set for O(1) lookup.
	ignoredSet := make(map[string]struct{}, len(a.ignoredCommenters))
	for _, ic := range a.ignoredCommenters {
		ignoredSet[ic.Login] = struct{}{}
	}

	filtered := filterIgnoredThreads(threads, ignoredSet)
	comments, err := ghapi.FetchIssueComments(a.ctx, a.ghClient, owner, repo, number)
	if err != nil {
		logger.L.Error("FetchIssueComments failed", "err", err)
		return model.PullRequestSummary{}, err
	}
	reviews, err := ghapi.FetchReviews(a.ctx, a.ghClient, owner, repo, number)
	if err != nil {
		logger.L.Error("FetchReviews failed", "err", err)
		return model.PullRequestSummary{}, err
	}
	filteredComments := filterIgnoredIssueComments(comments, ignoredSet)
	decoratedReviews := ghapi.DecorateReviewsWithThreadStatus(filterIgnoredReviews(reviews, ignoredSet), filtered)

	commentCount := len(filtered)
	unresolvedCount := countUnresolved(filtered)

	pr.CommentCount = commentCount
	pr.UnresolvedCount = unresolvedCount
	pr.Owner = owner
	pr.Repo = repo

	samePR := a.isLoadedPR(owner, repo, number)

	// Cache for subsequent GetCommentThreads calls.
	a.prOwner = owner
	a.prRepo = repo
	a.prNumber = number
	a.prCache = pr
	a.threads = filtered
	a.issueComments = filteredComments
	a.reviews = decoratedReviews
	a.prFilesCache = nil
	a.viewedFiles = nil
	a.StopLocalReview()
	// Draft comments live only in memory; a refresh of the same PR must not drop them.
	if !samePR {
		a.pendingReviewID = 0
		a.pendingComments = nil
		a.pendingHeadSHA = ""
	}

	logger.L.Info("LoadPR complete",
		"threads", len(filtered),
		"unresolved", unresolvedCount,
	)
	return *pr, nil
}

func (a *App) isLoadedPR(owner, repo string, number int) bool {
	return a.prCache != nil && a.prOwner == owner && a.prRepo == repo && a.prNumber == number
}

// filterIgnoredThreads returns threads whose root comment author is not in ignored.
// Threads with no comments are always included.
func filterIgnoredThreads(threads []model.CommentThreadDTO, ignored map[string]struct{}) []model.CommentThreadDTO {
	filtered := make([]model.CommentThreadDTO, 0, len(threads))
	for _, t := range threads {
		if len(t.Comments) == 0 {
			filtered = append(filtered, t)
			continue
		}
		if _, ok := ignored[t.Comments[0].AuthorLogin]; !ok {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// filterIgnoredIssueComments returns general PR comments whose authors are not ignored.
func filterIgnoredIssueComments(comments []model.IssueCommentDTO, ignored map[string]struct{}) []model.IssueCommentDTO {
	filtered := make([]model.IssueCommentDTO, 0, len(comments))
	for _, c := range comments {
		if _, ok := ignored[c.AuthorLogin]; ok {
			continue
		}
		filtered = append(filtered, c)
	}
	return filtered
}

// filterIgnoredReviews returns reviews whose authors are not ignored.
func filterIgnoredReviews(reviews []model.ReviewDTO, ignored map[string]struct{}) []model.ReviewDTO {
	filtered := make([]model.ReviewDTO, 0, len(reviews))
	for _, r := range reviews {
		if _, ok := ignored[r.AuthorLogin]; ok {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}

// GetPROverview returns the cached overview data for the loaded PR.
// When includeResolvedReviews is false, reviews whose child conversations are all
// resolved are excluded. Empty reviews with no linked threads are always hidden.
func (a *App) GetPROverview(includeResolvedReviews bool) (model.PROverviewDTO, error) {
	if a.prCache == nil {
		return model.PROverviewDTO{}, fmt.Errorf("notfound: no PR loaded; call LoadPR first")
	}
	comments := make([]model.IssueCommentDTO, len(a.issueComments))
	copy(comments, a.issueComments)
	reviews := make([]model.ReviewDTO, 0, len(a.reviews))
	numbers := make(map[int64]int)
	for _, review := range a.reviews {
		isVerdict := review.State == "APPROVED" || review.State == "CHANGES_REQUESTED"
		if strings.TrimSpace(review.Body) == "" && !review.HasThreads && !isVerdict {
			continue
		}
		numbers[review.ReviewID] = len(numbers) + 1
		if !includeResolvedReviews && review.AllThreadsResolved {
			continue
		}
		reviews = append(reviews, review)
	}
	return model.PROverviewDTO{
		Summary:         *a.prCache,
		GeneralComments: comments,
		Reviews:         reviews,
		ReviewNumbers:   numbers,
	}, nil
}

// countUnresolved returns the number of unresolved threads in the slice.
func countUnresolved(threads []model.CommentThreadDTO) int {
	n := 0
	for _, t := range threads {
		if !t.Resolved {
			n++
		}
	}
	return n
}

// CommitSuggestion applies the suggestion block in a review comment to the PR
// branch and creates a commit.
//
// commentID is the database ID of the comment that contains the suggestion.
// The method finds the comment across all cached threads, resolves the file
// path from its parent thread, and delegates to suggestion.CommitSuggestion.
//
// Error prefixes: "auth:" — no client; "notfound:comment" — unknown comment;
// "validation:" — not a suggestion; "github:conflict" — SHA conflict;
// "github:" — other API errors.
func (a *App) CommitSuggestion(commentID int64, commitMessage string) (model.SuggestionCommitResult, error) {
	if a.ghClient == nil {
		return model.SuggestionCommitResult{}, fmt.Errorf("auth: not authenticated")
	}
	if a.prCache == nil {
		return model.SuggestionCommitResult{}, fmt.Errorf("notfound:comment %d — no PR loaded", commentID)
	}

	// Find the comment and its parent thread.
	var (
		found      model.ReviewCommentDTO
		threadPath string
	)
	for _, t := range a.threads {
		for _, c := range t.Comments {
			if c.ID == commentID {
				found = c
				threadPath = t.Path
				goto done
			}
		}
	}
done:
	if found.ID == 0 {
		return model.SuggestionCommitResult{}, fmt.Errorf("notfound:comment %d", commentID)
	}

	result, err := ghapi.CommitSuggestion(
		a.ctx,
		a.ghClient,
		a.prOwner,
		a.prRepo,
		a.prCache.HeadBranch,
		threadPath,
		found,
		commitMessage,
	)
	if err != nil {
		logger.L.Error("CommitSuggestion failed", "comment_id", commentID, "err", err)
		return model.SuggestionCommitResult{}, err
	}

	logger.L.Info("CommitSuggestion complete", "comment_id", commentID, "commit_sha", result.CommitSHA)
	return result, nil
}

// MergePR merges the loaded PR at its loaded head with method "merge", "squash" or "rebase".
// Error prefix: "auth:" — no client; "notfound:" — no PR loaded; "github:" — API error.
func (a *App) MergePR(method string) error {
	if a.ghClient == nil {
		return fmt.Errorf("auth: not authenticated")
	}
	if a.prCache == nil {
		return fmt.Errorf("notfound: no PR loaded")
	}
	return ghapi.MergePR(a.ctx, a.ghClient, a.prOwner, a.prRepo, a.prNumber, a.prCache.HeadSHA, method)
}

// ClosePR closes the loaded PR without merging.
// Error prefix: "auth:" — no client; "notfound:" — no PR loaded; "github:" — API error.
func (a *App) ClosePR() error {
	if a.ghClient == nil {
		return fmt.Errorf("auth: not authenticated")
	}
	if a.prCache == nil {
		return fmt.Errorf("notfound: no PR loaded")
	}
	return ghapi.ClosePR(a.ctx, a.ghClient, a.prOwner, a.prRepo, a.prNumber)
}
