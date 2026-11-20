package app

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"gitura/internal/db"
	"gitura/internal/gitlocal"
	"gitura/internal/logger"
	"gitura/internal/settings"
)

// validatePRIdentity checks whether owner/repo/number can uniquely identify a PR.
func validatePRIdentity(owner, repo string, number int) error {
	if strings.TrimSpace(owner) == "" {
		return fmt.Errorf("validation: owner is required")
	}
	if strings.TrimSpace(repo) == "" {
		return fmt.Errorf("validation: repo is required")
	}
	if number <= 0 {
		return fmt.Errorf("validation: PR number must be greater than zero")
	}
	return nil
}

// rescanInterval limits how often a clone that can't be found triggers another project folder scan.
const rescanInterval = time.Minute

// GetPRLocalPath returns the local checkout for a PR: the path set for it or, when none is, one
// found and saved for it; empty when no clone is found. See prLocalPath.
//
// Error prefix: "validation:" — invalid owner/repo/number.
func (a *App) GetPRLocalPath(owner, repo string, number int) (string, error) {
	if err := validatePRIdentity(owner, repo, number); err != nil {
		return "", err
	}
	return a.prLocalPath(owner, repo, number), nil
}

// prLocalPath returns the PR's saved path; otherwise it takes the clone saved for another PR of the
// repository or finds one in the project folders, prefers its worktree on the PR's head branch, and
// saves that for the PR.
func (a *App) prLocalPath(owner, repo string, number int) string {
	if a.queries != nil {
		state, err := a.queries.GetPRState(a.ctx, db.GetPRStateParams{Owner: owner, Repo: repo, Number: int64(number)})
		if err == nil && state.LocalPath != "" {
			return state.LocalPath
		}
	}
	clone := a.repoClone(owner, repo)
	if clone == "" {
		return ""
	}
	head := ""
	if a.isLoadedPR(owner, repo, number) {
		head = a.prCache.HeadBranch
	}
	path := gitlocal.WorktreeFor(a.ctx, clone, head)
	if a.queries != nil {
		err := a.queries.UpsertPRLocalPath(a.ctx, db.UpsertPRLocalPathParams{Owner: owner, Repo: repo, Number: int64(number), LocalPath: path})
		if err != nil {
			logger.L.Warn("save found local path", "path", path, "err", err)
		}
	}
	return path
}

// repoClone returns a local clone of owner/repo, or an empty string when none is known.
func (a *App) repoClone(owner, repo string) string {
	if a.queries != nil {
		p, err := a.queries.GetRepoLocalPath(a.ctx, db.GetRepoLocalPathParams{Owner: owner, Repo: repo})
		if err == nil && isDir(p) {
			return p
		}
	}
	if err := a.loadConfig(); err != nil {
		return ""
	}
	key := strings.ToLower(owner + "/" + repo)
	a.repoIndexMu.Lock()
	defer a.repoIndexMu.Unlock()
	// Read under repoIndexMu: applyConfig clears the index under it after swapping in new folders.
	projectDirs := a.conf().ProjectDirs
	if len(projectDirs) == 0 {
		return ""
	}
	if p := a.repoIndex[key]; p != "" && isDir(p) {
		return p
	}
	if time.Since(a.repoIndexAt) < rescanInterval {
		return ""
	}
	dirs := make([]string, len(projectDirs))
	for i, d := range projectDirs {
		dirs[i] = expandHome(d)
	}
	a.repoIndex = gitlocal.FindRepos(dirs)
	a.repoIndexAt = time.Now()
	return a.repoIndex[key]
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func expandHome(path string) string {
	rest, ok := strings.CutPrefix(path, "~")
	if !ok || (rest != "" && rest[0] != '/' && rest[0] != filepath.Separator) {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, rest)
}

// GetProjectDirs returns the folders searched for a PR's local clone.
//
// Error prefix: "settings:" — load failure.
func (a *App) GetProjectDirs() ([]string, error) {
	if err := a.loadConfig(); err != nil {
		return nil, fmt.Errorf("settings: load: %w", err)
	}
	return slices.Clone(a.conf().ProjectDirs), nil
}

// SetProjectDirs replaces the folders searched for a PR's local clone.
//
// Error prefix: "settings:" — load or save failure.
func (a *App) SetProjectDirs(dirs []string) error {
	_, err := a.updateConfig(func(cfg *settings.Config) error {
		cfg.ProjectDirs = slices.Clone(dirs)
		return nil
	})
	return err
}

// SetPRLocalPath persists a local filesystem path for a PR.
// Pass an empty string to clear the stored path.
//
// Error prefix: "validation:" — invalid owner/repo/number; "db:" — database write failure.
func (a *App) SetPRLocalPath(owner, repo string, number int, localPath string) error {
	if err := validatePRIdentity(owner, repo, number); err != nil {
		return err
	}
	if a.queries == nil {
		return fmt.Errorf("db: state database is not available")
	}
	return a.queries.UpsertPRLocalPath(a.ctx, db.UpsertPRLocalPathParams{
		Owner:     owner,
		Repo:      repo,
		Number:    int64(number),
		LocalPath: localPath,
	})
}

// OpenFolderPicker opens a native OS folder picker dialog and returns the
// selected directory path, or an empty string if the user cancels.
func (a *App) OpenFolderPicker(title string, defaultDir string) (string, error) {
	dialog := application.Get().Dialog.OpenFile()
	if w, ok := mainWindow(); ok {
		dialog.AttachToWindow(w)
	}
	return dialog.
		CanChooseFiles(false).
		CanChooseDirectories(true).
		CanCreateDirectories(true).
		SetTitle(title).
		SetDirectory(defaultDir).
		PromptForSingleSelection()
}
