package app

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/google/go-github/v93/github"

	"gitura/internal/db"
	"gitura/internal/ghapi"
	"gitura/internal/logger"
	"gitura/internal/model"
)

// prSubmodule is one submodule pointer the loaded PR moves.
type prSubmodule struct {
	ghapi.SubmoduleChanges
	path, from, to, rng string
	// in is the submodule whose bump moves this one inside it, nil for one the loaded PR bumps itself.
	in *prSubmodule
	// failed is set when loading the files failed, so GetPRFiles retries.
	failed bool
	// pr is the submodule's open PR whose head is to, nil when there is none; prFiles are its files by relative
	// path, a pointer and the file replacing it sharing one.
	pr      *github.PullRequest
	prFiles map[string][]*github.CommitFile
	// prUnknown is set while looking up pr has failed, as opposed to finding none.
	prUnknown bool
	// checkedAt is when pr was last found to be open at to, or found missing.
	checkedAt time.Time
	// viewedUnknown is set while reading the viewed marks on pr has failed.
	viewedUnknown bool
	// retryAt is when a failed load or lookup may be tried again.
	retryAt time.Time
}

// submoduleRetryDelay spaces retries of a failed submodule load, which run under prMu.
const submoduleRetryDelay = 30 * time.Second

// submodulePRMaxAge is how long a viewed mark or routing trusts that a submodule's PR is still open at the
// bump, or still missing.
const submodulePRMaxAge = time.Minute

func (s *prSubmodule) backOff() {
	s.retryAt = time.Now().Add(submoduleRetryDelay)
}

func (s *prSubmodule) due() bool {
	return !time.Now().Before(s.retryAt)
}

// staleNoPR reports whether s's PR was found missing over submodulePRMaxAge ago: one may have opened since.
func (s *prSubmodule) staleNoPR() bool {
	return s.pr == nil && !s.prUnknown && !s.failed && s.to != "" && len(s.Files) > 0 && time.Since(s.checkedAt) > submodulePRMaxAge
}

// bump is s's "old..new" commits, "" when s is nil.
func (s *prSubmodule) bump() string {
	if s == nil {
		return ""
	}
	return s.from + ".." + s.to
}

// rel is path relative to s's repository; with s nil, the loaded PR's.
func (s *prSubmodule) rel(path string) string {
	if s == nil {
		return path
	}
	return strings.TrimPrefix(path, s.path+"/")
}

// prFile returns a submodule PR's file at path, nil when path isn't in that PR's diff.
func (s *prSubmodule) prFile(path string) *github.CommitFile {
	if fs := s.prFiles[s.rel(path)]; s.pr != nil && len(fs) > 0 {
		return fs[0]
	}
	return nil
}

// prFileLike returns the submodule PR's file at bf's path that, like bf, a file of s's bump, reads as a pointer
// change or not, nil when the PR lists none.
func (s *prSubmodule) prFileLike(bf *github.CommitFile) *github.CommitFile {
	if s.pr == nil || bf == nil {
		return nil
	}
	_, _, pointer := ghapi.SubmoduleBump(bf)
	for _, f := range s.prFiles[s.rel(bf.GetFilename())] {
		if _, _, bump := ghapi.SubmoduleBump(f); bump == pointer {
			return f
		}
	}
	return nil
}

// bumpFile returns the file at path in s's bump, its pointer row when pointer is set: a submodule replaced by a
// file at its path lists both. nil when s is nil or its bump changes no such file. Callers hold prMu.
func (a *App) bumpFile(s *prSubmodule, path string, pointer bool) *github.CommitFile {
	if s == nil {
		return nil
	}
	for _, f := range s.Files {
		if f.GetFilename() == path && isPointerRow(a.prSubmodules[path], s, f) == pointer {
			return f
		}
	}
	return nil
}

// isPointerRow reports whether f, listed in submodule in (nil for the loaded PR), is the pointer row of bump s.
func isPointerRow(s, in *prSubmodule, f *github.CommitFile) bool {
	if s == nil || s.in != in {
		return false
	}
	_, _, ok := ghapi.SubmoduleBump(f)
	return ok
}

// pointerOf returns the bump whose pointer row f is, nil for any other file. Callers hold prMu.
func (a *App) pointerOf(f *github.CommitFile) *prSubmodule {
	if s := a.prSubmodules[f.GetFilename()]; isPointerRow(s, a.prSubmoduleOf[f], f) {
		return s
	}
	return nil
}

// idOf is the fileID of f, listed in submodule in (nil for the loaded PR). Callers hold prMu.
func (a *App) idOf(in *prSubmodule, f *github.CommitFile) fileID {
	id := fileID{path: f.GetFilename(), pointer: isPointerRow(a.prSubmodules[f.GetFilename()], in, f)}
	if in != nil {
		id.repo = in.path
	}
	return id
}

