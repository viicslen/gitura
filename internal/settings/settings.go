// Package settings manages persistent user configuration for gitura,
// including the ignored-commenters list stored as TOML on disk.
package settings

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/kirsle/configdir"

	"gitura/internal/model"
)

const fileName = "settings.toml"

// settingsFile is the top-level TOML document structure.
type settingsFile struct {
	IgnoredCommenters []ignoredCommenter `toml:"ignored_commenters"`
}

// ignoredCommenter mirrors model.IgnoredCommenterDTO with explicit TOML tags.
type ignoredCommenter struct {
	Login   string    `toml:"login"`
	AddedAt time.Time `toml:"added_at"`
}

// ConfigDir returns the OS-appropriate configuration directory for gitura,
// e.g. ~/.config/gitura on Linux or ~/Library/Application Support/gitura on macOS.
func ConfigDir() (string, error) {
	dir := configdir.LocalConfig("gitura")
	if dir == "" {
		return "", fmt.Errorf("settings: cannot determine user config dir")
	}
	return dir, nil
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

	var sf settingsFile
	if err := toml.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("settings: parse %s: %w", path, err)
	}

	result := make([]model.IgnoredCommenterDTO, len(sf.IgnoredCommenters))
	for i, c := range sf.IgnoredCommenters {
		result[i] = model.IgnoredCommenterDTO{Login: c.Login, AddedAt: c.AddedAt}
	}
	return result, nil
}

// Save atomically writes the ignored-commenters list to disk as TOML.
// The gitura config directory is created if absent.
func Save(commenters []model.IgnoredCommenterDTO) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	if err := configdir.MakePath(dir); err != nil {
		return fmt.Errorf("settings: create config dir %s: %w", dir, err)
	}

	entries := make([]ignoredCommenter, len(commenters))
	for i, c := range commenters {
		entries[i] = ignoredCommenter{Login: c.Login, AddedAt: c.AddedAt}
	}
	sf := settingsFile{IgnoredCommenters: entries}

	data, err := toml.Marshal(sf)
	if err != nil {
		return fmt.Errorf("settings: marshal: %w", err)
	}

	// Atomic write: write to temp file then rename.
	// On Windows os.Rename fails when the destination already exists, so we
	// attempt to remove it first (no-op if it doesn't exist yet). If Remove
	// fails (e.g., destination is a directory) the subsequent Rename will also
	// fail and return a descriptive error to the caller.
	path := filepath.Join(dir, fileName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("settings: write temp file: %w", err)
	}
	_ = os.Remove(path)
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("settings: rename to %s: %w", path, err)
	}
	return nil
}
