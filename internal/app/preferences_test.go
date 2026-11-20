package app

import (
	"reflect"
	"testing"

	"gitura/internal/model"
	"gitura/internal/settings"
)

// isolateConfig points os.UserConfigDir at a temp dir on every OS, so a test never writes the real settings.toml.
func isolateConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
}

func TestSetAppearance_OtherSettingsAndRestart_BothPersist(t *testing.T) {
	isolateConfig(t)

	others := settings.Config{
		IgnoredCommenters: []model.IgnoredCommenterDTO{{Login: "renovate"}},
		Commands:          []model.CommandDTO{{Name: "agent", Command: "opencode run"}},
		DefaultCommands:   map[string]string{"all": "agent"},
	}
	if err := settings.Save(others); err != nil {
		t.Fatalf("seed settings: %v", err)
	}

	want := model.AppearanceDTO{TextFont: "atkinson", CodeFont: "system", TextSize: "large", Theme: "dark", Effects: "off"}
	a := New("")
	if err := a.SetToggles(map[string]bool{"wrap_lines": false}); err != nil {
		t.Fatalf("set toggles: %v", err)
	}
	if err := a.SetAppearance(want); err != nil {
		t.Fatalf("set appearance: %v", err)
	}
	if err := a.AddIgnoredCommenter("bot"); err != nil {
		t.Fatalf("add ignored commenter: %v", err)
	}

	cfg, err := settings.Load()
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if cfg.Appearance != want {
		t.Fatalf("appearance on disk: got %+v want %+v", cfg.Appearance, want)
	}
	wantIgnored := []model.IgnoredCommenterDTO{{Login: "renovate"}, {Login: "bot"}}
	if !reflect.DeepEqual(cfg.IgnoredCommenters, wantIgnored) || !reflect.DeepEqual(cfg.Commands, others.Commands) || !reflect.DeepEqual(cfg.DefaultCommands, others.DefaultCommands) || !reflect.DeepEqual(cfg.Toggles, map[string]bool{"wrap_lines": false}) {
		t.Fatalf("other settings not preserved: %+v", cfg)
	}

	got, err := New("").GetAppearance()
	if err != nil {
		t.Fatalf("get appearance after restart: %v", err)
	}
	if got != want {
		t.Fatalf("appearance after restart: got %+v want %+v", got, want)
	}
}

func TestSetAppearance_AfterSetKeybindings_KeepsBoth(t *testing.T) {
	isolateConfig(t)
	appearance := model.AppearanceDTO{TextFont: "atkinson", CodeFont: "system", TextSize: "large", Theme: "dark", Effects: "off"}
	keys := map[string]string{"nextFile": "j"}

	a := New("")
	if err := a.SetKeybindings(keys); err != nil {
		t.Fatalf("set keybindings: %v", err)
	}
	if err := a.SetAppearance(appearance); err != nil {
		t.Fatalf("set appearance: %v", err)
	}
	if cfg, err := settings.Load(); err != nil || !reflect.DeepEqual(cfg.Keybindings, keys) {
		t.Fatalf("SetAppearance dropped keybindings: %+v, %v", cfg.Keybindings, err)
	}
	if err := a.SetKeybindings(map[string]string{"nextFile": "k"}); err != nil {
		t.Fatalf("set keybindings: %v", err)
	}
	if cfg, err := settings.Load(); err != nil || cfg.Appearance != appearance {
		t.Fatalf("SetKeybindings dropped appearance: %+v, %v", cfg.Appearance, err)
	}
}

func TestUpdateCommand_RenameDefault_PersistsAndKeepsDefault(t *testing.T) {
	isolateConfig(t)
	seed := settings.Config{
		Commands:        []model.CommandDTO{{Name: "fix", Command: "t3 thread start . x"}, {Name: "other", Command: "echo"}},
		DefaultCommands: map[string]string{"all": "fix", "pr": "other", "thread": "fix"},
	}
	if err := settings.Save(seed); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	a := New("")

	if _, err := a.UpdateCommand("fix", model.CommandDTO{Name: "other", Command: "echo"}); err == nil {
		t.Fatal("rename onto an existing name: want error")
	}
	got, err := a.UpdateCommand("fix", model.CommandDTO{Name: " Fix with T3 ", Command: "t3 thread start . {{pr_number}}"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	want := []model.CommandDTO{{Name: "Fix with T3", Command: "t3 thread start . {{pr_number}}"}, {Name: "other", Command: "echo"}}
	cfg, err := settings.Load()
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(cfg.Commands, want) || !reflect.DeepEqual(cfg.DefaultCommands, map[string]string{"all": "Fix with T3", "pr": "other", "thread": "Fix with T3"}) {
		t.Fatalf("got %+v, on disk %+v", got, cfg)
	}
}

func TestSetDefaultCommand_PerKindThenRemove_ClearsOnlyRemovedKinds(t *testing.T) {
	isolateConfig(t)
	if err := settings.Save(settings.Config{Commands: []model.CommandDTO{{Name: "fix", Command: "a"}, {Name: "eval", Command: "b"}}}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	a := New("")
	if err := a.SetDefaultCommand("pr", "missing"); err == nil {
		t.Fatal("unknown command: want error")
	}
	for kind, name := range map[string]string{"all": "fix", "pr": "eval", "draft": "eval"} {
		if err := a.SetDefaultCommand(kind, name); err != nil {
			t.Fatalf("set %s: %v", kind, err)
		}
	}
	if err := a.SetDefaultCommand("draft", ""); err != nil {
		t.Fatalf("clear draft: %v", err)
	}
	if _, err := a.RemoveCommand("eval"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	got, err := New("").GetDefaultCommands()
	if err != nil || !reflect.DeepEqual(got, map[string]string{"all": "fix"}) {
		t.Fatalf("defaults after restart: %+v, %v", got, err)
	}
}