// loadPRFiles fills prFilesCache, adding the files changed inside each bumped submodule after its pointer.
// Callers hold prMu.
func (a *App) loadPRFiles() error {
	if a.prFilesCache != nil {
		return nil
	}
	raw, err := ghapi.FetchPRFilesRaw(a.ctx, a.ghClient, a.prOwner, a.prRepo, a.prNumber)
	if err != nil {
		return err
	}
	subs := map[string]*prSubmodule{}
	for _, s := range a.loadBumps(nil, raw) {
		subs[s.path] = s
	}
	files, subOf := assemblePRFiles(raw, subs)
	a.prFilesCache, a.prSubmodules, a.prSubmoduleOf = files, subs, subOf
	a.prFilesTruncated = len(raw) >= prFileListCap
	a.loadSubViewed()
	for _, s := range subs {
		a.migrateSubViewed(s)
	}
	return nil
}

// prFileListCap is the most files GitHub's PR files list returns; it drops the rest without saying so.
const prFileListCap = 3000

// maxSubmoduleDepth is how many levels of submodules inside submodules PR mode expands.
const maxSubmoduleDepth = 3

// loadBumps loads each submodule bump among files, those of in (nil for the loaded PR's own), then the bumps
// inside each in turn.
// shortcut: a bump nested deeper than maxSubmoduleDepth shows as its pointer change only; raise the limit if needed.
func (a *App) loadBumps(in *prSubmodule, files []*github.CommitFile) []*prSubmodule {
	if in.depth() >= maxSubmoduleDepth {
		return nil
	}
	var out []*prSubmodule
	for _, f := range files {
		if from, to, ok := ghapi.SubmoduleBump(f); ok {
			if s := a.loadSubmodule(in, f.GetFilename(), from, to); s != nil {
				out = append(append(out, s), a.loadBumps(s, s.Files)...)
			}
		}
	}
	return out
}

// depth is how many submodules deep s sits, 1 for one the loaded PR bumps itself and 0 for nil.
func (s *prSubmodule) depth() int {
	n := 0
	for ; s != nil; s = s.in {
		n++
	}
	return n
}

// assemblePRFiles lists the parent PR's files with each submodule's files after its pointer row, nested ones
// likewise, and maps each submodule file to its bump. A submodule file may share its path with a parent file,
// as when a PR turns a tracked directory into a submodule: both are listed.
func assemblePRFiles(parent []*github.CommitFile, subs map[string]*prSubmodule) ([]*github.CommitFile, map[*github.CommitFile]*prSubmodule) {
	files := make([]*github.CommitFile, 0, len(parent))
	subOf := map[*github.CommitFile]*prSubmodule{}
	var add func(in *prSubmodule, fs []*github.CommitFile)
	add = func(in *prSubmodule, fs []*github.CommitFile) {
		for _, f := range fs {
			files = append(files, f)
			if in != nil {
				subOf[f] = in
			}
			if s := subs[f.GetFilename()]; isPointerRow(s, in, f) {
				add(s, s.Files)
			}
		}
	}
	add(nil, parent)
	return files, subOf
}

// retryFailedSubmodules loads again the submodules whose files failed to load, looks up again the PRs
// of those whose lookup failed, and reads again the viewed marks that failed to read, each at most once
// per submoduleRetryDelay.
func (a *App) retryFailedSubmodules() {
	var retried []*prSubmodule
	for _, s := range a.prSubmodules {
		switch {
		case !s.due():
		case s.failed:
			retried = append(retried, a.reloadSubmodule(s)...)
		case s.prUnknown:
			a.retrySubmodulePR(s)
		case s.viewedUnknown:
			a.addSubmoduleViewed(s)
		}
	}
	if len(retried) == 0 {
		return
	}
	parent := slices.DeleteFunc(slices.Clone(a.prFilesCache), func(f *github.CommitFile) bool { return a.prSubmoduleOf[f] != nil })
	a.prFilesCache, a.prSubmoduleOf = assemblePRFiles(parent, a.prSubmodules)
	for _, s := range retried {
		a.addSubmoduleViewed(s)
	}
	a.loadSubViewed()
	for _, s := range retried {
		a.migrateSubViewed(s)
	}
}

// reloadSubmodule loads failed bump s again, with the bumps inside it, and returns them all.
func (a *App) reloadSubmodule(s *prSubmodule) []*prSubmodule {
	path := s.path
	if s = a.loadSubmodule(s.in, s.path, s.from, s.to); s == nil {
		delete(a.prSubmodules, path)
		return nil
	}
	loaded := append([]*prSubmodule{s}, a.loadBumps(s, s.Files)...)
	for _, n := range loaded {
		a.prSubmodules[n.path] = n
	}
	return loaded
}

// retrySubmodulePR looks up s's PR again after a failed lookup, then loads its viewed marks and moves there
// those kept in the app.
func (a *App) retrySubmodulePR(s *prSubmodule) {
	a.findSubmodulePR(s)
	a.addSubmoduleViewed(s)
	a.migrateSubViewed(s)
}

