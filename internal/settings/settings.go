// Package settings manages persistent user configuration for gitura,
// including the ignored-commenters list stored as JSON on disk.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gitura/internal/model"
)

const (
	appDir   = "gitura"
	fileName = "ignored_commenters.json"
)

// ConfigDir returns the OS-appropriate configuration directory for gitura,
// e.g. ~/.config/gitura on Linux or ~/Library/Application Support/gitura on macOS.
func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("settings: cannot determine user config dir: %w", err)
	}
	return filepath.Join(base, appDir), nil
}

// Load reads the ignored-commenters list from disk.
// If the file does not exist, an empty (non-nil) slice is returned with no error.
func Load() ([]model.IgnoredCommenterDTO, error) {
	dir, err := ConfigDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(dir, fileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []model.IgnoredCommenterDTO{}, nil
		}
		return nil, fmt.Errorf("settings: read %s: %w", path, err)
	}

	var commenters []model.IgnoredCommenterDTO
	if err := json.Unmarshal(data, &commenters); err != nil {
		return nil, fmt.Errorf("settings: parse %s: %w", path, err)
	}
	if commenters == nil {
		commenters = []model.IgnoredCommenterDTO{}
	}
	return commenters, nil
}

// Save atomically writes the ignored-commenters list to disk.
// An empty list is written as "[]". The gitura config directory is created if absent.
func Save(commenters []model.IgnoredCommenterDTO) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("settings: create config dir %s: %w", dir, err)
	}

	if commenters == nil {
		commenters = []model.IgnoredCommenterDTO{}
	}
	data, err := json.Marshal(commenters)
	if err != nil {
		return fmt.Errorf("settings: marshal: %w", err)
	}

	// Atomic write: write to temp file then rename.
	path := filepath.Join(dir, fileName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("settings: write temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("settings: rename to %s: %w", path, err)
	}
	return nil
}
