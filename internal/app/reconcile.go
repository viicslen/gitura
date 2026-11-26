package app

import (
	"cmp"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/google/go-github/v93/github"

	"gitura/internal/logger"
	"gitura/internal/model"
)

// clockSkew is how much earlier than the failed attempt, by our clock, GitHub may date what it created.
const clockSkew = time.Minute

// ambiguous reports whether a failed write may still have gone through: GitHub answered 5xx, nothing, or 2xx
// with a body that could not be read or decoded.
func ambiguous(resp *github.Response) bool {
	return resp == nil || resp.StatusCode >= http.StatusInternalServerError || resp.StatusCode/100 == 2
}

// errNoReview is a 2xx answer to a review write that did not identify the review: go-github decodes an empty,
// null or {} body without error.
var errNoReview = errors.New("GitHub's answer did not identify the review")

// reviewStates maps a review event sent to GitHub to the state of the review it creates.
var reviewStates = map[string]string{"APPROVE": "APPROVED", "REQUEST_CHANGES": "CHANGES_REQUESTED", "COMMENT": "COMMENTED"}

// claim is a comment, or a review when review is set, that a submit counts as posted.
type claim struct {
	review bool
	id     int64
}

func (a *App) markClaimed(review bool, id int64) {
	if a.claimed == nil {
		a.claimed = map[claim]bool{}
	}
	a.claimed[claim{review, id}] = true
}

// finder reports whether a send started at the time given went through, and the URL of what it posted.
type finder func(start time.Time) (url string, found bool, err error)

// uncertainError is a send that may have gone through: GitHub's answer and the lookup after it both failed.
// find looks again for what that send, to its destination with its payload, would have posted.
type uncertainError struct {
	start time.Time
	err   error
	find  finder
	// review is set for a review, else the send was a single comment; where names its pull request, and
	// parent is set when that is the loaded PR, else path is the submodule's.
	review, parent bool
	where, path    string
	// sent is the review's verdict and text.
	sent model.InFlightReviewDTO
	// pending is the review a submit of a pending review left, nil for other sends.
	pending *orphanReview
	// unknownPending is set for a create of a pending review: one it left, of unknown ID, blocks the next.
	unknownPending bool
	// folded counts the submodule drafts the loaded PR's review quoted in its text.
	folded int
}

func (e *uncertainError) Error() string { return e.err.Error() }

func (e *uncertainError) Unwrap() error { return e.err }

// errFrozen refuses a change to a draft, or to its submodule's choice, while a send of it has an unknown outcome.
var errFrozen = errors.New("conflict: this comment's earlier submit may have reached GitHub; submit again to settle it first")

// mode is the submodule choice u sent its drafts with.
func (u *uncertainError) mode() string {
	if u.review {
		return "review"
	}
	return "comments"
}

// inFlight is the review u sent, nil when u is nil or sent single comments.
func (u *uncertainError) inFlight() *model.InFlightReviewDTO {
	if u == nil || !u.review {
		return nil
	}
	sent := u.sent
	return &sent
}

// frozenModes maps each submodule with a send of unknown outcome to that send while its PR, still the one sent
// to, takes one of the drafts sent: the submodule's choice is held to its mode, verdict and text until a submit
// settles the send. Drafts that can no longer go there fold, and the submit settles the send before writing anything.
func (a *App) frozenModes() map[string]*uncertainError {
	sends := map[string]*uncertainError{}
	for _, d := range a.pendingComments {
		u := a.uncertain[d.ID]
		if u != nil && !u.parent && a.prSubmodules[u.path].prName() == u.where && a.canGoToSubmodulePR(d) {
			sends[u.path] = u
		}
	}
	return sends
}

// checkFrozen rejects a request that changes what a send of unknown outcome sent: a submodule's mode, or the
// verdict or text of a review, the loaded PR's included.
func (a *App) checkFrozen(req model.ReviewSubmitRequest) error {
	for path, u := range a.frozenModes() {
		c := choiceFor(req.Submodules, path)
		if c.Mode != u.mode() || (u.review && u.sent != model.InFlightReviewDTO{Verdict: c.Verdict, Body: c.Body}) {
			return errFrozen
		}
	}
	if p := a.parentAttempt; p != nil && p.sent != (model.InFlightReviewDTO{Verdict: req.Verdict, Body: req.Body}) {
		return errFrozen
	}
	return nil
}