// loadSubmodule fetches what a bump changes, inside submodule in or the loaded PR when nil, and the
// submodule's open PR at the new commit. Failures leave the pointer row in place with the files or PR missing.
// nil when path holds no submodule, only a file whose patch reads like a pointer change.
func (a *App) loadSubmodule(in *prSubmodule, path, from, to string) *prSubmodule {
	s := &prSubmodule{in: in, path: path, from: from, to: to, rng: shortSHA(from) + ".." + shortSHA(to)}
	owner, repo, ref, err := a.holderRef(s)
	if err == nil {
		s.SubmoduleChanges, err = ghapi.FetchSubmoduleChanges(a.ctx, a.ghClient, owner, repo, ref, in.rel(path), from, to)
	}
	if errors.Is(err, ghapi.ErrNotSubmodule) {
		return nil
	}
	if err == nil && in != nil {
		s.MoveUnder(in.path)
	}
	if err != nil {
		logger.L.Warn("submodule files unavailable", "path", path, "err", err)
		s.failed = true
		s.backOff()
		s.Unavailable = cmp.Or(s.Unavailable, "failed to load")
		return s
	}
	if len(s.Files) > 0 && to != "" {
		a.findSubmodulePR(s)
	}
	return s
}

// holderRef is the repository holding s and a commit of it where s exists: for a nested submodule the one
// holding it is bumped to, or bumped from when s is removed there.
func (a *App) holderRef(s *prSubmodule) (owner, repo, ref string, err error) {
	if s.in == nil {
		ref, err = a.submoduleRef(s.to)
		return a.prOwner, a.prRepo, ref, err
	}
	ref = s.in.to
	if s.to == "" {
		ref = s.in.from
	}
	return s.in.Owner, s.in.Repo, ref, nil
}

// submoduleRef is a parent commit holding a submodule bumped to to: the PR head, or for a removed
// submodule (to "") the merge base the PR's diff starts from.
func (a *App) submoduleRef(to string) (string, error) {
	if to != "" {
		return a.prCache.HeadSHA, nil
	}
	c, _, err := a.ghClient.Repositories.CompareCommits(a.ctx, a.prOwner, a.prRepo, a.prCache.BaseBranch, a.prCache.HeadSHA, &github.ListOptions{PerPage: 1})
	if err != nil {
		return "", fmt.Errorf("github: get PR merge base: %w", err)
	}
	return c.GetMergeBaseCommit().GetSHA(), nil
}

// findSubmodulePR looks up s's open PR at its new commit and that PR's files, setting prUnknown when either fails.
func (a *App) findSubmodulePR(s *prSubmodule) {
	pr, err := ghapi.FindOpenPRForCommit(a.ctx, a.ghClient, s.Owner, s.Repo, s.to)
	if err != nil {
		logger.L.Warn("submodule PR lookup failed", "path", s.path, "err", err)
		s.prUnknown = true
		s.backOff()
		return
	}
	s.prUnknown, s.checkedAt = false, time.Now()
	s.pr, s.prFiles = nil, nil
	if pr == nil {
		return
	}
	prFiles, err := ghapi.FetchPRFilesRaw(a.ctx, a.ghClient, s.Owner, s.Repo, pr.GetNumber())
	if err != nil {
		// Without its files the PR can take neither comments nor viewed marks.
		logger.L.Warn("submodule PR files unavailable", "path", s.path, "err", err)
		s.prUnknown = true
		s.backOff()
		return
	}
	s.setPR(pr, prFiles)
}

func (s *prSubmodule) setPR(pr *github.PullRequest, files []*github.CommitFile) {
	s.pr, s.prFiles = pr, make(map[string][]*github.CommitFile, len(files))
	for _, f := range files {
		s.prFiles[f.GetFilename()] = append(s.prFiles[f.GetFilename()], f)
	}
}

// revalidateSubmodulePR reads s's PR again and drops it from s when it is closed or its head is no longer
// the bumped commit, so its drafts fold and its viewed marks stay in the app; it reports whether it dropped it.
// A PR retargeted to another base keeps its head but not its diff, so its files are read again, as they are
// when refetch is set, and its viewed marks with them.
func (a *App) revalidateSubmodulePR(s *prSubmodule, refetch bool) (bool, error) {
	pr, _, err := a.ghClient.PullRequests.Get(a.ctx, s.Owner, s.Repo, s.pr.GetNumber())
	if err != nil {
		return false, fmt.Errorf("github: check %s is still open: %w", s.prName(), err)
	}
	if pr.GetState() != "open" || pr.GetHead().GetSHA() != s.to {
		logger.L.Info("submodule PR no longer takes comments", "pr", s.prName(), "state", pr.GetState())
		s.pr, s.prFiles, s.checkedAt, s.viewedUnknown = nil, nil, time.Now(), false
		return true, nil
	}
	if refetch || pr.GetBase().GetSHA() != s.pr.GetBase().GetSHA() {
		files, err := ghapi.FetchPRFilesRaw(a.ctx, a.ghClient, s.Owner, s.Repo, pr.GetNumber())
		if err != nil {
			return false, fmt.Errorf("github: read the files of %s again: %w", s.prName(), err)
		}
		logger.L.Info("submodule PR files read again", "pr", s.prName(), "base", pr.GetBase().GetSHA())
		s.setPR(pr, files)
		a.addSubmoduleViewed(s)
		a.migrateSubViewed(s)
	}
	s.checkedAt = time.Now()
	return false, nil
}

