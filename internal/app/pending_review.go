package app

import (
	"cmp"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/google/go-github/v93/github"

	"gitura/internal/ghapi"
	"gitura/internal/logger"
	"gitura/internal/model"
)

// pendingReviewState returns the current in-memory pending review state.
// Returns PendingReviewDTO{HasPending: false} when no pending review exists.
func (a *App) pendingReviewState() model.PendingReviewDTO {
	if len(a.pendingComments) == 0 {
		return model.PendingReviewDTO{HasPending: false, InFlightReview: a.parentAttempt.inFlight()}
	}
	comments := make([]model.DraftCommentDTO, len(a.pendingComments))
	copy(comments, a.pendingComments)
	return model.PendingReviewDTO{
		Comments:       comments,
		HasPending:     true,
		HeadSHA:        a.parentHeadSHA(),
		InFlightReview: a.parentAttempt.inFlight(),
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
	a.prMu.Lock()
	defer a.prMu.Unlock()
	if a.local.Load() != nil {
		return a.pendingReviewState(), nil
	}
	if a.ghClient == nil {
		return model.PendingReviewDTO{}, fmt.Errorf("auth: not authenticated")
	}
	if a.prCache == nil {
		return model.PendingReviewDTO{}, fmt.Errorf("notfound: no PR loaded; call LoadPR first")
	}

	// If we already have local state, return it directly.
	if len(a.pendingComments) > 0 || a.parentAttempt != nil {
		return a.pendingReviewState(), nil
	}
	return a.restorePendingReview()
}

// restorePendingReview loads the user's PENDING review on the loaded PR from GitHub into memory.
func (a *App) restorePendingReview() (model.PendingReviewDTO, error) {
	pending, err := a.findPendingReview(a.prOwner, a.prRepo, a.prNumber)
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
	if err := a.loadPRFiles(); err != nil {
		return model.PendingReviewDTO{}, err
	}
	a.pendingReviewID = reviewID
	a.pendingHeadSHA = reviewCommit
	a.pendingComments = nil
	for _, d := range drafts {
		d.Pointer = a.restoredOnPointer(d)
		a.pendingComments = append(a.pendingComments, a.withID(d))
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

// restoredOnPointer reports whether d, a draft restored from GitHub on the loaded PR's own files, is on a pointer
// row. GitHub keeps no row kind, so between a pointer row and a file at its path the one whose hunks alone hold d's
// lines wins, then the pointer if d sits on a "Subproject commit" line, else the file. Callers hold prMu.
func (a *App) restoredOnPointer(d model.DraftCommentDTO) bool {
	var rows []*github.CommitFile
	for _, f := range a.prFilesCache {
		if a.prSubmoduleOf[f] == nil && f.GetFilename() == d.Path {
			rows = append(rows, f)
		}
	}
	if len(rows) == 1 {
		return a.pointerOf(rows[0]) != nil
	}
	if d.Line == 0 {
		return false
	}
	rows = slices.DeleteFunc(rows, func(f *github.CommitFile) bool { return !draftInHunks(d, ghapi.ParseCommitFileDiff(f).Hunks) })
	if len(rows) == 1 {
		return a.pointerOf(rows[0]) != nil
	}
	return len(rows) > 1 && endsOnPointerLine(d.DiffHunk)
}

// endsOnPointerLine reports whether the last line of hunk is a "Subproject commit" line.
func endsOnPointerLine(hunk string) bool {
	last := hunk[strings.LastIndex(hunk, "\n")+1:]
	return len(last) > 0 && strings.HasPrefix(last[1:], "Subproject commit ")
}

// AddDraftComment adds a comment to the local pending review batch.
// Comments are stored in-memory only; no GitHub API calls are made during
// the draft phase. All comments are submitted together by SubmitReview.
// The comment's Submodule names the repository of the file commented on, as its FileKeyDTO does.
// Error prefix: "auth:" — no client; "notfound:" — no PR loaded; "conflict:" — drafts on an older commit;
// "validation:" — Submodule is not a submodule the loaded change has that file in.
func (a *App) AddDraftComment(comment model.DraftCommentDTO) (model.PendingReviewDTO, error) {
	a.prMu.Lock()
	defer a.prMu.Unlock()
	if l := a.local.Load(); l != nil {
		if err := l.checkSubmodule(comment); err != nil {
			return model.PendingReviewDTO{}, err
		}
	} else if err := a.prDraft(&comment); err != nil {
		return model.PendingReviewDTO{}, err
	}
	a.pendingComments = append(a.pendingComments, a.withID(comment))
	logger.L.Info("AddDraftComment (local)", "path", comment.Path, "line", comment.Line, "total", len(a.pendingComments))
	return a.pendingReviewState(), nil
}

// withID is d numbered as a new pending draft.
func (a *App) withID(d model.DraftCommentDTO) model.DraftCommentDTO {
	a.draftSeq++
	d.ID = a.draftSeq
	return d
}

// checkCommentHead rejects a comment written against another head than the loaded PR's. Callers hold prMu.
func (a *App) checkCommentHead(comment model.DraftCommentDTO) error {
	if comment.HeadSHA != "" && comment.HeadSHA != a.prCache.HeadSHA {
		return fmt.Errorf("conflict: the pull request changed while the comment was being added; reload and try again")
	}
	return nil
}

// prDraft checks that comment can join the loaded PR's drafts and records its submodule's bump. Callers hold prMu.
func (a *App) prDraft(comment *model.DraftCommentDTO) error {
	if a.ghClient == nil {
		return fmt.Errorf("auth: not authenticated")
	}
	if a.prCache == nil {
		return fmt.Errorf("notfound: no PR loaded; call LoadPR first")
	}
	if err := a.checkCommentHead(*comment); err != nil {
		return err
	}
	if err := a.loadPRFiles(); err != nil {
		return err
	}
	if head := a.parentHeadSHA(); comment.Submodule == "" && head != "" && head != a.prCache.HeadSHA {
		// One review has one commit_id; mixing line numbers from two commits would misplace or reject comments.
		return fmt.Errorf("conflict: pending comments were written against an older commit; submit or discard them first")
	}
	s, err := a.commentSubmodule(*comment)
	if err != nil {
		return err
	}
	comment.Bump = s.bump()
	if s == nil {
		a.pendingHeadSHA = a.prCache.HeadSHA
	}
	return nil
}

// parentHeadSHA is the commit the pending drafts on the loaded PR's own files were written on, "" when there are
// none: drafts on submodule files are pinned to their bump instead.
func (a *App) parentHeadSHA() string {
	if !slices.ContainsFunc(a.pendingComments, func(d model.DraftCommentDTO) bool { return d.Submodule == "" }) {
		return ""
	}
	return a.pendingHeadSHA
}

// commentSubmodule returns the bump comment.Submodule names, nil for a comment on the loaded PR's own file.
// Callers hold prMu.
func (a *App) commentSubmodule(comment model.DraftCommentDTO) (*prSubmodule, error) {
	if comment.Submodule == "" {
		return nil, nil
	}
	s := a.prSubmodules[comment.Submodule]
	if comment.Range != "" && (s == nil || comment.Range != s.rng) {
		return nil, fmt.Errorf("conflict: submodule %s changed while the comment was being added; reload and try again", comment.Submodule)
	}
	if a.bumpFile(s, comment.Path, comment.Pointer) == nil {
		return nil, fmt.Errorf("validation: %q is not a file this pull request changes in submodule %q", comment.Path, comment.Submodule)
	}
	return s, nil
}

// UpdateDraftComment replaces the body of the pending draft at index, provided it still reads expected.
// Error prefixes: "validation:" — empty body; "conflict:" — draft moved or changed, or its last submit may
// have reached GitHub.
func (a *App) UpdateDraftComment(index int, expected, body string) (model.PendingReviewDTO, error) {
	if strings.TrimSpace(body) == "" {
		return model.PendingReviewDTO{}, fmt.Errorf("validation: comment body is empty")
	}
	a.prMu.Lock()
	defer a.prMu.Unlock()
	if err := a.checkDraft(index, expected); err != nil {
		return model.PendingReviewDTO{}, err
	}
	a.pendingComments[index].Body = body
	return a.pendingReviewState(), nil
}

// DeleteDraftComment removes the pending draft at index, provided it still reads expected.
// Removing the last draft discards the whole pending review.
// Error prefixes: "conflict:" — draft moved or changed, or its last submit may have reached GitHub;
// "github:" — API error.
func (a *App) DeleteDraftComment(index int, expected string) (model.PendingReviewDTO, error) {
	a.prMu.Lock()
	defer a.prMu.Unlock()
	if err := a.checkDraft(index, expected); err != nil {
		return model.PendingReviewDTO{}, err
	}
	if len(a.pendingComments) == 1 {
		// Otherwise a synced GitHub pending review would be restored, comment and all, on the next sync.
		if err := a.discardPendingReview(); err != nil {
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
	if a.uncertain[a.pendingComments[index].ID] != nil {
		return errFrozen
	}
	return nil
}

// PostImmediateComment posts a standalone inline comment immediately (not as a
// draft review comment). The comment is immediately visible on GitHub. A comment on a submodule
// file goes to the submodule's open PR when its lines are in that PR's diff, and otherwise to the
// loaded PR's conversation, quoting the file and lines.
// Error prefix: "auth:" — no client; "notfound:" — no PR loaded; "conflict:" — written against another head,
// or the submodule PR changed since it was loaded; "validation:" — Submodule is not a submodule the loaded PR has that file in; "github:" — API error.
func (a *App) PostImmediateComment(comment model.DraftCommentDTO) (model.ReviewCommentDTO, error) {
	a.prMu.Lock()
	defer a.prMu.Unlock()
	if a.ghClient == nil {
		return model.ReviewCommentDTO{}, fmt.Errorf("auth: not authenticated")
	}
	if a.prCache == nil {
		return model.ReviewCommentDTO{}, fmt.Errorf("notfound: no PR loaded; call LoadPR first")
	}
	s, err := a.immediateSubmodule(comment)
	if err != nil {
		return model.ReviewCommentDTO{}, err
	}
	owner, repo, number, commit := a.prOwner, a.prRepo, a.prNumber, a.prCache.HeadSHA
	if s != nil {
		if !a.toSubmodulePR(s, comment) {
			return a.postFoldedComment(comment)
		}
		if err := a.recheckImmediate(s, comment); err != nil {
			return model.ReviewCommentDTO{}, err
		}
		owner, repo, number, commit = s.Owner, s.Repo, s.pr.GetNumber(), s.pr.GetHead().GetSHA()
		comment.Path = s.rel(comment.Path)
	}
	key := immediateKey{fmt.Sprintf("%s/%s#%d", owner, repo, number), commit, draftKey(comment)}
	created, err := a.postReviewComment(owner, repo, number, commit, comment, a.sendOnce(key))
	if s != nil && hasPendingReview(err) {
		return model.ReviewCommentDTO{}, pendingReviewError(s)
	}
	if err != nil {
		return model.ReviewCommentDTO{}, fmt.Errorf("github: post immediate comment: %w", err)
	}

	logger.L.Info("PostImmediateComment", "repo", owner+"/"+repo, "path", comment.Path, "line", comment.Line, "id", created.GetID())
	return model.ReviewCommentDTO{
		ID:           created.GetID(),
		Body:         created.GetBody(),
		AuthorLogin:  created.GetUser().GetLogin(),
		AuthorAvatar: created.GetUser().GetAvatarURL(),
		CreatedAt:    created.GetCreatedAt().UTC().Format("2006-01-02T15:04:05Z"),
		HTMLURL:      created.GetHTMLURL(),
	}, nil
}

// recheckImmediate reads s's PR again before comment goes there, and refuses it when that PR no longer takes it.
func (a *App) recheckImmediate(s *prSubmodule, comment model.DraftCommentDTO) error {
	name := s.prName()
	dropped, err := a.revalidateSubmodulePR(s, false)
	if err != nil {
		return err
	}
	if dropped || !a.toSubmodulePR(s, comment) {
		return fmt.Errorf("conflict: %s changed since it was loaded and no longer takes this comment there; nothing was posted, reload and try again", name)
	}
	return nil
}

// immediateSubmodule checks that comment can be posted now and returns the bump its file is in, nil for a
// file of the loaded PR. Callers hold prMu.
func (a *App) immediateSubmodule(comment model.DraftCommentDTO) (*prSubmodule, error) {
	if err := a.checkCommentHead(comment); err != nil {
		return nil, err
	}
	if err := a.loadPRFiles(); err != nil {
		return nil, err
	}
	return a.commentSubmodule(comment)
}

// postReviewComment posts comment on its own, outside any review, on PR owner/repo#number at commit, through post.
func (a *App) postReviewComment(owner, repo string, number int, commit string, comment model.DraftCommentDTO, post sender) (*github.PullRequestComment, error) {
	prComment := github.CreatePullRequestCommentRequest{
		Path:     comment.Path,
		Body:     comment.Body,
		CommitID: commit,
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

	var created *github.PullRequestComment
	err := post(func(start time.Time) (string, bool, error) {
		c, err := a.postedReviewComment(owner, repo, number, prComment, start)
		created = c
		return c.GetHTMLURL(), c != nil, err
	}, func() (*github.Response, error) {
		c, resp, err := a.ghClient.PullRequests.CreateComment(a.ctx, owner, repo, number, prComment)
		if err == nil {
			created = c
			a.markClaimed(false, c.GetID())
		}
		return resp, err
	})
	return created, err
}

// postFoldedComment posts a comment on a submodule file as a conversation comment on the loaded PR.
func (a *App) postFoldedComment(comment model.DraftCommentDTO) (model.ReviewCommentDTO, error) {
	body := foldComments([]model.DraftCommentDTO{comment})
	key := immediateKey{where: fmt.Sprintf("%s/%s#%d", a.prOwner, a.prRepo, a.prNumber), comment: commentKey{body: body}}
	var created *github.IssueComment
	err := a.sendOnce(key)(func(start time.Time) (string, bool, error) {
		c, err := a.postedIssueComment(a.prOwner, a.prRepo, a.prNumber, body, start)
		created = c
		return c.GetHTMLURL(), c != nil, err
	}, func() (*github.Response, error) {
		c, resp, err := a.ghClient.Issues.CreateComment(a.ctx, a.prOwner, a.prRepo, a.prNumber, github.IssueCommentRequest{Body: body})
		if err == nil {
			created = c
		}
		return resp, err
	})
	if err != nil {
		return model.ReviewCommentDTO{}, fmt.Errorf("github: post comment: %w", err)
	}
	logger.L.Info("PostImmediateComment: folded into the PR conversation", "path", comment.Path, "id", created.GetID())
	return model.ReviewCommentDTO{
		ID:           created.GetID(),
		Body:         created.GetBody(),
		AuthorLogin:  created.GetUser().GetLogin(),
		AuthorAvatar: created.GetUser().GetAvatarURL(),
		CreatedAt:    created.GetCreatedAt().UTC().Format("2006-01-02T15:04:05Z"),
		HTMLURL:      created.GetHTMLURL(),
	}, nil
}

// SubmitReview submits all pending draft comments as a single review.
// If an existing GitHub pending review was synced, it is deleted first so
// there is no conflict when creating the new submitted review.
// Drafts on submodule files go first, as one review per submodule PR that has their lines in its
// diff, or as single comments there, unless req.Submodules folds them; the rest are quoted in the
// review body. A retry after a failure does not repost the submodule reviews or comments that went through.
// Every check, and every lookup of what an earlier attempt sent, runs before anything is written.
// Side effect: clears pendingReviewID and pendingComments on success.
// Error prefix: "auth:" — no client; "notfound:" — no PR loaded; "validation:" — bad verdict or
// submodule choice, or an empty review; "conflict:" — a submodule PR closed or moved on since it was
// loaded, or a choice differs from that of an earlier attempt that may have reached GitHub; "github:" — API error.
func (a *App) SubmitReview(req model.ReviewSubmitRequest) (model.ReviewSubmitResult, error) {
	a.prMu.Lock()
	defer a.prMu.Unlock()
	if a.ghClient == nil {
		return model.ReviewSubmitResult{}, fmt.Errorf("auth: not authenticated")
	}
	if a.prCache == nil {
		return model.ReviewSubmitResult{}, fmt.Errorf("notfound: no PR loaded; call LoadPR first")
	}
	parentURL, quoted, err := a.prepareSubmit(req)
	if err != nil {
		return model.ReviewSubmitResult{}, err
	}
	if parentURL != "" {
		return a.reconciledParent(parentURL, quoted)
	}

	// A synced, half-built or still-pending review is recreated below from the local comments;
	// GitHub allows one pending review per user, so it has to go first.
	if err := a.dropPendingReview(); err != nil {
		return model.ReviewSubmitResult{}, err
	}
	if err := a.postSubmoduleReviews(req.Submodules); err != nil {
		return model.ReviewSubmitResult{}, err
	}

	comments, folded := a.splitSubmoduleDrafts()
	result := model.ReviewSubmitResult{SubmoduleReviewURLs: a.subReviewURLs, SubmoduleCommentURLs: a.subCommentURLs, FoldedComments: len(folded)}
	body := withFolded(req.Body, folded)
	if a.onlySubmoduleReviews(comments, body, req.Verdict) {
		a.resetDrafts()
		return result, nil
	}
	review, err := a.submitParentReview(model.ReviewSubmitRequest{Verdict: req.Verdict, Body: body}, comments)
	// The body quotes the folded drafts, so they go with the review.
	a.settle(a.pendingComments, err, model.InFlightReviewDTO{Verdict: req.Verdict, Body: req.Body}, nil)
	if a.parentAttempt != nil {
		a.parentAttempt.folded = len(folded)
	}
	if err != nil {
		return model.ReviewSubmitResult{}, fmt.Errorf("github: submit review: %w", err)
	}

	a.resetDrafts()
	logger.L.Info("SubmitReview complete", "review_id", review.GetID(), "verdict", req.Verdict, "folded", len(folded))
	result.ReviewID, result.HTMLURL = review.GetID(), review.GetHTMLURL()
	return result, nil
}

// prepareSubmit checks req, settles earlier uncertain sends, then checks what is left to send, all without
// writing; parentURL is set when an earlier attempt at the loaded PR's review turned out to have gone through,
// folded to the submodule drafts its text quoted.
func (a *App) prepareSubmit(req model.ReviewSubmitRequest) (parentURL string, folded int, err error) {
	if err := a.loadPRFiles(); err != nil {
		return "", 0, err
	}
	if err := a.checkSubmit(req); err != nil {
		return "", 0, err
	}
	if parentURL, folded, err = a.reconcileUncertain(); err != nil || parentURL != "" {
		return parentURL, folded, err
	}
	if err := a.revalidateTargets(req.Submodules); err != nil {
		return "", 0, err
	}
	return "", 0, a.checkParentReview(req)
}

// reconciledParent ends a submit whose earlier attempt at the loaded PR's review, at url, turned out to have
// gone through, quoting folded submodule drafts; drafts added since stay pending for another submit.
func (a *App) reconciledParent(url string, folded int) (model.ReviewSubmitResult, error) {
	if n := len(a.pendingComments); n > 0 {
		return model.ReviewSubmitResult{}, fmt.Errorf("conflict: your earlier review did go through (%s); the %d comments added since are still pending, submit again to post them", url, n)
	}
	result := model.ReviewSubmitResult{HTMLURL: url, SubmoduleReviewURLs: a.subReviewURLs, SubmoduleCommentURLs: a.subCommentURLs, FoldedComments: folded}
	a.resetDrafts()
	return result, nil
}

// checkSubmit rejects req's verdict and submodule choices, and choices made on routing other than the drafts' now.
func (a *App) checkSubmit(req model.ReviewSubmitRequest) error {
	if !slices.Contains(reviewVerdicts, req.Verdict) {
		return fmt.Errorf("validation: unknown verdict %q", req.Verdict)
	}
	if err := a.checkSubmoduleChoices(req.Submodules); err != nil {
		return err
	}
	if err := a.checkFrozen(req); err != nil {
		return err
	}
	return a.checkRouting(req.Submodules)
}

// revalidateTargets reads again each submodule PR drafts are about to go to. One closed or moved on since is
// dropped, so its drafts fold, and one retargeted has its files read again; either fails the submit before
// anything is sent when it changes which drafts go there, as SubmoduleDrafts counted them, or when a pending
// review of ours is left there, which is deleted first.
func (a *App) revalidateTargets(choices []model.SubmoduleReviewChoice) error {
	for _, g := range a.submoduleDraftGroups() {
		if choiceFor(choices, g.sub.path).Mode == "fold" {
			continue
		}
		s := g.sub
		pr, before := orphanReview{s.Owner, s.Repo, s.pr.GetNumber(), 0}, a.routing(s)
		base := s.pr.GetBase().GetSHA()
		if _, err := a.revalidateSubmodulePR(s, false); err != nil {
			return err
		}
		var had bool
		var derr error
		if s.prName() == "" || s.pr.GetBase().GetSHA() != base {
			had, derr = a.dropOrphansOn(pr)
		}
		if had || !slices.Equal(before, a.routing(s)) {
			return targetConflict(pr, s.path, derr)
		}
	}
	return nil
}

func targetConflict(pr orphanReview, path string, derr error) error {
	msg := fmt.Sprintf("conflict: %s/%s#%d is closed, retargeted or no longer at the commit submodule %s is bumped to; nothing was sent, check where its comments go and submit again", pr.owner, pr.repo, pr.number, path)
	if derr != nil {
		msg += fmt.Sprintf(" (your unpublished pending review there could not be deleted, the next submit or discard tries again: %v)", derr)
	}
	return errors.New(msg)
}

// dropOrphansOn deletes the orphans on pr's pull request, keeping those it fails to delete; had reports
// whether there were any.
func (a *App) dropOrphansOn(pr orphanReview) (had bool, err error) {
	var kept []orphanReview
	for _, o := range a.orphans {
		if o.owner != pr.owner || o.repo != pr.repo || o.number != pr.number {
			kept = append(kept, o)
			continue
		}
		had = true
		if derr := a.deletePendingReview(o.owner, o.repo, o.number, o.id); derr != nil {
			kept, err = append(kept, o), derr
		}
	}
	a.orphans = kept
	return had, err
}

// routing says, for each pending draft on a file of s, whether it can go to s's PR.
func (a *App) routing(s *prSubmodule) []bool {
	var r []bool
	for _, d := range a.pendingComments {
		if d.Submodule == s.path {
			r = append(r, a.canGoToSubmodulePR(d))
		}
	}
	return r
}

// checkParentReview rejects req when, with its drafts routed to submodule PRs as req says, the review left
// for the loaded PR would be empty yet still sent: GitHub refuses an empty Request changes or Comment review.
func (a *App) checkParentReview(req model.ReviewSubmitRequest) error {
	if strings.TrimSpace(req.Body) != "" || req.Verdict == "APPROVE" {
		return nil
	}
	posted := len(a.subReviewURLs)+len(a.subCommentURLs) > 0
	for _, d := range a.pendingComments {
		if d.Submodule == "" || !a.canGoToSubmodulePR(d) || choiceFor(req.Submodules, d.Submodule).Mode == "fold" {
			return nil
		}
		posted = true
	}
	switch {
	case req.Verdict == "REQUEST_CHANGES":
		return fmt.Errorf("validation: requesting changes needs a message or a comment on this pull request")
	case !posted:
		return fmt.Errorf("validation: the review is empty; add a message or a comment")
	}
	return nil
}

// splitSubmoduleDrafts splits the pending drafts into those for the loaded PR and those on submodule files.
func (a *App) splitSubmoduleDrafts() (comments, folded []model.DraftCommentDTO) {
	for _, c := range a.pendingComments {
		if c.Submodule != "" {
			folded = append(folded, c)
		} else {
			comments = append(comments, c)
		}
	}
	return comments, folded
}

// onlySubmoduleReviews reports whether the submodule reviews said everything: GitHub rejects an empty COMMENT review.
func (a *App) onlySubmoduleReviews(comments []model.DraftCommentDTO, body, verdict string) bool {
	return len(comments) == 0 && strings.TrimSpace(body) == "" && verdict == "COMMENT" && len(a.subReviewURLs)+len(a.subCommentURLs) > 0
}

// resetDrafts drops the pending review state.
func (a *App) resetDrafts() {
	a.pendingReviewID = 0
	a.pendingComments = nil
	a.pendingHeadSHA = ""
	a.subReviewURLs, a.subCommentURLs, a.orphans = nil, nil, nil
	a.claimed, a.uncertain, a.parentAttempt = nil, nil, nil
}

// submitParentReview submits req with comments on the loaded PR.
func (a *App) submitParentReview(req model.ReviewSubmitRequest, comments []model.DraftCommentDTO) (*github.PullRequestReview, error) {
	var lineComments []*github.DraftReviewComment
	var fileComments []model.DraftCommentDTO
	for _, c := range comments {
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
	if len(fileComments) > 0 {
		return a.submitReviewWithFileComments(request, fileComments)
	}
	return a.createReview(a.prOwner, a.prRepo, a.prNumber, request)
}

type draftGroup struct {
	sub    *prSubmodule
	drafts []model.DraftCommentDTO
}

// postSubmoduleReviews posts the drafts that can go to submodule PRs as one review per PR, as
// choices say, and drops each review's drafts once it is posted. Folded groups keep their drafts.
func (a *App) postSubmoduleReviews(choices []model.SubmoduleReviewChoice) error {
	for _, g := range a.submoduleDraftGroups() {
		c := choiceFor(choices, g.sub.path)
		switch c.Mode {
		case "fold":
			continue
		case "comments":
			if err := a.postSubmoduleComments(g); err != nil {
				return err
			}
			continue
		}
		url, err := a.postSubmoduleReview(g.sub, g.drafts, c)
		a.settle(g.drafts, err, model.InFlightReviewDTO{Verdict: c.Verdict, Body: c.Body}, g.sub)
		if err != nil {
			return submoduleError(g.sub, "submit review", err)
		}
		a.subReviewURLs = append(a.subReviewURLs, url)
		a.pendingComments = slices.DeleteFunc(a.pendingComments, func(d model.DraftCommentDTO) bool {
			return d.Submodule == g.sub.path && a.canGoToSubmodulePR(d)
		})
	}
	return nil
}

// postSubmoduleComments posts g's drafts one at a time as single comments on its submodule's PR,
// each leaving the pending drafts as soon as it is posted.
func (a *App) postSubmoduleComments(g draftGroup) error {
	s := g.sub
	for _, d := range g.drafts {
		rel := d
		rel.Path = s.rel(d.Path)
		created, err := a.postReviewComment(s.Owner, s.Repo, s.pr.GetNumber(), s.pr.GetHead().GetSHA(), rel, sendUnlessPosted)
		a.settle([]model.DraftCommentDTO{d}, err, model.InFlightReviewDTO{}, s)
		if err != nil {
			return submoduleError(s, "comment", err)
		}
		a.subCommentURLs = append(a.subCommentURLs, created.GetHTMLURL())
		if i := slices.Index(a.pendingComments, d); i >= 0 {
			a.pendingComments = slices.Delete(a.pendingComments, i, i+1)
		}
	}
	return nil
}

// submoduleDraftGroups groups the pending drafts that can go to a submodule PR by submodule.
func (a *App) submoduleDraftGroups() []draftGroup {
	var groups []draftGroup
	for _, d := range a.pendingComments {
		if !a.canGoToSubmodulePR(d) {
			continue
		}
		s := a.prSubmodules[d.Submodule]
		i := slices.IndexFunc(groups, func(g draftGroup) bool { return g.sub == s })
		if i < 0 {
			groups, i = append(groups, draftGroup{sub: s}), len(groups)
		}
		groups[i].drafts = append(groups[i].drafts, d)
	}
	return groups
}

// postSubmoduleReview posts drafts as a review on s's PR with c's verdict and text, and returns its URL.
func (a *App) postSubmoduleReview(s *prSubmodule, drafts []model.DraftCommentDTO, c model.SubmoduleReviewChoice) (string, error) {
	var lineComments []*github.DraftReviewComment
	var fileComments []model.DraftCommentDTO
	for _, d := range drafts {
		d.Path = s.rel(d.Path)
		if d.Line == 0 {
			fileComments = append(fileComments, d)
		} else {
			lineComments = append(lineComments, draftCommentToGitHub(d))
		}
	}
	req := &github.PullRequestReviewRequest{CommitID: new(s.pr.GetHead().GetSHA()), Comments: lineComments}
	if len(fileComments) == 0 {
		req.Body, req.Event = &c.Body, &c.Verdict
		return a.createSubmoduleReview(s, req)
	}
	return a.postSubmoduleFileReview(s, req, fileComments, c.Body, c.Verdict)
}

// postSubmoduleFileReview posts req plus file-level comments as a review on s's PR: file-level comments can only
// join a pending review, over GraphQL.
func (a *App) postSubmoduleFileReview(s *prSubmodule, req *github.PullRequestReviewRequest, fileComments []model.DraftCommentDTO, body, verdict string) (string, error) {
	owner, repo, number := s.Owner, s.Repo, s.pr.GetNumber()
	pending, err := a.createReview(owner, repo, number, req)
	if err != nil {
		return "", err
	}
	review, err := a.finishSubmoduleReview(s, pending, fileComments, body, verdict)
	var u *uncertainError
	if errors.As(err, &u) {
		return "", err
	}
	if err != nil {
		// Left behind, it would block the retry: GitHub allows one pending review per user.
		if derr := a.deletePendingReview(owner, repo, number, pending.GetID()); derr != nil {
			logger.L.Warn("could not delete half-built submodule review", "pr", s.prName(), "err", derr)
			a.orphans = append(a.orphans, orphanReview{owner, repo, number, pending.GetID()})
		}
		return "", err
	}
	return review.GetHTMLURL(), nil
}

func (a *App) finishSubmoduleReview(s *prSubmodule, pending *github.PullRequestReview, files []model.DraftCommentDTO, body, verdict string) (*github.PullRequestReview, error) {
	httpClient := ghapi.NewHTTPClient(a.authToken)
	for _, c := range files {
		if err := ghapi.AddFileReviewThread(a.ctx, httpClient, pending.GetNodeID(), c.Path, c.Body); err != nil {
			return nil, err
		}
	}
	return a.submitPending(orphanReview{s.Owner, s.Repo, s.pr.GetNumber(), pending.GetID()}, github.PullRequestSubmitReviewRequest{Body: &body, Event: verdict})
}

// createSubmoduleReview posts req, a review with only line comments, on s's PR and returns its URL.
func (a *App) createSubmoduleReview(s *prSubmodule, req *github.PullRequestReviewRequest) (string, error) {
	review, err := a.createReview(s.Owner, s.Repo, s.pr.GetNumber(), req)
	return review.GetHTMLURL(), err
}

// submoduleError is err, from sending drafts to s's PR to what, as SubmitReview reports it.
func submoduleError(s *prSubmodule, what string, err error) error {
	if hasPendingReview(err) {
		return pendingReviewError(s)
	}
	return fmt.Errorf("github: %s on %s: %w", what, s.prName(), err)
}

func pendingReviewError(s *prSubmodule) error {
	return fmt.Errorf("github: you already have a pending review on %s; submit or discard it on GitHub", s.prName())
}

// hasPendingReview reports whether err is GitHub refusing a new review because the user already has a pending one on that PR.
func hasPendingReview(err error) bool {
	var e *github.ErrorResponse
	return errors.As(err, &e) && e.Response != nil && e.Response.StatusCode == http.StatusUnprocessableEntity &&
		strings.Contains(e.Error(), "pending review")
}

// orphanReview is a pending review a failed submit left on owner/repo#number.
type orphanReview struct {
	owner, repo string
	number      int
	id          int64
}

// dropPendingReview deletes the pending review on the loaded PR and those failed submits left, if any.
func (a *App) dropPendingReview() error {
	if a.pendingReviewID != 0 {
		if err := a.deletePendingReview(a.prOwner, a.prRepo, a.prNumber, a.pendingReviewID); err != nil {
			return err
		}
		a.pendingReviewID = 0
	}
	for len(a.orphans) > 0 {
		o := a.orphans[0]
		if err := a.deletePendingReview(o.owner, o.repo, o.number, o.id); err != nil {
			return err
		}
		a.orphans = a.orphans[1:]
	}
	return nil
}

// deletePendingReview deletes review id on owner/repo#number; one already gone counts as deleted.
func (a *App) deletePendingReview(owner, repo string, number int, id int64) error {
	_, resp, err := a.ghClient.PullRequests.DeletePendingReview(a.ctx, owner, repo, number, id)
	if err != nil && (resp == nil || resp.StatusCode != http.StatusNotFound) {
		return fmt.Errorf("github: delete pending review: %w", err)
	}
	return nil
}

// submitReviewWithFileComments submits req plus file-level comments, which the REST review
// endpoint cannot take: they join the review over GraphQL while it is still pending.
func (a *App) submitReviewWithFileComments(req *github.PullRequestReviewRequest, files []model.DraftCommentDTO) (*github.PullRequestReview, error) {
	event, body := *req.Event, req.Body
	req.Event, req.Body = nil, nil
	pending, err := a.createReview(a.prOwner, a.prRepo, a.prNumber, req)
	if err != nil {
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
	review, err := a.submitPending(orphanReview{a.prOwner, a.prRepo, a.prNumber, pending.GetID()}, github.PullRequestSubmitReviewRequest{Body: body, Event: event})
	var u *uncertainError
	if err == nil || errors.As(err, &u) {
		// An uncertain review is the next submit's to look up, not to delete.
		a.pendingReviewID = 0
	}
	return review, err
}

// pendingBefore returns the ID of the viewer's pending review on owner/repo#number, 0 when there is none.
func (a *App) pendingBefore(owner, repo string, number int) (int64, error) {
	r, err := a.findPendingReview(owner, repo, number)
	if err != nil {
		return 0, fmt.Errorf("github: list reviews: %w", err)
	}
	return r.GetID(), nil
}

// findPendingReview returns the authenticated user's PENDING review on owner/repo#number, or nil.
// GitHub shows PENDING reviews only to their author, so any one found is ours.
func (a *App) findPendingReview(owner, repo string, number int) (*github.PullRequestReview, error) {
	return a.findReview(owner, repo, number, func(r *github.PullRequestReview) bool { return r.GetState() == "PENDING" })
}

// findReview returns the first review on owner/repo#number that match accepts, or nil.
func (a *App) findReview(owner, repo string, number int, match func(*github.PullRequestReview) bool) (*github.PullRequestReview, error) {
	for r, err := range a.ghClient.PullRequests.ListReviewsIter(a.ctx, owner, repo, number, &github.ListOptions{PerPage: 100}) {
		if err != nil {
			return nil, err
		}
		if match(r) {
			return r, nil
		}
	}
	return nil, nil
}

// DiscardPendingReview clears the local pending review state and deletes from GitHub the synced
// pending review and those failed submits left, if any are still pending.
// Error prefix: "auth:" — no client; "github:" — API error, the drafts are kept.
func (a *App) DiscardPendingReview() error {
	a.prMu.Lock()
	defer a.prMu.Unlock()
	return a.discardPendingReview()
}

// discardPendingReview is DiscardPendingReview for callers holding prMu. A review it can't read or delete
// stays remembered, with the drafts, for the next discard or submit.
func (a *App) discardPendingReview() error {
	if err := a.findUnknownPending(); err != nil {
		return err
	}
	remembered := a.rememberedPending()
	if len(remembered) > 0 && a.ghClient == nil {
		return fmt.Errorf("auth: not authenticated")
	}
	var kept []orphanReview
	var errs []error
	for _, p := range remembered {
		if err := a.dropIfPending(p); err != nil {
			kept, errs = append(kept, p), append(errs, err)
		}
	}
	if len(kept) > 0 {
		a.pendingReviewID, a.orphans = 0, kept
		return errors.Join(errs...)
	}
	a.resetDrafts()
	logger.L.Info("DiscardPendingReview: cleared state")
	return nil
}

// rememberedPending lists the pending reviews this app may have left on GitHub: the loaded PR's, the orphans,
// and those whose submit has an unknown outcome.
func (a *App) rememberedPending() []orphanReview {
	all := slices.Clone(a.orphans)
	if a.pendingReviewID != 0 {
		all = append(all, orphanReview{a.prOwner, a.prRepo, a.prNumber, a.pendingReviewID})
	}
	for _, u := range a.attempts() {
		if u.pending != nil && !slices.Contains(all, *u.pending) {
			all = append(all, *u.pending)
		}
	}
	return all
}

// dropIfPending deletes review p if it is still pending; one gone or submitted is left alone.
func (a *App) dropIfPending(p orphanReview) error {
	r, gone, err := a.readReview(p)
	switch {
	case gone:
		return nil
	case err != nil:
		return fmt.Errorf("github: check pending review %d on %s/%s#%d: %w", p.id, p.owner, p.repo, p.number, err)
	case r.GetState() != "PENDING":
		return nil
	}
	return a.deletePendingReview(p.owner, p.repo, p.number, p.id)
}

// reviewCommitSHA is the commit a review is submitted against: the one pending drafts on the PR's own files were
// written on, else the PR head.
func (a *App) reviewCommitSHA() string {
	return cmp.Or(a.parentHeadSHA(), a.prCache.HeadSHA)
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
