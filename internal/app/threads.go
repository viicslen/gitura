package app

import (
	"fmt"
	"slices"
	"strings"

	"github.com/google/go-github/v67/github"

	"gitura/internal/ghapi"
	"gitura/internal/logger"
	"gitura/internal/model"
)

// GetCommentThreads returns the cached review threads for the loaded PR.
// When includeResolved is false, resolved threads are excluded.
// Returns "notfound:" error if no PR has been loaded yet.
func (a *App) GetCommentThreads(includeResolved bool) ([]model.CommentThreadDTO, error) {
	if a.prCache == nil {
		return nil, fmt.Errorf("notfound: no PR loaded; call LoadPR first")
	}
	if includeResolved {
		return a.threads, nil
	}
	result := make([]model.CommentThreadDTO, 0, len(a.threads))
	for _, t := range a.threads {
		if !t.Resolved {
			result = append(result, t)
		}
	}
	return result, nil
}

// ReloadReviewActivity re-fetches review threads and submitted reviews for the
// loaded PR so newly posted comments and reviews appear. Returns all threads, resolved included.
// Error prefixes: "auth:" — no client; "notfound:" — no PR loaded; "github:" — API error.
func (a *App) ReloadReviewActivity() ([]model.CommentThreadDTO, error) {
	if a.ghClient == nil {
		return nil, fmt.Errorf("auth: not authenticated")
	}
	if a.prCache == nil {
		return nil, fmt.Errorf("notfound: no PR loaded; call LoadPR first")
	}
	owner, repo, number := a.prOwner, a.prRepo, a.prNumber
	threads, err := ghapi.FetchReviewThreads(
		a.ctx, ghapi.NewHTTPClient(a.authToken), owner, repo, number, nil,
	)
	if err != nil {
		return nil, err
	}
	reviews, err := ghapi.FetchReviews(a.ctx, a.ghClient, owner, repo, number)
	if err != nil {
		return nil, err
	}
	if !a.isLoadedPR(owner, repo, number) {
		return nil, fmt.Errorf("notfound: pull request changed during reload")
	}
	ignoredSet := make(map[string]struct{}, len(a.ignoredCommenters))
	for _, ic := range a.ignoredCommenters {
		ignoredSet[ic.Login] = struct{}{}
	}
	a.threads = filterIgnoredThreads(threads, ignoredSet)
	a.reviews = ghapi.DecorateReviewsWithThreadStatus(filterIgnoredReviews(reviews, ignoredSet), a.threads)
	return a.threads, nil
}

// ReplyToComment posts a reply to an existing review thread.
// threadRootID identifies the thread by its root comment's database ID.
// Returns the new ReviewCommentDTO on success and appends it to the cached thread.
// Error prefixes: "validation:" — empty body; "notfound:thread" — unknown thread; "github:" — API error.
func (a *App) ReplyToComment(threadRootID int64, body string) (model.ReviewCommentDTO, error) {
	if strings.TrimSpace(body) == "" {
		return model.ReviewCommentDTO{}, fmt.Errorf("validation:body required")
	}
	if a.ghClient == nil {
		return model.ReviewCommentDTO{}, fmt.Errorf("auth: not authenticated")
	}

	if a.threadIndex(threadRootID) == -1 {
		return model.ReviewCommentDTO{}, fmt.Errorf("notfound:thread %d", threadRootID)
	}

	comment, _, err := a.ghClient.PullRequests.CreateCommentInReplyTo(a.ctx, a.prOwner, a.prRepo, a.prNumber, body, threadRootID)
	if err != nil {
		return model.ReviewCommentDTO{}, fmt.Errorf("github: create comment: %w", err)
	}

	dto := model.ReviewCommentDTO{
		ID:           comment.GetID(),
		InReplyToID:  threadRootID,
		ReviewID:     comment.GetPullRequestReviewID(),
		Body:         comment.GetBody(),
		AuthorLogin:  comment.GetUser().GetLogin(),
		AuthorAvatar: comment.GetUser().GetAvatarURL(),
		CreatedAt:    comment.GetCreatedAt().UTC().Format("2006-01-02T15:04:05Z"),
		IsSuggestion: strings.Contains(comment.GetBody(), "```suggestion"),
	}

	// Re-find the thread: ReloadReviewActivity may have replaced a.threads during the API call.
	if i := a.threadIndex(threadRootID); i >= 0 {
		a.threads[i].Comments = append(a.threads[i].Comments, dto)
	}

	logger.L.Info("ReplyToComment complete", "thread_root_id", threadRootID, "new_comment_id", dto.ID)
	return dto, nil
}

