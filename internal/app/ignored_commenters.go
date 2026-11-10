package app

import (
	"fmt"
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
	if err := a.loadConfig(); err != nil {
		return fmt.Errorf("settings: load: %w", err)
	}
	for _, ic := range a.ignoredCommenters {
		if ic.Login == login {
			return nil // already present — no-op
		}
	}
	a.ignoredCommenters = append(a.ignoredCommenters, model.IgnoredCommenterDTO{
		Login: login,
	})
	if err := settings.Save(settings.Config{IgnoredCommenters: a.ignoredCommenters, Commands: a.commands, DefaultCommandName: a.defaultCommandName, Appearance: a.appearance, Keybindings: a.keybindings}); err != nil {
		// Roll back in-memory change.
		a.ignoredCommenters = a.ignoredCommenters[:len(a.ignoredCommenters)-1]
		return fmt.Errorf("settings: save: %w", err)
	}
	logger.L.Info("AddIgnoredCommenter", "login", login)
	return nil
}

// RemoveIgnoredCommenter removes a GitHub login from the ignored-commenters list.
// Silently no-ops if the login is not present.
// Error prefix: "settings:" — save failure.
func (a *App) RemoveIgnoredCommenter(login string) error {
	if err := a.loadConfig(); err != nil {
		return fmt.Errorf("settings: load: %w", err)
	}
	idx := -1
	for i, ic := range a.ignoredCommenters {
		if ic.Login == login {
			idx = i
			break
		}
	}
	if idx == -1 {
		return nil // not present — no-op
	}
	updated := make([]model.IgnoredCommenterDTO, 0, len(a.ignoredCommenters)-1)
	updated = append(updated, a.ignoredCommenters[:idx]...)
	updated = append(updated, a.ignoredCommenters[idx+1:]...)
	if err := settings.Save(settings.Config{IgnoredCommenters: updated, Commands: a.commands, DefaultCommandName: a.defaultCommandName, Appearance: a.appearance, Keybindings: a.keybindings}); err != nil {
		return fmt.Errorf("settings: save: %w", err)
	}
	a.ignoredCommenters = updated
	logger.L.Info("RemoveIgnoredCommenter", "login", login)
	return nil
}
