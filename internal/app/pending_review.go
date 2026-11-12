package app

import (
	"fmt"
	"slices"
	"strings"

	"github.com/google/go-github/v93/github"

	"gitura/internal/logger"
	"gitura/internal/model"
)

// pendingReviewState returns the current in-memory pending review state.
// Returns PendingReviewDTO{HasPending: false} when no pending review exists.
func (a *App) pendingReviewState() model.PendingReviewDTO {
	if len(a.pendingComments) == 0 {
		return model.PendingReviewDTO{HasPending: false}
	}
	comments := make([]model.DraftCommentDTO, len(a.pendingComments))
	copy(comments, a.pendingComments)
	return model.PendingReviewDTO{
		Comments:   comments,
		HasPending: true,
		HeadSHA:    a.pendingHeadSHA,
	}
}

// SyncPendingReview queries GitHub for any PENDING review on the current PR
// that belongs to the authenticated user, and restores in-memory state from it.
// This recovers pending-review state across app restarts. If in-memory state
// already exists it is returned immediately without a network call.
// GitHub only returns PENDING reviews to their own author, so any PENDING entry
// in the list is guaranteed to belong to us.
// Error prefix: "auth:" — no client; "notfound:" — no PR loaded; "github:" — API error.
func (a *App) SyncPendingReview() (model.PendingReviewDTO, error) {
	if a.local != nil {
		return a.pendingReviewState(), nil
	}
	if a.ghClient == nil {
		return model.PendingReviewDTO{}, fmt.Errorf("auth: not authenticated")
	}
	if a.prCache == nil {
		return model.PendingReviewDTO{}, fmt.Errorf("notfound: no PR loaded; call LoadPR first")
	}

	// If we already have local state, return it directly.
	if len(a.pendingComments) > 0 {
		return a.pendingReviewState(), nil
	}
	return a.restorePendingReview()
}

// restorePendingReview loads the user's PENDING review on the loaded PR from GitHub into memory.
func (a *App) restorePendingReview() (model.PendingReviewDTO, error) {
	// List all reviews; PENDING reviews are only visible to their author,
	// so any PENDING entry belongs to the authenticated user.
	reviews, _, err := a.ghClient.PullRequests.ListReviews(
		a.ctx, a.prOwner, a.prRepo, a.prNumber, nil,
	)
	if err != nil {
		return model.PendingReviewDTO{}, fmt.Errorf("github: list reviews: %w", err)
	}

	var reviewID int64
	var reviewCommit string
	for _, r := range reviews {
		if r.GetState() == "PENDING" {
			reviewID = r.GetID()
			reviewCommit = r.GetCommitID()
			break
		}
	}
	if reviewID == 0 {
		return model.PendingReviewDTO{HasPending: false}, nil
	}

	a.pendingReviewID = reviewID
	a.pendingHeadSHA = reviewCommit

	// Fetch the comments for this review to populate the local cache.
	rawComments, _, err := a.ghClient.PullRequests.ListReviewComments(
		a.ctx, a.prOwner, a.prRepo, a.prNumber, reviewID, nil,
	)
	if err != nil {
		// Non-fatal: we have the review ID; comment list will just be empty.
		logger.L.Warn("SyncPendingReview: failed to fetch review comments", "err", err)
		a.pendingComments = nil
	} else {
		a.pendingComments = make([]model.DraftCommentDTO, 0, len(rawComments))
		for _, c := range rawComments {
			a.pendingComments = append(a.pendingComments, model.DraftCommentDTO{
				Path:      c.GetPath(),
				Body:      c.GetBody(),
				Line:      c.GetLine(),
				Side:      c.GetSide(),
				StartLine: c.GetStartLine(),
				StartSide: c.GetStartSide(),
			})
		}
	}

	dtos := make([]model.DraftCommentDTO, len(a.pendingComments))
	copy(dtos, a.pendingComments)
	logger.L.Info("SyncPendingReview: restored pending review", "review_id", reviewID, "comments", len(dtos))
	return model.PendingReviewDTO{
		ReviewID:   reviewID,
		Comments:   dtos,
		HasPending: true,
		HeadSHA:    a.pendingHeadSHA,
	}, nil
}

