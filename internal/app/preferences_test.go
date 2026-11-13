package app

import (
	"reflect"
	"testing"

	"gitura/internal/model"
	"gitura/internal/settings"
)

func TestSetAppearance_OtherSettingsAndRestart_BothPersist(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	others := settings.Config{
		IgnoredCommenters:  []model.IgnoredCommenterDTO{{Login: "renovate"}},
		Commands:           []model.CommandDTO{{Name: "agent", Command: "opencode run"}},
		DefaultCommandName: "agent",
	}
	if err := settings.Save(others); err != nil {
		t.Fatalf("seed settings: %v", err)
	}

	want := model.AppearanceDTO{TextFont: "atkinson", CodeFont: "system", TextSize: "large", Theme: "dark"}
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
	if !reflect.DeepEqual(cfg.IgnoredCommenters, wantIgnored) || !reflect.DeepEqual(cfg.Commands, others.Commands) || cfg.DefaultCommandName != "agent" || !reflect.DeepEqual(cfg.Toggles, map[string]bool{"wrap_lines": false}) {
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
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	appearance := model.AppearanceDTO{TextFont: "atkinson", CodeFont: "system", TextSize: "large", Theme: "dark"}
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
