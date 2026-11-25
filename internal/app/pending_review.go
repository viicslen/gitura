package app

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/google/go-github/v93/github"

	"gitura/internal/ghapi"
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
	pending, err := a.findPendingReview()
	if err != nil {
		return model.PendingReviewDTO{}, fmt.Errorf("github: list reviews: %w", err)
	}
	if pending == nil {
		return model.PendingReviewDTO{HasPending: false}, nil
	}
	reviewID, reviewCommit := pending.GetID(), pending.GetCommitID()

	drafts, err := ghapi.FetchPendingReviewDrafts(
		a.ctx, ghapi.NewHTTPClient(a.authToken), a.prOwner, a.prRepo, a.prNumber, reviewID,
	)
	if err != nil {
		// Adopting the review without its comments would let the next submit delete them.
		return model.PendingReviewDTO{}, err
	}
	a.pendingReviewID = reviewID
	a.pendingHeadSHA = reviewCommit
	a.pendingComments = drafts

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
	if comment.Line == 0 {
		subject := "file"
		prComment.Line, prComment.Side, prComment.SubjectType = nil, nil, &subject
	} else if comment.StartLine > 0 {
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

	// A synced or half-built pending review is recreated below from the local comments;
	// GitHub allows one pending review per user, so it has to go first.
	if err := a.dropPendingReview(); err != nil {
		return model.ReviewSubmitResult{}, err
	}

	var lineComments []*github.DraftReviewComment
	var fileComments []model.DraftCommentDTO
	for _, c := range a.pendingComments {
		if c.Line == 0 {
			fileComments = append(fileComments, c)
		} else {
			lineComments = append(lineComments, draftCommentToGitHub(c))
		}
	}

	commitSHA := a.reviewCommitSHA()
	request := &github.PullRequestReviewRequest{
		CommitID: &commitSHA,
		Body:     &req.Body,
		Event:    &req.Verdict,
		Comments: lineComments,
	}
	var review *github.PullRequestReview
	var err error
	if len(fileComments) == 0 {
		review, _, err = a.ghClient.PullRequests.CreateReview(a.ctx, a.prOwner, a.prRepo, a.prNumber, request)
	} else {
		review, err = a.submitReviewWithFileComments(request, fileComments)
	}
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

// dropPendingReview deletes the pending review on GitHub, if any; one already gone counts as deleted.
func (a *App) dropPendingReview() error {
	if a.pendingReviewID == 0 {
		return nil
	}
	_, resp, err := a.ghClient.PullRequests.DeletePendingReview(a.ctx, a.prOwner, a.prRepo, a.prNumber, a.pendingReviewID)
	if err != nil && (resp == nil || resp.StatusCode != http.StatusNotFound) {
		return fmt.Errorf("github: delete pending review: %w", err)
	}
	a.pendingReviewID = 0
	return nil
}

// submitReviewWithFileComments submits req plus file-level comments, which the REST review
// endpoint cannot take: they join the review over GraphQL while it is still pending.
func (a *App) submitReviewWithFileComments(req *github.PullRequestReviewRequest, files []model.DraftCommentDTO) (*github.PullRequestReview, error) {
	event, body := *req.Event, req.Body
	req.Event, req.Body = nil, nil
	pending, resp, err := a.ghClient.PullRequests.CreateReview(a.ctx, a.prOwner, a.prRepo, a.prNumber, req)
	if err != nil {
		if resp != nil && resp.StatusCode >= http.StatusInternalServerError {
			a.adoptOrphanedPendingReview()
		}
		return nil, err
	}
	// Should a step below fail, a retry deletes this half-built review before starting over.
	a.pendingReviewID = pending.GetID()

	httpClient := ghapi.NewHTTPClient(a.authToken)
	for _, c := range files {
		if err := ghapi.AddFileReviewThread(a.ctx, httpClient, pending.GetNodeID(), c.Path, c.Body); err != nil {
			return nil, err
		}
	}
	review, _, err := a.ghClient.PullRequests.SubmitReview(a.ctx, a.prOwner, a.prRepo, a.prNumber, pending.GetID(),
		github.PullRequestSubmitReviewRequest{Body: body, Event: event})
	if err != nil {
		return nil, err
	}
	a.pendingReviewID = 0
	return review, nil
}

// adoptOrphanedPendingReview records a pending review GitHub created despite answering 5xx,
// so the retry deletes it instead of failing on GitHub's one-pending-review-per-user rule.
func (a *App) adoptOrphanedPendingReview() {
	r, err := a.findPendingReview()
	if err != nil {
		logger.L.Warn("SubmitReview: could not check for an orphaned pending review", "err", err)
		return
	}
	if r != nil {
		a.pendingReviewID = r.GetID()
		logger.L.Info("SubmitReview: adopted pending review created by a failed request", "review_id", r.GetID())
	}
}

// findPendingReview returns the authenticated user's PENDING review on the loaded PR, or nil.
// GitHub shows PENDING reviews only to their author, so any one found is ours.
func (a *App) findPendingReview() (*github.PullRequestReview, error) {
	opts := &github.ListOptions{PerPage: 100}
	for {
		reviews, resp, err := a.ghClient.PullRequests.ListReviews(a.ctx, a.prOwner, a.prRepo, a.prNumber, opts)
		if err != nil {
			return nil, err
		}
		for _, r := range reviews {
			if r.GetState() == "PENDING" {
				return r, nil
			}
		}
		if resp.NextPage == 0 {
			return nil, nil
		}
		opts.Page = resp.NextPage
	}
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