// AddDraftComment adds a comment to the local pending review batch.
// Comments are stored in-memory only; no GitHub API calls are made during
// the draft phase. All comments are submitted together by SubmitReview.
// Error prefix: "auth:" — no client; "notfound:" — no PR loaded.
func (a *App) AddDraftComment(comment model.DraftCommentDTO) (model.PendingReviewDTO, error) {
	if a.local == nil {
		if a.ghClient == nil {
			return model.PendingReviewDTO{}, fmt.Errorf("auth: not authenticated")
		}
		if a.prCache == nil {
			return model.PendingReviewDTO{}, fmt.Errorf("notfound: no PR loaded; call LoadPR first")
		}
		if len(a.pendingComments) == 0 || a.pendingHeadSHA == "" {
			a.pendingHeadSHA = a.prCache.HeadSHA
		} else if a.pendingHeadSHA != a.prCache.HeadSHA {
			// One review has one commit_id; mixing line numbers from two commits would misplace or reject comments.
			return model.PendingReviewDTO{}, fmt.Errorf("conflict: pending comments were written against an older commit; submit or discard them first")
		}
	}
	a.pendingComments = append(a.pendingComments, comment)
	logger.L.Info("AddDraftComment (local)", "path", comment.Path, "line", comment.Line, "total", len(a.pendingComments))

	comments := make([]model.DraftCommentDTO, len(a.pendingComments))
	copy(comments, a.pendingComments)
	return model.PendingReviewDTO{
		Comments:   comments,
		HasPending: true,
		HeadSHA:    a.pendingHeadSHA,
	}, nil
}

// UpdateDraftComment replaces the body of the pending draft at index, provided it still reads expected.
// Error prefixes: "validation:" — empty body; "conflict:" — draft moved or changed.
func (a *App) UpdateDraftComment(index int, expected, body string) (model.PendingReviewDTO, error) {
	if strings.TrimSpace(body) == "" {
		return model.PendingReviewDTO{}, fmt.Errorf("validation: comment body is empty")
	}
	if err := a.checkDraft(index, expected); err != nil {
		return model.PendingReviewDTO{}, err
	}
	a.pendingComments[index].Body = body
	return a.pendingReviewState(), nil
}

// DeleteDraftComment removes the pending draft at index, provided it still reads expected.
// Removing the last draft discards the whole pending review.
// Error prefixes: "conflict:" — draft moved or changed; "github:" — API error.
func (a *App) DeleteDraftComment(index int, expected string) (model.PendingReviewDTO, error) {
	if err := a.checkDraft(index, expected); err != nil {
		return model.PendingReviewDTO{}, err
	}
	if len(a.pendingComments) == 1 {
		// Otherwise a synced GitHub pending review would be restored, comment and all, on the next sync.
		if err := a.DiscardPendingReview(); err != nil {
			return model.PendingReviewDTO{}, err
		}
		return model.PendingReviewDTO{}, nil
	}
	a.pendingComments = slices.Delete(a.pendingComments, index, index+1)
	return a.pendingReviewState(), nil
}

func (a *App) checkDraft(index int, expected string) error {
	if index < 0 || index >= len(a.pendingComments) || a.pendingComments[index].Body != expected {
		return fmt.Errorf("conflict: pending comment changed; reload and try again")
	}
	return nil
}

// PostImmediateComment posts a standalone inline comment immediately (not as a
// draft review comment). The comment is immediately visible on GitHub.
// Error prefix: "auth:" — no client; "notfound:" — no PR loaded; "github:" — API error.
func (a *App) PostImmediateComment(comment model.DraftCommentDTO) (model.ReviewCommentDTO, error) {
	if a.ghClient == nil {
		return model.ReviewCommentDTO{}, fmt.Errorf("auth: not authenticated")
	}
	if a.prCache == nil {
		return model.ReviewCommentDTO{}, fmt.Errorf("notfound: no PR loaded; call LoadPR first")
	}

	prComment := github.CreatePullRequestCommentRequest{
		Path:     comment.Path,
		Body:     comment.Body,
		CommitID: a.prCache.HeadSHA,
		Line:     &comment.Line,
		Side:     &comment.Side,
	}
	if comment.StartLine > 0 {
		prComment.StartLine = &comment.StartLine
		prComment.StartSide = &comment.StartSide
	}

	created, _, err := a.ghClient.PullRequests.CreateComment(
		a.ctx, a.prOwner, a.prRepo, a.prNumber, prComment,
	)
	if err != nil {
		return model.ReviewCommentDTO{}, fmt.Errorf("github: post immediate comment: %w", err)
	}

	logger.L.Info("PostImmediateComment", "path", comment.Path, "line", comment.Line, "id", created.GetID())
	return model.ReviewCommentDTO{
		ID:           created.GetID(),
		Body:         created.GetBody(),
		AuthorLogin:  created.GetUser().GetLogin(),
		AuthorAvatar: created.GetUser().GetAvatarURL(),
		CreatedAt:    created.GetCreatedAt().UTC().Format("2006-01-02T15:04:05Z"),
	}, nil
}