// sendUnlessPosted runs send, which reports GitHub's answer; after a send that fails ambiguously, find looks
// for what it posted. A failed find leaves the outcome unknown: the error is then an *uncertainError.
func sendUnlessPosted(find finder, send func() (*github.Response, error)) error {
	start := time.Now()
	resp, err := send()
	if err == nil || !ambiguous(resp) {
		return err
	}
	_, found, ferr := find(start)
	switch {
	case ferr != nil:
		logger.L.Warn("could not check whether a failed request went through", "err", ferr)
		return &uncertainError{start: start, err: err, find: find}
	case found:
		return nil
	}
	return err
}

// settle records, for drafts just sent as review sent (or single comments, when sent has no verdict) to s's PR,
// or the loaded PR's when s is nil, whether err leaves it unknown if they went through.
func (a *App) settle(drafts []model.DraftCommentDTO, err error, sent model.InFlightReviewDTO, s *prSubmodule) {
	var u *uncertainError
	unknown := errors.As(err, &u)
	if unknown {
		u.review, u.sent, u.parent = sent.Verdict != "", sent, s == nil
		u.where = fmt.Sprintf("%s/%s#%d", a.prOwner, a.prRepo, a.prNumber)
		if s != nil {
			u.where, u.path = s.prName(), s.path
		}
		if a.uncertain == nil {
			a.uncertain = map[int64]*uncertainError{}
		}
	}
	if s == nil {
		a.parentAttempt = nil
		if unknown {
			a.parentAttempt = u
		}
	}
	for _, d := range drafts {
		if unknown {
			a.uncertain[d.ID] = u
		} else {
			delete(a.uncertain, d.ID)
		}
	}
}

// reconcileUncertain looks, for each pending draft whose last send may have gone through, and for the loaded
// PR's review, for what that send posted, reading only. Drafts found posted leave the pending ones; the others
// are free to go again. A lookup that fails again returns an error before anything is sent. parentURL is the
// loaded PR's review, when found, and folded the submodule drafts its text quoted.
func (a *App) reconcileUncertain() (parentURL string, folded int, err error) {
	for u := a.nextUncertain(); u != nil; u = a.nextUncertain() {
		url, found, err := u.find(u.start)
		if err != nil {
			return "", 0, uncheckedError(u, err)
		}
		sent := a.forget(u)
		if !found {
			continue
		}
		a.pendingComments = slices.DeleteFunc(a.pendingComments, func(p model.DraftCommentDTO) bool { return sent[p.ID] })
		switch {
		case u.parent:
			parentURL, folded = url, u.folded
		case u.review:
			a.subReviewURLs = append(a.subReviewURLs, url)
		default:
			a.subCommentURLs = append(a.subCommentURLs, url)
		}
	}
	return parentURL, folded, nil
}

// nextUncertain is a send of unknown outcome still to look up: one a pending draft went with, else the loaded
// PR's review; nil when there is none.
func (a *App) nextUncertain() *uncertainError {
	if i := slices.IndexFunc(a.pendingComments, func(d model.DraftCommentDTO) bool { return a.uncertain[d.ID] != nil }); i >= 0 {
		return a.uncertain[a.pendingComments[i].ID]
	}
	return a.parentAttempt
}

func uncheckedError(u *uncertainError, err error) error {
	return fmt.Errorf("github: could not check whether an earlier attempt on %s went through, so nothing was sent: %w", u.where, err)
}

// forget drops send u from the uncertain ones and returns the IDs of the drafts it carried.
func (a *App) forget(u *uncertainError) map[int64]bool {
	if a.parentAttempt == u {
		a.parentAttempt = nil
	}
	sent := map[int64]bool{}
	for id, v := range a.uncertain {
		if v == u {
			sent[id] = true
			delete(a.uncertain, id)
		}
	}
	return sent
}

