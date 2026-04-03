// Package settings manages persistent user configuration for gitura,
// stored as a TOML file at ConfigDir()/settings.toml.
package settings

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"gitura/internal/model"
)

const (
	appDir   = "gitura"
	fileName = "settings.toml"
)

// Config holds all user-editable settings for gitura.
type Config struct {
	IgnoredCommenters []model.IgnoredCommenterDTO `toml:"ignored_commenters"`
}

// ConfigDir returns the OS-appropriate configuration directory for gitura,
// e.g. ~/.config/gitura on Linux or ~/Library/Application Support/gitura on macOS.
func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("settings: cannot determine user config dir: %w", err)
	}
	return filepath.Join(base, appDir), nil
}

// Load reads the settings from disk.
// If the file does not exist, a zero-value Config is returned with no error.
func Load() (Config, error) {
	dir, err := ConfigDir()
	if err != nil {
		return Config{}, err
	}

	path := filepath.Join(dir, fileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{IgnoredCommenters: []model.IgnoredCommenterDTO{}}, nil
		}
		return Config{}, fmt.Errorf("settings: read %s: %w", path, err)
	}

	var cfg Config
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return Config{}, fmt.Errorf("settings: parse %s: %w", path, err)
	}
	if cfg.IgnoredCommenters == nil {
		cfg.IgnoredCommenters = []model.IgnoredCommenterDTO{}
	}
	return cfg, nil
}

// Save atomically writes the config to disk.
// The gitura config directory is created if absent.
func Save(cfg Config) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("settings: create config dir %s: %w", dir, err)
	}

	if cfg.IgnoredCommenters == nil {
		cfg.IgnoredCommenters = []model.IgnoredCommenterDTO{}
	}

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return fmt.Errorf("settings: encode: %w", err)
	}

	// Atomic write: write to temp file then rename.
	path := filepath.Join(dir, fileName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o600); err != nil {
		return fmt.Errorf("settings: write temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("settings: rename to %s: %w", path, err)
	}
	return nil
}