// UpdateBodyIfUnchanged replaces the PR description or a comment body with next, but only
// while GitHub still holds expected, so a stale view can't overwrite someone else's edit.
// kind is "pr_body", "issue_comment", "review_comment" or "review"; id is ignored for "pr_body".
// Error prefixes: "conflict:" — the body changed on GitHub; "github:" — API error.
func (a *App) UpdateBodyIfUnchanged(kind string, id int64, expected, next string) (string, error) {
	if a.ghClient == nil {
		return "", fmt.Errorf("auth: not authenticated")
	}
	if a.prCache == nil {
		return "", fmt.Errorf("notfound: no PR loaded")
	}

	var err error
	switch kind {
	case "pr_body":
		err = a.updatePRBody(expected, next)
	case "issue_comment":
		err = a.updateIssueCommentBody(id, expected, next)
	case "review_comment":
		err = a.updateReviewCommentBody(id, expected, next)
	case "review":
		err = a.updateReviewBody(id, expected, next)
	default:
		return "", fmt.Errorf("validation:unknown kind %q", kind)
	}
	if err != nil {
		return "", err
	}

	logger.L.Info("UpdateBodyIfUnchanged complete", "kind", kind, "id", id)
	return next, nil
}

func bodyConflict(current, expected string) error {
	if current != expected {
		return fmt.Errorf("conflict: this comment changed on GitHub; refresh to see the latest")
	}
	return nil
}

func (a *App) updatePRBody(expected, next string) error {
	pr, _, err := a.ghClient.PullRequests.Get(a.ctx, a.prOwner, a.prRepo, a.prNumber)
	if err != nil {
		return fmt.Errorf("github: get pull request: %w", err)
	}
	if err := bodyConflict(pr.GetBody(), expected); err != nil {
		return err
	}
	if _, _, err = a.ghClient.PullRequests.Edit(a.ctx, a.prOwner, a.prRepo, a.prNumber, &github.PullRequest{Body: &next}); err != nil {
		return fmt.Errorf("github: edit pull request: %w", err)
	}
	a.prCache.Body = next
	return nil
}

func (a *App) updateIssueCommentBody(id int64, expected, next string) error {
	c, _, err := a.ghClient.Issues.GetComment(a.ctx, a.prOwner, a.prRepo, id)
	if err != nil {
		return fmt.Errorf("github: get comment: %w", err)
	}
	if err := bodyConflict(c.GetBody(), expected); err != nil {
		return err
	}
	if _, _, err = a.ghClient.Issues.EditComment(a.ctx, a.prOwner, a.prRepo, id, &github.IssueComment{Body: &next}); err != nil {
		return fmt.Errorf("github: edit comment: %w", err)
	}
	for i := range a.issueComments {
		if a.issueComments[i].ID == id {
			a.issueComments[i].Body = next
		}
	}
	return nil
}

func (a *App) updateReviewCommentBody(id int64, expected, next string) error {
	c, _, err := a.ghClient.PullRequests.GetComment(a.ctx, a.prOwner, a.prRepo, id)
	if err != nil {
		return fmt.Errorf("github: get review comment: %w", err)
	}
	if err := bodyConflict(c.GetBody(), expected); err != nil {
		return err
	}
	if _, _, err = a.ghClient.PullRequests.EditComment(a.ctx, a.prOwner, a.prRepo, id, &github.PullRequestComment{Body: &next}); err != nil {
		return fmt.Errorf("github: edit review comment: %w", err)
	}
	for i := range a.threads {
		for j := range a.threads[i].Comments {
			if a.threads[i].Comments[j].ID == id {
				a.threads[i].Comments[j].Body = next
			}
		}
	}
	return nil
}

func (a *App) updateReviewBody(id int64, expected, next string) error {
	r, _, err := a.ghClient.PullRequests.GetReview(a.ctx, a.prOwner, a.prRepo, a.prNumber, id)
	if err != nil {
		return fmt.Errorf("github: get review: %w", err)
	}
	if err := bodyConflict(r.GetBody(), expected); err != nil {
		return err
	}
	if _, _, err = a.ghClient.PullRequests.UpdateReview(a.ctx, a.prOwner, a.prRepo, a.prNumber, id, next); err != nil {
		return fmt.Errorf("github: update review: %w", err)
	}
	for i := range a.reviews {
		if a.reviews[i].ReviewID == id {
			a.reviews[i].Body = next
		}
	}
	return nil
}