// postedReviewComment returns, and claims, the viewer's unclaimed single comment on owner/repo#number matching
// sent and created since start, nil when there is none.
func (a *App) postedReviewComment(owner, repo string, number int, sent github.CreatePullRequestCommentRequest, start time.Time) (*github.PullRequestComment, error) {
	login, err := a.viewerLogin()
	if err != nil {
		return nil, err
	}
	earliest := start.Add(-clockSkew)
	opts := &github.PullRequestListCommentsOptions{Since: earliest, ListOptions: github.ListOptions{PerPage: 100}}
	for c, err := range a.ghClient.PullRequests.ListCommentsIter(a.ctx, owner, repo, number, opts) {
		if err != nil {
			return nil, err
		}
		if c.GetUser().GetLogin() == login && !a.claimed[claim{false, c.GetID()}] && sameReviewComment(c, sent, earliest) {
			logger.L.Info("adopted a comment a failed request posted", "pr", fmt.Sprintf("%s/%s#%d", owner, repo, number), "id", c.GetID())
			a.markClaimed(false, c.GetID())
			return c, nil
		}
	}
	return nil, nil
}

func sameReviewComment(c *github.PullRequestComment, sent github.CreatePullRequestCommentRequest, earliest time.Time) bool {
	if c.GetCreatedAt().Before(earliest) || c.GetPath() != sent.Path || c.GetBody() != sent.Body || c.GetCommitID() != sent.CommitID {
		return false
	}
	if sent.Line == nil {
		return c.GetSubjectType() == "file"
	}
	return c.GetLine() == *sent.Line && c.GetSide() == *sent.Side && c.GetStartLine() == sent.GetStartLine() &&
		c.GetStartSide() == sent.GetStartSide()
}

// postedIssueComment returns the viewer's conversation comment on owner/repo#number reading body and
// created since start, nil when there is none or the lookup fails.
func (a *App) postedIssueComment(owner, repo string, number int, body string, start time.Time) *github.IssueComment {
	login, err := a.viewerLogin()
	if err != nil {
		logger.L.Warn("could not check whether a failed comment was posted", "err", err)
		return nil
	}
	since := start.Add(-clockSkew)
	opts := &github.IssueListCommentsOptions{Since: &since, ListOptions: github.ListOptions{PerPage: 100}}
	for c, err := range a.ghClient.Issues.ListCommentsIter(a.ctx, owner, repo, number, opts) {
		if err != nil {
			logger.L.Warn("could not check whether a failed comment was posted", "err", err)
			return nil
		}
		if c.GetUser().GetLogin() == login && c.GetBody() == body && !c.GetCreatedAt().Before(since) {
			logger.L.Info("adopted a comment a failed request posted", "pr", fmt.Sprintf("%s/%s#%d", owner, repo, number), "id", c.GetID())
			return c
		}
	}
	return nil
}

// postedReview returns, and claims, the unclaimed review on owner/repo#number the viewer submitted since start
// like sent, with its verdict, commit, body and line comments, nil when there is none.
func (a *App) postedReview(owner, repo string, number int, sent *github.PullRequestReviewRequest, start time.Time) (*github.PullRequestReview, error) {
	login, err := a.viewerLogin()
	if err != nil {
		return nil, err
	}
	earliest := start.Add(-clockSkew)
	var commentsErr error
	r, err := a.findReview(owner, repo, number, func(r *github.PullRequestReview) bool {
		if !reviewLike(r, sent, login, earliest) || a.claimed[claim{true, r.GetID()}] {
			return false
		}
		same, err := a.sameReviewComments(owner, repo, number, r, sent, true)
		commentsErr = cmp.Or(commentsErr, err)
		return same
	})
	if r == nil {
		return nil, cmp.Or(err, commentsErr)
	}
	if err != nil {
		return nil, err
	}
	logger.L.Info("adopted a review a failed request posted", "pr", fmt.Sprintf("%s/%s#%d", owner, repo, number), "review_id", r.GetID())
	a.markClaimed(true, r.GetID())
	return r, nil
}

