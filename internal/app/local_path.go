package app

import (
	"fmt"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"gitura/internal/db"
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

// GetPRLocalPath returns the local filesystem path associated with a PR,
// or an empty string if none has been set.
//
// Error prefix: "validation:" — invalid owner/repo/number; "db:" — database query failure.
func (a *App) GetPRLocalPath(owner, repo string, number int) (string, error) {
	if err := validatePRIdentity(owner, repo, number); err != nil {
		return "", err
	}
	if a.queries == nil {
		return "", nil
	}
	state, err := a.queries.GetPRState(a.ctx, db.GetPRStateParams{
		Owner:  owner,
		Repo:   repo,
		Number: int64(number),
	})
	if err != nil {
		// sql.ErrNoRows is expected when no path has been set yet.
		return "", nil
	}
	return state.LocalPath, nil
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
	opts := runtime.OpenDialogOptions{
		Title:            title,
		DefaultDirectory: defaultDir,
	}
	selected, err := runtime.OpenDirectoryDialog(a.ctx, opts)
	if err != nil {
		return "", err
	}
	return selected, nil
}
