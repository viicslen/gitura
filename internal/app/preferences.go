package app

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"gitura/internal/fonts"
	"gitura/internal/logger"
	"gitura/internal/model"
	"gitura/internal/runner"
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
	err := a.updateConfig(func(cfg *settings.Config) error {
		if !slices.ContainsFunc(cfg.Commands, func(c model.CommandDTO) bool { return c.Name == cmd.Name }) {
			cfg.Commands = append(cfg.Commands, cmd)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	logger.L.Info("AddCommand", "name", cmd.Name)
	return slices.Clone(a.commands), nil
}

// RemoveCommand removes the command with the given name.
// Silently no-ops if the name is not found.
// Error prefix: "settings:" — save failure.
func (a *App) RemoveCommand(name string) ([]model.CommandDTO, error) {
	err := a.updateConfig(func(cfg *settings.Config) error {
		cfg.Commands = slices.DeleteFunc(cfg.Commands, func(c model.CommandDTO) bool { return c.Name == name })
		cfg.DefaultCommands = renameDefaults(cfg.DefaultCommands, name, "")
		return nil
	})
	if err != nil {
		return nil, err
	}
	logger.L.Info("RemoveCommand", "name", name)
	return slices.Clone(a.commands), nil
}

// UpdateCommand replaces the command named name with cmd, which may rename it.
// A renamed default command stays the default for its kinds.
// Error prefixes: "validation:" — missing name or command, name taken by another
// command; "not_found:" — unknown name; "settings:" — load or save failure.
func (a *App) UpdateCommand(name string, cmd model.CommandDTO) ([]model.CommandDTO, error) {
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Command = strings.TrimSpace(cmd.Command)
	if cmd.Name == "" || cmd.Command == "" {
		return nil, fmt.Errorf("validation: name and command required")
	}
	err := a.updateConfig(func(cfg *settings.Config) error {
		idx := slices.IndexFunc(cfg.Commands, func(c model.CommandDTO) bool { return c.Name == name })
		if idx == -1 {
			return fmt.Errorf("not_found: command %q", name)
		}
		if cmd.Name != name && slices.ContainsFunc(cfg.Commands, func(c model.CommandDTO) bool { return c.Name == cmd.Name }) {
			return fmt.Errorf("validation: a command named %q already exists", cmd.Name)
		}
		cfg.Commands[idx] = cmd
		cfg.DefaultCommands = renameDefaults(cfg.DefaultCommands, name, cmd.Name)
		return nil
	})
	if err != nil {
		return nil, err
	}
	logger.L.Info("UpdateCommand", "name", name, "new_name", cmd.Name)
	return slices.Clone(a.commands), nil
}

// GetCommandVariables lists the PR and comment placeholders a command can use.
func (a *App) GetCommandVariables() []model.CommandVariableDTO {
	return slices.Clone(runner.Variables)
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
	return a.updateConfig(func(cfg *settings.Config) error {
		cfg.Keybindings = maps.Clone(bindings)
		return nil
	})
}

// GetDefaultCommands returns the default command name for each run target kind
// that has one; "all" is the fallback for the others.
//
// Error prefix: "settings:" — load failure.
func (a *App) GetDefaultCommands() (map[string]string, error) {
	if err := a.loadConfig(); err != nil {
		return nil, fmt.Errorf("settings: load: %w", err)
	}
	return maps.Clone(a.defaultCommands), nil
}

// SetDefaultCommand persists name as the default command for kind. Pass an
// empty name to clear it, so kind falls back to "all".
//
// Error prefix: "validation:" — empty kind or unknown command name; "settings:" — load or save failure.
func (a *App) SetDefaultCommand(kind, name string) error {
	if kind == "" {
		return fmt.Errorf("validation: kind required")
	}
	return a.updateConfig(func(cfg *settings.Config) error {
		if name != "" && !slices.ContainsFunc(cfg.Commands, func(c model.CommandDTO) bool { return c.Name == name }) {
			return fmt.Errorf("validation: unknown command name %q", name)
		}
		if cfg.DefaultCommands == nil {
			cfg.DefaultCommands = map[string]string{}
		}
		if name == "" {
			delete(cfg.DefaultCommands, kind)
		} else {
			cfg.DefaultCommands[kind] = name
		}
		return nil
	})
}

// renameDefaults returns defaults with every kind set to from pointed at to, or dropped when to is empty.
func renameDefaults(defaults map[string]string, from, to string) map[string]string {
	out := make(map[string]string, len(defaults))
	for kind, name := range defaults {
		switch {
		case name != from:
			out[kind] = name
		case to != "":
			out[kind] = to
		}
	}
	return out
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
	return a.updateConfig(func(cfg *settings.Config) error {
		cfg.Appearance = appearance
		return nil
	})
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
	return a.updateConfig(func(cfg *settings.Config) error {
		cfg.Toggles = maps.Clone(toggles)
		return nil
	})
}