// reviewLike reports whether login submitted r like sent, at earliest or after, its line comments aside.
func reviewLike(r *github.PullRequestReview, sent *github.PullRequestReviewRequest, login string, earliest time.Time) bool {
	return r.GetState() == reviewStates[sent.GetEvent()] && r.GetUser().GetLogin() == login && r.GetCommitID() == sent.GetCommitID() &&
		r.GetBody() == sent.GetBody() && !r.GetSubmittedAt().Before(earliest)
}

// createReview creates req on owner/repo#number, as a pending review when req has no event. After a create that
// fails ambiguously, createdReview looks for what it did; when that lookup fails too the error is an
// *uncertainError.
func (a *App) createReview(owner, repo string, number int, req *github.PullRequestReviewRequest) (*github.PullRequestReview, error) {
	at := orphanReview{owner, repo, number, 0}
	if req.Event == nil {
		before, err := a.pendingBefore(owner, repo, number)
		if err != nil {
			return nil, err
		}
		at.id = before
	}
	var review *github.PullRequestReview
	err := sendUnlessPosted(func(start time.Time) (string, bool, error) {
		r, err := a.createdReview(at, req, start)
		review = r
		return r.GetHTMLURL(), r != nil, err
	}, func() (*github.Response, error) {
		r, resp, err := a.ghClient.PullRequests.CreateReview(a.ctx, owner, repo, number, req)
		if err == nil && r.GetID() == 0 {
			err = errNoReview
		}
		if err == nil {
			review = r
			a.markClaimed(true, r.GetID())
		}
		return resp, err
	})
	var u *uncertainError
	if errors.As(err, &u) {
		u.unknownPending = req.Event == nil
	}
	return review, err
}

// createdReview finds what a create of sent on at's PR that failed ambiguously did: the review it submitted, when
// sent has an event. A pending review it left joins the orphans, for the next submit or discard to delete; at.id
// is the one there before.
func (a *App) createdReview(at orphanReview, sent *github.PullRequestReviewRequest, start time.Time) (*github.PullRequestReview, error) {
	if sent.Event != nil {
		return a.postedReview(at.owner, at.repo, at.number, sent, start)
	}
	id, err := a.leftPending(at, sent)
	if o := (orphanReview{at.owner, at.repo, at.number, id}); id != 0 && !slices.Contains(a.orphans, o) {
		a.orphans = append(a.orphans, o)
	}
	return nil, err
}

// leftPending returns the ID of the viewer's pending review on at's PR like sent, else 0. It matches only one
// other than at's, the pending review there before the create: one started on github.com would lose its drafts.
func (a *App) leftPending(at orphanReview, sent *github.PullRequestReviewRequest) (int64, error) {
	r, err := a.findPendingReview(at.owner, at.repo, at.number)
	if err != nil || r == nil || r.GetID() == at.id || r.GetCommitID() != sent.GetCommitID() {
		return 0, err
	}
	if same, err := a.sameReviewComments(at.owner, at.repo, at.number, r, sent, false); !same {
		logger.L.Warn("SubmitReview: pending review left unclaimed", "review_id", r.GetID(), "err", err)
		return 0, err
	}
	logger.L.Info("SubmitReview: adopted pending review created by a failed request", "pr", fmt.Sprintf("%s/%s#%d", at.owner, at.repo, at.number), "review_id", r.GetID())
	return r.GetID(), nil
}

// findUnknownPending looks up the pending reviews creates of unknown outcome may have left, adding them to the
// orphans, so a discard deletes them too.
func (a *App) findUnknownPending() error {
	for _, u := range a.attempts() {
		if !u.unknownPending {
			continue
		}
		if a.ghClient == nil {
			return fmt.Errorf("auth: not authenticated")
		}
		if _, _, err := u.find(u.start); err != nil {
			return fmt.Errorf("github: could not check for a pending review an earlier attempt may have left on %s, so nothing was discarded: %w", u.where, err)
		}
		a.forget(u)
	}
	return nil
}

// attempts lists each send of unknown outcome once, the loaded PR's review included.
func (a *App) attempts() []*uncertainError {
	var all []*uncertainError
	for _, u := range a.uncertain {
		if !slices.Contains(all, u) {
			all = append(all, u)
		}
	}
	if a.parentAttempt != nil && !slices.Contains(all, a.parentAttempt) {
		all = append(all, a.parentAttempt)
	}
	return all
}