// SubmitReview submits all pending draft comments as a single review.
// If an existing GitHub pending review was synced, it is deleted first so
// there is no conflict when creating the new submitted review.
// All locally-buffered comments are sent together with the verdict in one
// CreateReview call.
// Side effect: clears pendingReviewID and pendingComments on success.
// Error prefix: "auth:" — no client; "notfound:" — no PR loaded; "github:" — API error.
func (a *App) SubmitReview(req model.ReviewSubmitRequest) (model.ReviewSubmitResult, error) {
	if a.ghClient == nil {
		return model.ReviewSubmitResult{}, fmt.Errorf("auth: not authenticated")
	}
	if a.prCache == nil {
		return model.ReviewSubmitResult{}, fmt.Errorf("notfound: no PR loaded; call LoadPR first")
	}

	// If we synced an existing GitHub pending review, delete it first.
	// We recreate it as a submitted review with all local comments below.
	if a.pendingReviewID != 0 {
		_, _, delErr := a.ghClient.PullRequests.DeletePendingReview(
			a.ctx, a.prOwner, a.prRepo, a.prNumber, a.pendingReviewID,
		)
		if delErr != nil {
			logger.L.Warn("SubmitReview: failed to delete old pending review (continuing)", "review_id", a.pendingReviewID, "err", delErr)
		}
		a.pendingReviewID = 0
	}

	draftComments := make([]*github.DraftReviewComment, len(a.pendingComments))
	for i, c := range a.pendingComments {
		draftComments[i] = draftCommentToGitHub(c)
	}

	commitSHA := a.reviewCommitSHA()
	review, _, err := a.ghClient.PullRequests.CreateReview(
		a.ctx, a.prOwner, a.prRepo, a.prNumber,
		&github.PullRequestReviewRequest{
			CommitID: &commitSHA,
			Body:     &req.Body,
			Event:    &req.Verdict,
			Comments: draftComments,
		},
	)
	if err != nil {
		return model.ReviewSubmitResult{}, fmt.Errorf("github: submit review: %w", err)
	}

	a.pendingComments = nil
	a.pendingHeadSHA = ""
	logger.L.Info("SubmitReview complete", "review_id", review.GetID(), "verdict", req.Verdict)
	return model.ReviewSubmitResult{
		ReviewID: review.GetID(),
		HTMLURL:  review.GetHTMLURL(),
	}, nil
}

// DiscardPendingReview clears the local pending review state and, if a GitHub
// pending review was synced, deletes it from GitHub too.
// Error prefix: "auth:" — no client; "github:" — API error.
func (a *App) DiscardPendingReview() error {
	if a.pendingReviewID != 0 {
		if a.ghClient == nil {
			return fmt.Errorf("auth: not authenticated")
		}
		if _, _, err := a.ghClient.PullRequests.DeletePendingReview(
			a.ctx, a.prOwner, a.prRepo, a.prNumber, a.pendingReviewID,
		); err != nil {
			return fmt.Errorf("github: delete pending review: %w", err)
		}
	}
	a.pendingReviewID = 0
	a.pendingComments = nil
	a.pendingHeadSHA = ""
	logger.L.Info("DiscardPendingReview: cleared state")
	return nil
}

// reviewCommitSHA is the commit a review is submitted against: the one pending drafts were written on, else the PR head.
func (a *App) reviewCommitSHA() string {
	if len(a.pendingComments) > 0 && a.pendingHeadSHA != "" {
		return a.pendingHeadSHA
	}
	return a.prCache.HeadSHA
}

// draftCommentToGitHub converts a DraftCommentDTO to a go-github DraftReviewComment.
func draftCommentToGitHub(c model.DraftCommentDTO) *github.DraftReviewComment {
	dc := &github.DraftReviewComment{
		Path: &c.Path,
		Body: &c.Body,
		Line: &c.Line,
		Side: &c.Side,
	}
	if c.StartLine > 0 {
		dc.StartLine = &c.StartLine
		dc.StartSide = &c.StartSide
	}
	return dc
}