// DeleteComment deletes an issue comment or review comment on the loaded PR and drops it from the cache.
// kind is "issue_comment" or "review_comment". Submitted reviews can't be deleted on GitHub.
// Error prefixes: "validation:" — unknown kind; "github:" — API error.
func (a *App) DeleteComment(kind string, id int64) error {
	if a.ghClient == nil {
		return fmt.Errorf("auth: not authenticated")
	}
	if a.prCache == nil {
		return fmt.Errorf("notfound: no PR loaded")
	}

	switch kind {
	case "issue_comment":
		if _, err := a.ghClient.Issues.DeleteComment(a.ctx, a.prOwner, a.prRepo, id); err != nil {
			return fmt.Errorf("github: delete comment: %w", err)
		}
		a.issueComments = slices.DeleteFunc(a.issueComments, func(c model.IssueCommentDTO) bool { return c.ID == id })
	case "review_comment":
		if _, err := a.ghClient.PullRequests.DeleteComment(a.ctx, a.prOwner, a.prRepo, id); err != nil {
			return fmt.Errorf("github: delete review comment: %w", err)
		}
		for i := range a.threads {
			a.threads[i].Comments = slices.DeleteFunc(a.threads[i].Comments, func(c model.ReviewCommentDTO) bool { return c.ID == id })
		}
		a.threads = slices.DeleteFunc(a.threads, func(t model.CommentThreadDTO) bool { return len(t.Comments) == 0 })
	default:
		return fmt.Errorf("validation:unknown kind %q", kind)
	}

	logger.L.Info("DeleteComment complete", "kind", kind, "id", id)
	return nil
}

// ResolveThread marks a review thread as resolved.
// threadRootID identifies the thread by its root comment's database ID.
// Error prefixes: "notfound:thread" — unknown thread; "github:" — API error.
func (a *App) ResolveThread(threadRootID int64) error {
	if a.ghClient == nil {
		return fmt.Errorf("auth: not authenticated")
	}

	threadIdx := -1
	for i, t := range a.threads {
		if t.RootID == threadRootID {
			threadIdx = i
			break
		}
	}
	if threadIdx == -1 {
		return fmt.Errorf("notfound:thread %d", threadRootID)
	}

	nodeID := a.threads[threadIdx].NodeID
	httpClient := ghapi.NewHTTPClient(a.authToken)
	if err := ghapi.ResolveThread(a.ctx, httpClient, nodeID); err != nil {
		return err
	}

	if i := a.threadIndex(threadRootID); i >= 0 {
		a.threads[i].Resolved = true
	}
	a.refreshReviewResolutionState()
	logger.L.Info("ResolveThread complete", "thread_root_id", threadRootID)
	return nil
}

// UnresolveThread marks a review thread as unresolved.
// threadRootID identifies the thread by its root comment's database ID.
// Error prefixes: "notfound:thread" — unknown thread; "github:" — API error.
func (a *App) UnresolveThread(threadRootID int64) error {
	if a.ghClient == nil {
		return fmt.Errorf("auth: not authenticated")
	}

	threadIdx := -1
	for i, t := range a.threads {
		if t.RootID == threadRootID {
			threadIdx = i
			break
		}
	}
	if threadIdx == -1 {
		return fmt.Errorf("notfound:thread %d", threadRootID)
	}

	nodeID := a.threads[threadIdx].NodeID
	httpClient := ghapi.NewHTTPClient(a.authToken)
	if err := ghapi.UnresolveThread(a.ctx, httpClient, nodeID); err != nil {
		return err
	}

	if i := a.threadIndex(threadRootID); i >= 0 {
		a.threads[i].Resolved = false
	}
	a.refreshReviewResolutionState()
	logger.L.Info("UnresolveThread complete", "thread_root_id", threadRootID)
	return nil
}

func (a *App) threadIndex(rootID int64) int {
	for i, t := range a.threads {
		if t.RootID == rootID {
			return i
		}
	}
	return -1
}

func (a *App) refreshReviewResolutionState() {
	a.reviews = ghapi.DecorateReviewsWithThreadStatus(a.reviews, a.threads)
}
