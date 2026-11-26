package app

import (
	"context"
	"fmt"
	"maps"
	"sync"
	"time"

	"gitura/internal/gitlocal"
	"gitura/internal/model"
)

type localReview struct {
	dir, base string
	// mu guards files and viewed: the auto-refresh replaces files while other bound calls read them.
	mu    sync.Mutex
	files []gitlocal.File
	// viewed maps a file to its patch when marked viewed; a changed patch unmarks it, like GitHub.
	viewed map[fileID]string
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
	viewed := map[fileID]string{}
	old := a.local.Load()
	sameDir := old != nil && old.dir == dir
	if sameDir {
		old.mu.Lock()
		viewed = maps.Clone(old.viewed)
		old.mu.Unlock()
	}
	a.prMu.Lock()
	if !sameDir {
		a.resetDrafts()
	}
	a.prCache = nil
	a.prFilesCache, a.prSubmodules, a.prSubmoduleOf = nil, nil, nil
	a.viewedFiles = nil
	a.prMu.Unlock()

	ctx, cancel := context.WithCancel(a.ctx)
	// One swap, not a stop then a store, so two overlapping starts can't leave a watcher running.
	if prev := a.local.Swap(&localReview{dir: dir, base: base, viewed: viewed, stop: cancel}); prev != nil {
		prev.stop()
	}
	go a.watchLocal(ctx, dir)
	return dir, nil
}

// StopLocalReview leaves local review mode; its drafts are dropped on the next load.
func (a *App) StopLocalReview() {
	if l := a.local.Swap(nil); l != nil {
		l.stop()
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

// rows refreshes l's files from git and lists them.
func (l *localReview) rows(ctx context.Context) ([]model.PRFileDTO, error) {
	files, err := gitlocal.Diff(ctx, l.dir, l.base)
	if err != nil {
		return nil, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.files = files
	result := make([]model.PRFileDTO, 0, len(files))
	for _, f := range files {
		d := f.Diff
		result = append(result, model.PRFileDTO{
			Filename:             d.Filename,
			Status:               d.Status,
			Additions:            d.TotalAdditions,
			Deletions:            d.TotalDeletions,
			Changes:              d.TotalAdditions + d.TotalDeletions,
			PreviousFilename:     d.PreviousFilename,
			IsBinary:             d.IsBinary,
			Viewed:               l.isViewed(f),
			Repo:                 localID(f).repo,
			Submodule:            f.Submodule,
			SubmoduleRange:       f.SubmoduleRange,
			SubmoduleBackwards:   f.SubmoduleBackwards,
			SubmoduleDiverged:    f.SubmoduleDiverged,
			SubmoduleUnavailable: f.SubmoduleUnavailable,
		})
	}
	return result, nil
}

func (l *localReview) viewedKeys() []model.FileKeyDTO {
	l.mu.Lock()
	defer l.mu.Unlock()
	keys := []model.FileKeyDTO{}
	for _, f := range l.files {
		if l.isViewed(f) {
			id := localID(f)
			keys = append(keys, model.FileKeyDTO{Submodule: id.repo, Path: id.path, Pointer: id.pointer})
		}
	}
	return keys
}

func localID(f gitlocal.File) fileID {
	if f.SubmoduleRange != "" {
		return fileID{f.SubmoduleIn, f.Diff.Filename, true}
	}
	return fileID{f.Submodule, f.Diff.Filename, false}
}

func (l *localReview) isViewed(f gitlocal.File) bool {
	patch, ok := l.viewed[localID(f)]
	return ok && patch == f.Patch
}

func (l *localReview) setViewed(id fileID, viewed bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	f, err := l.fileLocked(id)
	if err != nil {
		return err
	}
	if viewed {
		l.viewed[id] = f.Patch
	} else {
		delete(l.viewed, id)
	}
	return nil
}

func (l *localReview) file(id fileID) (gitlocal.File, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.fileLocked(id)
}

func (l *localReview) fileLocked(id fileID) (gitlocal.File, error) {
	for _, f := range l.files {
		if localID(f) == id {
			return f, nil
		}
	}
	return gitlocal.File{}, fmt.Errorf("notfound: file %q has no local changes", id.path)
}

// checkSubmodule rejects a draft on a submodule file or pointer row that isn't a changed one.
func (l *localReview) checkSubmodule(d model.DraftCommentDTO) error {
	if d.Submodule == "" && !d.Pointer {
		return nil
	}
	if _, err := l.file(fileID{d.Submodule, d.Path, d.Pointer}); err != nil {
		return fmt.Errorf("validation: %q is not a changed file in submodule %q", d.Path, d.Submodule)
	}
	return nil
}
