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
	Commands          []model.CommandDTO          `toml:"commands"`
	// DefaultCommandName is the command name to use as the primary action
	// in split-run buttons. Empty string means no default is set.
	DefaultCommandName string `toml:"default_command_name"`
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
	data, err := readSettingsFile(path)
	if err != nil {
		return Config{}, err
	}
	if data == nil {
		return Config{IgnoredCommenters: []model.IgnoredCommenterDTO{}}, nil
	}

	raw, err := decodeConfig(data, path)
	if err != nil {
		return Config{}, err
	}

	cfg := buildConfig(raw)
	ensureSlices(&cfg)

	return cfg, nil
}

type legacyCommand struct {
	ID      string `toml:"id"`
	Name    string `toml:"name"`
	Command string `toml:"command"`
}

type decodedConfig struct {
	IgnoredCommenters  []model.IgnoredCommenterDTO `toml:"ignored_commenters"`
	Commands           []legacyCommand             `toml:"commands"`
	DefaultCommandName string                      `toml:"default_command_name"`
	DefaultCommandID   string                      `toml:"default_command_id"`
}

func readSettingsFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return data, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}

	return nil, fmt.Errorf("settings: read %s: %w", path, err)
}

func decodeConfig(data []byte, path string) (decodedConfig, error) {
	var raw decodedConfig
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return decodedConfig{}, fmt.Errorf("settings: parse %s: %w", path, err)
	}

	return raw, nil
}

func buildConfig(raw decodedConfig) Config {
	cfg := Config{
		IgnoredCommenters:  raw.IgnoredCommenters,
		Commands:           make([]model.CommandDTO, 0, len(raw.Commands)),
		DefaultCommandName: raw.DefaultCommandName,
	}
	for _, c := range raw.Commands {
		cfg.Commands = append(cfg.Commands, model.CommandDTO{
			Name:    c.Name,
			Command: c.Command,
		})
	}

	resolveLegacyDefaultCommandName(&cfg, raw)

	return cfg
}

func resolveLegacyDefaultCommandName(cfg *Config, raw decodedConfig) {
	if cfg.DefaultCommandName == "" && raw.DefaultCommandID != "" {
		for _, c := range raw.Commands {
			if c.ID == raw.DefaultCommandID {
				cfg.DefaultCommandName = c.Name
				break
			}
		}
	}
}

func ensureSlices(cfg *Config) {
	if cfg.IgnoredCommenters == nil {
		cfg.IgnoredCommenters = []model.IgnoredCommenterDTO{}
	}
	if cfg.Commands == nil {
		cfg.Commands = []model.CommandDTO{}
	}
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
	if cfg.Commands == nil {
		cfg.Commands = []model.CommandDTO{}
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