// submitPending submits pending review p with req. When GitHub's answer and the review's state after it both
// fail, the error is an *uncertainError.
func (a *App) submitPending(p orphanReview, req github.PullRequestSubmitReviewRequest) (*github.PullRequestReview, error) {
	start := time.Now()
	review, resp, err := a.ghClient.PullRequests.SubmitReview(a.ctx, p.owner, p.repo, p.number, p.id, req)
	if err == nil && review.GetID() == 0 {
		err = errNoReview
	}
	if err == nil || !ambiguous(resp) {
		return review, err
	}
	r, _, gerr := a.ghClient.PullRequests.GetReview(a.ctx, p.owner, p.repo, p.number, p.id)
	switch {
	case gerr == nil && r.GetState() == "PENDING":
		return nil, err
	case gerr == nil && submitted(r, p.id):
		a.markClaimed(true, p.id)
		return r, nil
	}
	logger.L.Warn("could not check whether a failed review was submitted", "err", gerr, "state", r.GetState())
	return nil, &uncertainError{start: start, err: err, find: a.pendingOutcome(p), pending: &p}
}

// submitted reports whether r is review id, read back submitted.
func submitted(r *github.PullRequestReview, id int64) bool {
	return r.GetID() == id && slices.Contains([]string{"APPROVED", "CHANGES_REQUESTED", "COMMENTED", "DISMISSED"}, r.GetState())
}

// pendingOutcome finds what an uncertain submit of pending review p published: the review once submitted,
// nothing once it is gone or still pending. One still pending joins the orphans, so the submit deletes it and
// sends its drafts again with what it now chooses.
func (a *App) pendingOutcome(p orphanReview) finder {
	return func(time.Time) (string, bool, error) {
		r, gone, err := a.readReview(p)
		switch {
		case gone:
			return "", false, nil
		case err != nil:
			return "", false, err
		case r.GetState() == "PENDING":
			a.orphans = append(a.orphans, p)
			return "", false, nil
		case !submitted(r, p.id):
			return "", false, fmt.Errorf("GitHub read review %d back as neither pending nor submitted", p.id)
		}
		a.markClaimed(true, p.id)
		return r.GetHTMLURL(), true, nil
	}
}

// readReview reads review p; gone is set when GitHub no longer has it.
func (a *App) readReview(p orphanReview) (r *github.PullRequestReview, gone bool, err error) {
	r, resp, err := a.ghClient.PullRequests.GetReview(a.ctx, p.owner, p.repo, p.number, p.id)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		return nil, true, nil
	}
	return r, false, err
}

// commentKey is what a line comment is matched on.
type commentKey struct {
	path      string
	line      int
	side      string
	startLine int
	startSide string
	body      string
}

// sameReviewComments reports whether review r holds exactly sent's line comments, matched on their anchors
// too when anchored, else on their paths and bodies.
// Pass anchored false for a pending review: GitHub's REST API omits line and side on its comments.
func (a *App) sameReviewComments(owner, repo string, number int, r *github.PullRequestReview, sent *github.PullRequestReviewRequest, anchored bool) (bool, error) {
	count := map[commentKey]int{}
	for c, err := range a.ghClient.PullRequests.ListReviewCommentsIter(a.ctx, owner, repo, number, r.GetID(), &github.ListOptions{PerPage: 100}) {
		if err != nil {
			return false, fmt.Errorf("read the comments of review %d: %w", r.GetID(), err)
		}
		count[keyOf(commentKey{c.GetPath(), c.GetLine(), c.GetSide(), c.GetStartLine(), c.GetStartSide(), c.GetBody()}, anchored)]++
	}
	for _, c := range sent.Comments {
		count[keyOf(commentKey{c.GetPath(), c.GetLine(), c.GetSide(), c.GetStartLine(), c.GetStartSide(), c.GetBody()}, anchored)]--
	}
	for _, n := range count {
		if n != 0 {
			return false, nil
		}
	}
	return true, nil
}

func keyOf(k commentKey, anchored bool) commentKey {
	if anchored {
		return k
	}
	return commentKey{path: k.path, body: k.body}
}
