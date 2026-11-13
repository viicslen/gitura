package app

import (
	"fmt"
	"maps"
	"strings"

	"gitura/internal/fonts"
	"gitura/internal/logger"
	"gitura/internal/model"
	"gitura/internal/settings"
)

// GetCommands returns the current list of configured commands.
// Error prefix: "settings:" — load failure.
func (a *App) GetCommands() ([]model.CommandDTO, error) {
	if err := a.loadConfig(); err != nil {
		return nil, fmt.Errorf("settings: load: %w", err)
	}
	result := make([]model.CommandDTO, len(a.commands))
	copy(result, a.commands)
	return result, nil
}

// AddCommand adds a new command configuration.
// Silently no-ops if a command with the same name already exists.
// Error prefixes: "validation:" — missing name or command; "settings:" — save failure.
func (a *App) AddCommand(cmd model.CommandDTO) ([]model.CommandDTO, error) {
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Command = strings.TrimSpace(cmd.Command)
	if cmd.Name == "" {
		return nil, fmt.Errorf("validation: name required")
	}
	if cmd.Command == "" {
		return nil, fmt.Errorf("validation: command required")
	}
	if err := a.loadConfig(); err != nil {
		return nil, fmt.Errorf("settings: load: %w", err)
	}
	for _, existing := range a.commands {
		if existing.Name == cmd.Name {
			result := make([]model.CommandDTO, len(a.commands))
			copy(result, a.commands)
			return result, nil // already present — no-op
		}
	}
	a.commands = append(a.commands, cmd)
	if err := settings.Save(a.config()); err != nil {
		a.commands = a.commands[:len(a.commands)-1]
		return nil, fmt.Errorf("settings: save: %w", err)
	}
	logger.L.Info("AddCommand", "name", cmd.Name)
	result := make([]model.CommandDTO, len(a.commands))
	copy(result, a.commands)
	return result, nil
}

// RemoveCommand removes the command with the given name.
// Silently no-ops if the name is not found.
// Error prefix: "settings:" — save failure.
func (a *App) RemoveCommand(name string) ([]model.CommandDTO, error) {
	if err := a.loadConfig(); err != nil {
		return nil, fmt.Errorf("settings: load: %w", err)
	}
	idx := -1
	for i, cmd := range a.commands {
		if cmd.Name == name {
			idx = i
			break
		}
	}
	if idx == -1 {
		result := make([]model.CommandDTO, len(a.commands))
		copy(result, a.commands)
		return result, nil
	}
	updated := make([]model.CommandDTO, 0, len(a.commands)-1)
	updated = append(updated, a.commands[:idx]...)
	updated = append(updated, a.commands[idx+1:]...)
	// Clear the default if the removed command was the default.
	if a.defaultCommandName == name {
		a.defaultCommandName = ""
	}
	cfg := a.config()
	cfg.Commands = updated
	if err := settings.Save(cfg); err != nil {
		return nil, fmt.Errorf("settings: save: %w", err)
	}
	a.commands = updated
	logger.L.Info("RemoveCommand", "name", name)
	result := make([]model.CommandDTO, len(a.commands))
	copy(result, a.commands)
	return result, nil
}

// GetKeybindings returns the user's shortcut overrides, keyed by action id.
//
// Error prefix: "settings:" — load failure.
func (a *App) GetKeybindings() (map[string]string, error) {
	if err := a.loadConfig(); err != nil {
		return nil, fmt.Errorf("settings: load: %w", err)
	}
	return maps.Clone(a.keybindings), nil
}

// SetKeybindings replaces the user's shortcut overrides.
//
// Error prefix: "settings:" — load or save failure.
func (a *App) SetKeybindings(bindings map[string]string) error {
	if err := a.loadConfig(); err != nil {
		return fmt.Errorf("settings: load: %w", err)
	}
	cfg := a.config()
	cfg.Keybindings = bindings
	if err := settings.Save(cfg); err != nil {
		return fmt.Errorf("settings: save: %w", err)
	}
	a.keybindings = maps.Clone(bindings)
	return nil
}

// GetDefaultCommandName returns the name of the user's default command, or an
// empty string if none is set.
//
// Error prefix: "settings:" — load failure.
func (a *App) GetDefaultCommandName() (string, error) {
	if err := a.loadConfig(); err != nil {
		return "", fmt.Errorf("settings: load: %w", err)
	}
	return a.defaultCommandName, nil
}

// SetDefaultCommandName persists the given command name as the default. Pass an
// empty string to clear the default.
//
// Error prefix: "validation:" — unknown command name; "settings:" — save failure.
func (a *App) SetDefaultCommandName(name string) error {
	if err := a.loadConfig(); err != nil {
		return fmt.Errorf("settings: load: %w", err)
	}
	if name != "" {
		found := false
		for _, c := range a.commands {
			if c.Name == name {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("validation: unknown command name %q", name)
		}
	}
	a.defaultCommandName = name
	if err := settings.Save(a.config()); err != nil {
		return err
	}
	return nil
}

// GetAppearance returns the persisted font and text-size choices. Empty fields
// mean the frontend default.
//
// Error prefix: "settings:" — load failure.
func (a *App) GetAppearance() (model.AppearanceDTO, error) {
	if err := a.loadConfig(); err != nil {
		return model.AppearanceDTO{}, fmt.Errorf("settings: load: %w", err)
	}
	return a.appearance, nil
}

// SetAppearance persists the font and text-size choices.
//
// Error prefix: "settings:" — load or save failure.
func (a *App) SetAppearance(appearance model.AppearanceDTO) error {
	if err := a.loadConfig(); err != nil {
		return fmt.Errorf("settings: load: %w", err)
	}
	cfg := a.config()
	cfg.Appearance = appearance
	if err := settings.Save(cfg); err != nil {
		return fmt.Errorf("settings: save: %w", err)
	}
	a.appearance = appearance
	return nil
}

// ListSystemFonts returns the font families installed on the system.
//
// Error prefix: "fonts:" — the platform font listing failed.
func (a *App) ListSystemFonts() ([]string, error) {
	return fonts.List(a.ctx)
}

// GetToggles returns the user's on/off preferences, keyed by toggle id.
//
// Error prefix: "settings:" — load failure.
func (a *App) GetToggles() (map[string]bool, error) {
	if err := a.loadConfig(); err != nil {
		return nil, fmt.Errorf("settings: load: %w", err)
	}
	return maps.Clone(a.toggles), nil
}

// SetToggles replaces the user's on/off preferences.
//
// Error prefix: "settings:" — load or save failure.
func (a *App) SetToggles(toggles map[string]bool) error {
	if err := a.loadConfig(); err != nil {
		return fmt.Errorf("settings: load: %w", err)
	}
	cfg := a.config()
	cfg.Toggles = toggles
	if err := settings.Save(cfg); err != nil {
		return fmt.Errorf("settings: save: %w", err)
	}
	a.toggles = maps.Clone(toggles)
	return nil
}
