package app

import (
	"context"
	"fmt"
	"time"

	"gitura/internal/gitlocal"
	"gitura/internal/model"
)

type localReview struct {
	dir, base string
	files     []gitlocal.File
	// viewed maps a path to its patch when marked viewed; a changed patch unmarks it, like GitHub.
	viewed map[string]string
	stop   context.CancelFunc
}

// StartLocalReview switches the Files view to the changes in a local checkout. base is
// gitlocal.BaseMergeBase or gitlocal.BaseHEAD. Restarting on the same repository keeps
// draft comments and viewed marks. Emits "local:changed" when the working tree changes.
// Returns the repository root.
//
// Error prefixes: "validation:" — unknown base; "git:" — not a repository or git failed.
func (a *App) StartLocalReview(path, base string) (string, error) {
	dir, err := gitlocal.TopLevel(a.ctx, path)
	if err != nil {
		return "", err
	}
	if _, err := gitlocal.ResolveBase(a.ctx, dir, base); err != nil {
		return "", err
	}
	viewed := map[string]string{}
	if a.local != nil && a.local.dir == dir {
		viewed = a.local.viewed
	} else {
		a.pendingReviewID = 0
		a.pendingComments = nil
		a.pendingHeadSHA = ""
	}
	a.StopLocalReview()
	a.prCache = nil
	a.prFilesCache = nil
	a.viewedFiles = nil

	ctx, cancel := context.WithCancel(a.ctx)
	a.local = &localReview{dir: dir, base: base, viewed: viewed, stop: cancel}
	go a.watchLocal(ctx, dir)
	return dir, nil
}

// StopLocalReview leaves local review mode; its drafts are dropped on the next load.
func (a *App) StopLocalReview() {
	if a.local != nil {
		a.local.stop()
		a.local = nil
	}
}

// watchLocal polls rather than using OS file events: git status already honours .gitignore,
// and recursive inotify watches run out on large trees like node_modules.
func (a *App) watchLocal(ctx context.Context, dir string) {
	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()
	last, _ := gitlocal.Fingerprint(ctx, dir)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		fp, err := gitlocal.Fingerprint(ctx, dir)
		if err != nil || fp == last {
			continue
		}
		last = fp
		a.emit("local:changed")
	}
}

func (a *App) localFiles() ([]model.PRFileDTO, error) {
	files, err := gitlocal.Diff(a.ctx, a.local.dir, a.local.base)
	if err != nil {
		return nil, err
	}
	a.local.files = files
	result := make([]model.PRFileDTO, 0, len(files))
	for _, f := range files {
		d := f.Diff
		result = append(result, model.PRFileDTO{
			Filename:         d.Filename,
			Status:           d.Status,
			Additions:        d.TotalAdditions,
			Deletions:        d.TotalDeletions,
			Changes:          d.TotalAdditions + d.TotalDeletions,
			PreviousFilename: d.PreviousFilename,
			IsBinary:         d.IsBinary,
			Viewed:           a.local.isViewed(f),
		})
	}
	return result, nil
}

func (l *localReview) isViewed(f gitlocal.File) bool {
	patch, ok := l.viewed[f.Diff.Filename]
	return ok && patch == f.Patch
}

func (l *localReview) file(path string) (gitlocal.File, error) {
	for _, f := range l.files {
		if f.Diff.Filename == path {
			return f, nil
		}
	}
	return gitlocal.File{}, fmt.Errorf("notfound: file %q has no local changes", path)
}
