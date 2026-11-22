package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupHome points HOME at a fresh temp dir and the system config at a path
// inside it, so tests never touch the real sway config.
func setupHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	orig := SystemConfigPath
	SystemConfigPath = filepath.Join(home, "etc-sway-config")
	t.Cleanup(func() { SystemConfigPath = orig })
	return home
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}

func mustContain(t *testing.T, content, want string) {
	t.Helper()
	if !strings.Contains(content, want) {
		t.Fatalf("expected content to contain %q, got:\n%s", want, content)
	}
}

func TestLoadWithNoConfigDirReturnsDefaults(t *testing.T) {
	setupHome(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load with missing config dir: %v", err)
	}
	if cfg.ModKey != "Mod4" || cfg.GapsInner != 4 || cfg.BlurPasses != 2 {
		t.Fatalf("expected defaults, got %+v", cfg)
	}
	if len(cfg.Outputs) != 0 {
		t.Fatalf("expected no outputs, got %+v", cfg.Outputs)
	}
}

func TestLoadFallsBackToSystemConfigForMainValues(t *testing.T) {
	setupHome(t)
	if err := os.WriteFile(SystemConfigPath, []byte("set $mod Mod1\nset $term foot\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ModKey != "Mod1" || cfg.Terminal != "foot" {
		t.Fatalf("expected values from system config, got mod=%q term=%q", cfg.ModKey, cfg.Terminal)
	}
}

func TestLoadSurfacesNonMissingErrors(t *testing.T) {
	setupHome(t)
	// A directory where a file is expected is a real error, not "missing".
	if err := os.MkdirAll(AppearancePath(), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("expected error when appearance path is a directory")
	}
}

func TestSaveCreatesDirsAndFiles(t *testing.T) {
	setupHome(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.ModKey = "Mod1"
	cfg.Terminal = "foot"
	cfg.Menu = "wofi --show drun"
	cfg.GapsInner = 7
	cfg.BlurEnabled = false
	cfg.Outputs = []Output{{Name: "DP-1", Resolution: "2560x1440", Position: "0,0", Transform: "90"}}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save with missing dirs/files: %v", err)
	}

	main := readFile(t, MainConfigPath())
	mustContain(t, main, "set $mod Mod1")
	mustContain(t, main, "set $term foot")
	mustContain(t, main, "set $menu wofi --show drun")
	mustContain(t, main, "include ~/.config/sway/config.d/*")

	app := readFile(t, AppearancePath())
	mustContain(t, app, "gaps inner 7")
	mustContain(t, app, "blur disable")
	mustContain(t, app, "default_dim_inactive 0.10")

	outs := readFile(t, OutputsPath())
	mustContain(t, outs, "output DP-1 resolution 2560x1440 position 0,0 transform 90")

	// Round trip.
	again, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if again.ModKey != "Mod1" || again.GapsInner != 7 || again.BlurEnabled || len(again.Outputs) != 1 {
		t.Fatalf("round trip mismatch: %+v", again)
	}
}

func TestSaveSeedsMainConfigFromSystemConfig(t *testing.T) {
	setupHome(t)
	sys := "set $mod Mod4\nset $term foot\n\nbindsym $mod+Return exec $term\n"
	if err := os.WriteFile(SystemConfigPath, []byte(sys), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Terminal = "kitty"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	main := readFile(t, MainConfigPath())
	mustContain(t, main, "bindsym $mod+Return exec $term")
	mustContain(t, main, "set $term kitty")
	if strings.Contains(main, "set $term foot") {
		t.Fatalf("old value not replaced:\n%s", main)
	}
	if strings.Count(main, "set $term") != 1 {
		t.Fatalf("expected one $term definition:\n%s", main)
	}
}

func TestSaveAddsMissingSettingsToExistingFiles(t *testing.T) {
	setupHome(t)
	if err := os.MkdirAll(filepath.Dir(AppearancePath()), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(MainConfigPath(), []byte("set $mod Mod4\nbindsym $mod+d exec $menu\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(AppearancePath(), []byte("gaps inner 3\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(OutputsPath(), []byte("output HDMI-A-1 resolution 1920x1080 position 0,0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Menu = "fuzzel"
	cfg.BlurRadius = 12
	cfg.Outputs = append(cfg.Outputs, Output{Name: "DP-2", Resolution: "1920x1080", Position: "1920,0"})
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	main := readFile(t, MainConfigPath())
	mustContain(t, main, "set $menu fuzzel")
	// Variables must be defined before they are used.
	if strings.Index(main, "set $menu") > strings.Index(main, "bindsym $mod+d") {
		t.Fatalf("$menu defined after use:\n%s", main)
	}

	app := readFile(t, AppearancePath())
	mustContain(t, app, "gaps inner 3")
	mustContain(t, app, "blur_radius 12")
	if strings.Count(app, "gaps inner") != 1 {
		t.Fatalf("duplicate gaps inner:\n%s", app)
	}

	outs := readFile(t, OutputsPath())
	mustContain(t, outs, "output HDMI-A-1 resolution 1920x1080 position 0,0")
	mustContain(t, outs, "output DP-2 resolution 1920x1080 position 1920,0")
}

func TestMainConfigPathDefaultsToSwayConfig(t *testing.T) {
	home := setupHome(t)
	want := filepath.Join(home, ".config", "sway", "config")
	if got := MainConfigPath(); got != want {
		t.Fatalf("MainConfigPath() = %q, want %q", got, want)
	}
}

func TestMainConfigPathPrefersLXQtConfig(t *testing.T) {
	home := setupHome(t)
	lxqt := filepath.Join(home, ".config", "lxqt", "wayland", "lxqt-sway.config")
	if err := os.MkdirAll(filepath.Dir(lxqt), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lxqt, []byte("set $mod Mod1\nset $term qterminal\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := MainConfigPath(); got != lxqt {
		t.Fatalf("MainConfigPath() = %q, want %q", got, lxqt)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ModKey != "Mod1" || cfg.Terminal != "qterminal" {
		t.Fatalf("expected values from LXQt config, got mod=%q term=%q", cfg.ModKey, cfg.Terminal)
	}
	cfg.Terminal = "foot"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	mustContain(t, readFile(t, lxqt), "set $term foot")
	if _, err := os.Stat(filepath.Join(home, ".config", "sway", "config")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("sway/config should not be created when LXQt config is used, stat err: %v", err)
	}
}