// prName is s's open PR as "owner/repo#12", "" when there is none or s is nil.
func (s *prSubmodule) prName() string {
	if s == nil || s.pr == nil {
		return ""
	}
	return fmt.Sprintf("%s/%s#%d", s.Owner, s.Repo, s.pr.GetNumber())
}

func shortSHA(sha string) string {
	if sha == "" {
		return "0000000"
	}
	return sha[:min(7, len(sha))]
}

// prFileDTO is f as the Files list shows it, submodule fields and viewed mark included. A GitHub viewed mark is
// per path, so a pointer row and a regular file at its path share it.
func (a *App) prFileDTO(f *github.CommitFile) model.PRFileDTO {
	dto := ghapi.PRFileFromCommitFile(f)
	in := a.prSubmoduleOf[f]
	if in == nil {
		dto.Viewed = a.viewedFiles[fileID{path: dto.Filename}]
	} else {
		dto.Repo, dto.Submodule = in.path, in.path
		dto.Viewed = a.viewedFiles[fileID{repo: in.path, path: dto.Filename}]
		dto.LinesUnknown = in.TreeRef != "" && f.Patch == nil && f.GetChanges() == 0
		if in.prFile(dto.Filename) == nil {
			dto.Viewed, dto.ViewedLocal = a.subViewedNow(in, f), true
		}
	}
	if s := a.pointerOf(f); s != nil {
		dto.Status, dto.Submodule, dto.SubmoduleRange = "submodule", s.path, s.rng
		dto.SubmoduleBackwards, dto.SubmoduleDiverged, dto.SubmoduleUnavailable = s.Backwards, s.Diverged, s.Unavailable
		dto.SubmodulePR, dto.SubmodulePRURL, dto.SubmodulePRUnknown = s.prName(), s.pr.GetHTMLURL(), s.prUnknown
		dto.SubmoduleViewedUnknown = s.viewedUnknown
	}
	return dto
}

// subViewedNow reports whether f, a file of s, carries a viewed mark kept in the app, made on its current change.
func (a *App) subViewedNow(s *prSubmodule, f *github.CommitFile) bool {
	fp, ok := a.subViewed[a.idOf(s, f)]
	return ok && fp == s.fingerprint(f)
}

