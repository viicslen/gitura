package app

import (
	"fmt"
	"slices"
	"strings"

	"gitura/internal/logger"
	"gitura/internal/model"
	"gitura/internal/settings"
)

// GetIgnoredCommenters returns the current list of ignored-commenter entries.
// The list is lazily loaded from disk on the first call.
// Error prefix: none — returns empty slice on load failure.
func (a *App) GetIgnoredCommenters() ([]model.IgnoredCommenterDTO, error) {
	if err := a.loadConfig(); err != nil {
		return nil, fmt.Errorf("settings: load: %w", err)
	}
	result := make([]model.IgnoredCommenterDTO, len(a.ignoredCommenters))
	copy(result, a.ignoredCommenters)
	return result, nil
}

// AddIgnoredCommenter adds a GitHub login to the ignored-commenters list.
// Silently no-ops if the login is already present.
// Error prefixes: "validation:" — empty login; "settings:" — save failure.
func (a *App) AddIgnoredCommenter(login string) error {
	login = strings.TrimSpace(login)
	if login == "" {
		return fmt.Errorf("validation: login required")
	}
	err := a.updateConfig(func(cfg *settings.Config) error {
		if !slices.ContainsFunc(cfg.IgnoredCommenters, func(ic model.IgnoredCommenterDTO) bool { return ic.Login == login }) {
			cfg.IgnoredCommenters = append(cfg.IgnoredCommenters, model.IgnoredCommenterDTO{Login: login})
		}
		return nil
	})
	if err != nil {
		return err
	}
	logger.L.Info("AddIgnoredCommenter", "login", login)
	return nil
}

// RemoveIgnoredCommenter removes a GitHub login from the ignored-commenters list.
// Silently no-ops if the login is not present.
// Error prefix: "settings:" — save failure.
func (a *App) RemoveIgnoredCommenter(login string) error {
	err := a.updateConfig(func(cfg *settings.Config) error {
		cfg.IgnoredCommenters = slices.DeleteFunc(cfg.IgnoredCommenters, func(ic model.IgnoredCommenterDTO) bool { return ic.Login == login })
		return nil
	})
	if err != nil {
		return err
	}
	logger.L.Info("RemoveIgnoredCommenter", "login", login)
	return nil
}