// fingerprint identifies the change to f, a file of s: its patch, or without one (a binary file) its old
// and new blobs.
// shortcut: a mode-only change keeps the mark, as the compare API gives no modes; fetch both trees if that matters.
func (s *prSubmodule) fingerprint(f *github.CommitFile) string {
	parts := []string{f.GetSHA(), f.GetStatus(), f.GetPreviousFilename()}
	switch {
	case s.TreeRef != "":
		// A whole-tree file is added or removed, so its blob is all there is; its patch loads later.
	case f.GetPatch() != "":
		parts = append(parts, f.GetPatch())
	default:
		parts = append(parts, s.OldSHAs[f.GetFilename()])
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

// loadSubViewed reads the loaded PR's viewed marks kept in the app, deleting those whose file changed since
// or is no longer listed. Failures leave the marks unread: they only decorate the file list.
func (a *App) loadSubViewed() {
	a.subViewed = map[fileID]string{}
	if a.queries == nil {
		return
	}
	rows, err := a.queries.ListSubmoduleViewed(a.ctx, db.ListSubmoduleViewedParams{Owner: a.prOwner, Repo: a.prRepo, Number: int64(a.prNumber)})
	if err != nil {
		logger.L.Warn("submodule viewed marks unavailable", "err", err)
		return
	}
	for _, r := range rows {
		id := fileID{r.Submodule, r.Path, r.Pointer}
		if a.subViewedCurrent(id, r.Fingerprint) {
			a.subViewed[id] = r.Fingerprint
			continue
		}
		if err := a.deleteSubViewed(id); err != nil {
			logger.L.Warn("could not clear stale viewed mark", "path", r.Path, "err", err)
		}
	}
}

// subViewedCurrent reports whether a mark on id made on change fp still applies: the file is listed with
// that change, or the listing of its submodule or of one holding it is incomplete, so can't be checked.
func (a *App) subViewedCurrent(id fileID, fp string) bool {
	if !a.listingComplete(id.repo) {
		return true
	}
	s := a.prSubmodules[id.repo]
	f := a.bumpFile(s, id.path, id.pointer)
	return f != nil && s.fingerprint(f) == fp
}

// listingComplete reports whether the submodule at path, missing or not, and every submodule holding it list
// all they change: one holding it that failed to load leaves it unlisted, as does a truncated parent file list
// that lists none of them.
func (a *App) listingComplete(path string) bool {
	listed := false
	for p := path; p != ""; p = p[:max(strings.LastIndex(p, "/"), 0)] {
		s := a.prSubmodules[p]
		if s != nil && !s.complete() {
			return false
		}
		listed = listed || s != nil
	}
	return listed || !a.prFilesTruncated
}

// complete reports whether s lists every file its bump changes, each with what its fingerprint needs.
func (s *prSubmodule) complete() bool {
	return !s.failed && !s.prUnknown && s.Unavailable == "" && !s.BlobsTruncated
}

// migrateSubViewed moves the viewed marks kept in the app on s's files that s's PR contains to that PR, if
// made on the file's current change. One made on another change is deleted, or kept while s's listing is
// incomplete. A mark that fails to move stays in the app.
func (a *App) migrateSubViewed(s *prSubmodule) {
	if s.pr == nil {
		return
	}
	for _, f := range s.Files {
		path, id := f.GetFilename(), a.idOf(s, f)
		fp, ok := a.subViewed[id]
		if !ok || s.prFile(path) == nil {
			continue
		}
		if fp != s.fingerprint(f) {
			a.dropStaleSubViewed(s, id)
			continue
		}
		if err := ghapi.SetFileViewed(a.ctx, ghapi.NewHTTPClient(a.authToken), s.pr.GetNodeID(), s.rel(path), true); err != nil {
			logger.L.Warn("could not move a viewed mark to the submodule PR", "path", path, "err", err)
			continue
		}
		if a.viewedFiles != nil {
			a.viewedFiles[fileID{repo: s.path, path: path}] = true
		}
		if err := a.deleteSubViewed(id); err != nil {
			logger.L.Warn("could not delete a viewed mark moved to the submodule PR", "path", path, "err", err)
			continue
		}
		delete(a.subViewed, id)
	}
}

// dropStaleSubViewed deletes the mark kept in the app on id, a file of s, made on another change, unless s's
// listing is incomplete.
func (a *App) dropStaleSubViewed(s *prSubmodule, id fileID) {
	if !s.complete() {
		return
	}
	if err := a.deleteSubViewed(id); err != nil {
		logger.L.Warn("could not clear stale viewed mark", "path", id.path, "err", err)
		return
	}
	delete(a.subViewed, id)
}

func (a *App) deleteSubViewed(id fileID) error {
	return a.queries.DeleteSubmoduleViewed(a.ctx, db.DeleteSubmoduleViewedParams{Owner: a.prOwner, Repo: a.prRepo, Number: int64(a.prNumber), Submodule: id.repo, Path: id.path, Pointer: id.pointer})
}

// addSubmoduleViewed reads the viewed marks on s's PR into those already fetched, if any.
func (a *App) addSubmoduleViewed(s *prSubmodule) {
	if a.viewedFiles != nil {
		a.fetchViewedOf(s)
	}
}

// fetchViewedOf replaces s's files' marks in viewedFiles with those on s's PR. A failed read sets
// viewedUnknown, for retryFailedSubmodules to retry.
func (a *App) fetchViewedOf(s *prSubmodule) {
	s.viewedUnknown = false
	if s.pr == nil {
		return
	}
	marks, err := ghapi.FetchViewedFiles(a.ctx, ghapi.NewHTTPClient(a.authToken), s.Owner, s.Repo, s.pr.GetNumber())
	if err != nil {
		logger.L.Warn("submodule viewed marks unavailable", "path", s.path, "err", err)
		s.viewedUnknown = true
		s.backOff()
		return
	}
	// GitHub omits the files whose mark it dismissed, as on a retarget.
	for _, f := range s.Files {
		a.viewedFiles[fileID{repo: s.path, path: f.GetFilename()}] = marks[s.rel(f.GetFilename())]
	}
}

// setSubmoduleFileViewed stores a viewed mark on f, a file of s, in s's PR, or in the app's database when
// the PR lacks it. Past submodulePRMaxAge the PR, or its absence, is checked again; rerouted reports that
// this moved the mark's destination.
func (a *App) setSubmoduleFileViewed(s *prSubmodule, f *github.CommitFile, viewed bool) (rerouted bool, err error) {
	path := f.GetFilename()
	if rerouted, err = a.recheckViewedRoute(s, path); err != nil {
		return false, err
	}
	if s.prFile(path) == nil {
		return rerouted, a.setAppViewed(s, f, viewed)
	}
	if err := ghapi.SetFileViewed(a.ctx, ghapi.NewHTTPClient(a.authToken), s.pr.GetNodeID(), s.rel(path), viewed); err != nil {
		return false, err
	}
	if a.viewedFiles != nil {
		a.viewedFiles[fileID{repo: s.path, path: path}] = viewed
	}
	if viewed || a.queries == nil {
		return rerouted, nil
	}
	// A mark that failed to move to the PR would show again once the PR's is gone.
	id := a.idOf(s, f)
	if err := a.deleteSubViewed(id); err != nil {
		return false, fmt.Errorf("db: %w", err)
	}
	delete(a.subViewed, id)
	return rerouted, nil
}

// recheckViewedRoute checks s's PR, or its absence, again past submodulePRMaxAge, and reports whether that
// changed which of s's files the PR takes viewed marks on, or their marks on GitHub.
func (a *App) recheckViewedRoute(s *prSubmodule, path string) (bool, error) {
	return a.rerouted(s, func() error {
		switch {
		case s.pr != nil && time.Since(s.checkedAt) > submodulePRMaxAge:
			// A file missing from the cached list may have joined the PR since, so the list is read again.
			_, err := a.revalidateSubmodulePR(s, s.prFile(path) == nil)
			return err
		case s.staleNoPR():
			a.retrySubmodulePR(s)
		}
		return nil
	})
}

// rerouted runs recheck and reports whether it changed which of s's files its PR takes viewed marks on,
// or their marks on GitHub.
func (a *App) rerouted(s *prSubmodule, recheck func() error) (bool, error) {
	before, marks := slices.Sorted(maps.Keys(s.prFiles)), a.githubMarks(s)
	if err := recheck(); err != nil {
		return false, err
	}
	return !slices.Equal(before, slices.Sorted(maps.Keys(s.prFiles))) || !slices.Equal(marks, a.githubMarks(s)), nil
}

// githubMarks are the cached GitHub viewed marks on s's files, in s.Files order.
func (a *App) githubMarks(s *prSubmodule) []bool {
	out := make([]bool, len(s.Files))
	for i, f := range s.Files {
		out[i] = a.viewedFiles[fileID{repo: s.path, path: f.GetFilename()}]
	}
	return out
}

// setAppViewed stores a viewed mark on f, a file of s, in the app's database.
func (a *App) setAppViewed(s *prSubmodule, f *github.CommitFile, viewed bool) error {
	if a.queries == nil {
		return fmt.Errorf("db: state database unavailable; viewed marks on %s can't be kept", s.path)
	}
	id := a.idOf(s, f)
	if !viewed {
		if err := a.deleteSubViewed(id); err != nil {
			return fmt.Errorf("db: %w", err)
		}
		delete(a.subViewed, id)
		return nil
	}
	fp := s.fingerprint(f)
	err := a.queries.UpsertSubmoduleViewed(a.ctx, db.UpsertSubmoduleViewedParams{Owner: a.prOwner, Repo: a.prRepo, Number: int64(a.prNumber), Submodule: id.repo, Path: id.path, Pointer: id.pointer, Fingerprint: fp})
	if err != nil {
		return fmt.Errorf("db: %w", err)
	}
	a.subViewed[id] = fp
	return nil
}

// toSubmodulePR reports whether d can go to s's PR as a review comment: it is on a file in that
// PR's diff, on lines inside its hunks. Other drafts on submodule files are folded into the parent's.
// Callers hold prMu.
func (a *App) toSubmodulePR(s *prSubmodule, d model.DraftCommentDTO) bool {
	bf := a.bumpFile(s, d.Path, d.Pointer)
	pf := s.prFileLike(bf)
	if pf == nil {
		return false
	}
	if d.Line == 0 {
		return true
	}
	// Both diffs end at the PR head, so right-side lines agree; left-side ones only if the base does too.
	sameBase := bf.GetPatch() == pf.GetPatch()
	if !sameBase && (d.Side == "LEFT" || (d.StartLine > 0 && d.StartSide == "LEFT")) {
		return false
	}
	return draftInHunks(d, ghapi.ParseCommitFileDiff(pf).Hunks)
}

// draftInHunks reports whether d's line, and start line if any, sit in one hunk of hunks.
func draftInHunks(d model.DraftCommentDTO, hunks []model.DiffHunkDTO) bool {
	start, startSide := d.Line, d.Side
	if d.StartLine > 0 {
		start, startSide = d.StartLine, d.StartSide
	}
	return slices.ContainsFunc(hunks, func(h model.DiffHunkDTO) bool {
		return hunkHas(h, startSide, start) && hunkHas(h, d.Side, d.Line)
	})
}

func hunkHas(h model.DiffHunkDTO, side string, line int) bool {
	if side == "LEFT" {
		return line >= h.OldStart && line < h.OldStart+h.OldLines
	}
	return line >= h.NewStart && line < h.NewStart+h.NewLines
}

// foldComments renders submodule drafts as a review-body section, one bullet per draft, each naming its
// submodule: two repositories can hold a file at one path.
func foldComments(drafts []model.DraftCommentDTO) string {
	var b strings.Builder
	b.WriteString("**Comments on submodule files**\n")
	for _, d := range drafts {
		where := "file"
		switch {
		case d.Line > 0 && d.StartLine > 0 && d.StartLine != d.Line:
			where = fmt.Sprintf("L%d–%d", d.StartLine, d.Line)
		case d.Line > 0:
			where = fmt.Sprintf("L%d", d.Line)
		}
		if d.Line > 0 && d.Side == "LEFT" {
			where += " (old)"
		}
		repo := "submodule " + d.Submodule
		if d.Pointer {
			repo = "pointer, in submodule " + d.Submodule
		}
		// Indenting continuation lines keeps a multi-line body inside its bullet.
		fmt.Fprintf(&b, "\n- `%s` (%s) %s — %s", d.Path, repo, where, strings.ReplaceAll(d.Body, "\n", "\n  "))
	}
	return b.String()
}

// withFolded appends the section for folded to body.
func withFolded(body string, folded []model.DraftCommentDTO) string {
	if len(folded) == 0 {
		return body
	}
	if body == "" {
		return foldComments(folded)
	}
	return body + "\n\n" + foldComments(folded)
}

var reviewVerdicts = []string{"COMMENT", "APPROVE", "REQUEST_CHANGES"}

// checkSubmoduleChoices rejects a choice for a path the loaded PR doesn't bump, with an unknown mode or
// verdict, or with a verdict GitHub refuses on the viewer's own PR.
func (a *App) checkSubmoduleChoices(choices []model.SubmoduleReviewChoice) error {
	for _, c := range choices {
		if err := a.checkSubmoduleChoice(c); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) checkSubmoduleChoice(c model.SubmoduleReviewChoice) error {
	switch {
	case a.prSubmodules[c.Path] == nil && !a.hasSubmoduleDrafts(c.Path):
		return fmt.Errorf("validation: %q is not a submodule this pull request changes", c.Path)
	case c.Mode == "fold" || c.Mode == "comments":
	case c.Mode != "review":
		return fmt.Errorf("validation: unknown mode %q for submodule %s", c.Mode, c.Path)
	case a.prSubmodules[c.Path].prName() == "":
		return fmt.Errorf("validation: submodule %s has no open pull request to review", c.Path)
	case !slices.Contains(reviewVerdicts, c.Verdict):
		return fmt.Errorf("validation: unknown verdict %q for submodule %s", c.Verdict, c.Path)
	case c.Verdict != "COMMENT":
		return a.checkNotOwnPR(a.prSubmodules[c.Path])
	}
	return nil
}

// hasSubmoduleDrafts reports whether drafts are pending on files of the submodule at path, which a bump since
// dropped from the PR may leave.
func (a *App) hasSubmoduleDrafts(path string) bool {
	return slices.ContainsFunc(a.pendingComments, func(d model.DraftCommentDTO) bool { return d.Submodule == path })
}

// checkNotOwnPR rejects an Approve or Request changes verdict on s's PR when the viewer wrote it.
func (a *App) checkNotOwnPR(s *prSubmodule) error {
	own, err := a.ownSubmodulePR(s)
	if err != nil {
		return err
	}
	if own {
		return fmt.Errorf("validation: %s is your own pull request; GitHub lets you only comment on it", s.prName())
	}
	return nil
}

// ownSubmodulePR reports whether the viewer wrote s's open PR.
func (a *App) ownSubmodulePR(s *prSubmodule) (bool, error) {
	if s.pr == nil {
		return false, nil
	}
	login, err := a.viewerLogin()
	if err != nil {
		return false, err
	}
	return s.pr.GetUser().GetLogin() == login, nil
}

// choiceFor returns the choice for the submodule at path; without one its drafts fold into the review body.
func choiceFor(choices []model.SubmoduleReviewChoice, path string) model.SubmoduleReviewChoice {
	for _, c := range choices {
		if c.Path == path {
			return c
		}
	}
	return model.SubmoduleReviewChoice{Path: path, Mode: "fold"}
}

// outdatedDrafts counts the pending drafts on the submodule at path written against another bump than s,
// the one loaded now (nil when the PR no longer bumps it).
func (a *App) outdatedDrafts(path string, s *prSubmodule) int {
	n := 0
	for _, d := range a.pendingComments {
		if d.Submodule == path && d.Bump != s.bump() {
			n++
		}
	}
	return n
}

// canGoToSubmodulePR reports whether d can go to its submodule's PR. The submodule is the one d was
// written in: a bump since dropped from the PR or changed leaves its drafts to fold. A parent head move that
// keeps the bump keeps d's lines, which come from the bump's diff.
func (a *App) canGoToSubmodulePR(d model.DraftCommentDTO) bool {
	s := a.prSubmodules[d.Submodule]
	return s != nil && d.Bump == s.bump() && a.toSubmodulePR(s, d)
}

// SubmoduleDrafts says, for each submodule with pending drafts on its files, how many of them can
// go to its open PR and how many SubmitReview adds to the review body regardless. A failed lookup
// of the submodule's PR is retried first, at most once per submoduleRetryDelay unless retry is set, and
// one that found none is repeated past submodulePRMaxAge.
// Error prefix: "auth:" — no client; "notfound:" — no PR loaded; "github:" — API error.
func (a *App) SubmoduleDrafts(retry bool) ([]model.SubmoduleDraftsDTO, error) {
	a.prMu.Lock()
	defer a.prMu.Unlock()
	if a.ghClient == nil {
		return nil, fmt.Errorf("auth: not authenticated")
	}
	if a.prCache == nil {
		return nil, fmt.Errorf("notfound: no PR loaded; call LoadPR first")
	}
	if err := a.loadPRFiles(); err != nil {
		return nil, err
	}
	out := []model.SubmoduleDraftsDTO{}
	for _, d := range a.pendingComments {
		if d.Submodule == "" || slices.ContainsFunc(out, func(o model.SubmoduleDraftsDTO) bool { return o.Path == d.Submodule }) {
			continue
		}
		entry, err := a.submoduleDraftsEntry(d.Submodule, retry)
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	routes := a.draftRoutes()
	for i := range out {
		r := routes[out[i].Path]
		out[i].ToPR, out[i].Folded = r.toPR, r.folded
	}
	return out, nil
}

// route is where a submodule's pending drafts go: how many to its PR, if any, and how many into the review body.
type route struct {
	pr           string
	toPR, folded int
}

// draftRoutes is the route of each submodule with pending drafts, by path.
func (a *App) draftRoutes() map[string]route {
	routes := map[string]route{}
	for _, d := range a.pendingComments {
		if d.Submodule == "" {
			continue
		}
		r := routes[d.Submodule]
		r.pr = a.prSubmodules[d.Submodule].prName()
		if a.canGoToSubmodulePR(d) {
			r.toPR++
		} else {
			r.folded++
		}
		routes[d.Submodule] = r
	}
	return routes
}

// checkRouting rejects choices made on other routes than the drafts' now, as SubmoduleDrafts showed them.
func (a *App) checkRouting(choices []model.SubmoduleReviewChoice) error {
	shown := map[string]route{}
	for _, c := range choices {
		shown[c.Path] = route{c.PR, c.ToPR, c.Folded}
	}
	if !maps.Equal(shown, a.draftRoutes()) {
		return fmt.Errorf("conflict: where the comments on submodule files go changed since you looked; nothing was sent, check them and submit again")
	}
	return nil
}

// submoduleDraftsEntry is the SubmoduleDrafts entry for the submodule at path, without counts.
func (a *App) submoduleDraftsEntry(path string, retry bool) (model.SubmoduleDraftsDTO, error) {
	s := a.prSubmodules[path]
	if s == nil {
		return model.SubmoduleDraftsDTO{Path: path, Outdated: a.outdatedDrafts(path, nil)}, nil
	}
	rerouted := false
	if (s.prUnknown && (retry || s.due())) || s.staleNoPR() {
		a.retrySubmodulePR(s)
	} else {
		rerouted = a.recheckFoldingPR(s)
	}
	own, err := a.ownSubmodulePR(s)
	if err != nil {
		return model.SubmoduleDraftsDTO{}, err
	}
	u := a.frozenModes()[path]
	entry := model.SubmoduleDraftsDTO{Path: path, PR: s.prName(), PRURL: s.pr.GetHTMLURL(), OwnPR: own, PRUnknown: s.prUnknown, InFlightReview: u.inFlight(), Outdated: a.outdatedDrafts(path, s), Rerouted: rerouted}
	if u != nil {
		entry.InFlight = u.mode()
	}
	return entry, nil
}

// recheckFoldingPR reads s's PR again when none of its drafts can go there: a retarget may have brought their
// lines into its diff, and submit rechecks only the PRs drafts go to. A failed read leaves them folding.
// It reports whether the read moved viewed marks, as recheckViewedRoute does.
func (a *App) recheckFoldingPR(s *prSubmodule) bool {
	if s.pr == nil || a.draftRoutes()[s.path].toPR > 0 {
		return false
	}
	rerouted, err := a.rerouted(s, func() error {
		_, err := a.revalidateSubmodulePR(s, false)
		return err
	})
	if err != nil {
		logger.L.Warn("submodule PR recheck failed", "path", s.path, "err", err)
	}
	return rerouted
}
