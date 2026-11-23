package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeBehaviorConf(t *testing.T, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(BehaviorPath()), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(BehaviorPath(), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadReadsTopLevelBehaviorFromMainOnly(t *testing.T) {
	setupHome(t)
	writeMain(t, "focus_on_window_activation focus\nbar {\n    font pango:Sans 9\n}\nmode \"resize\" {\n    bindsym Left resize shrink width 10px\n}\nfont pango:Noto Sans 10\n")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Behavior["focus_on_window_activation"] != "focus" {
		t.Fatalf("focus_on_window_activation = %q", cfg.Behavior["focus_on_window_activation"])
	}
	if cfg.Behavior["font"] != "pango:Noto Sans 10" {
		t.Fatalf("font = %q; the bar block's font must be ignored", cfg.Behavior["font"])
	}
}

func TestSaveWritesOnlyChangedBehavior(t *testing.T) {
	setupHome(t)
	writeMain(t, "focus_on_window_activation focus\n")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, BehaviorPath()); strings.TrimSpace(got) != "" {
		t.Fatalf("unchanged behavior must not be written:\n%s", got)
	}

	cfg.Behavior["focus_follows_mouse"] = "no"
	cfg.Behavior["client.focused"] = "#4c7899 #285577 #ffffff #2e9ef4 #285577"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	want := "client.focused #4c7899 #285577 #ffffff #2e9ef4 #285577\nfocus_follows_mouse no\n"
	if got := readFile(t, BehaviorPath()); got != want {
		t.Fatalf("behavior.conf =\n%s\nwant\n%s", got, want)
	}
	mustContain(t, readFile(t, MainConfigPath()), "focus_on_window_activation focus\n")
}

func TestSaveUpdatesBehaviorInPlaceAndRemovesCleared(t *testing.T) {
	setupHome(t)
	writeBehaviorConf(t, "# mine\nsmart_gaps on\nfocus_follows_mouse no\nfor_window [app_id=foo] floating enable\n")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Behavior["smart_gaps"] = "off"
	cfg.Behavior["focus_follows_mouse"] = ""
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	want := "# mine\nsmart_gaps off\nfor_window [app_id=foo] floating enable\n"
	if got := readFile(t, BehaviorPath()); got != want {
		t.Fatalf("behavior.conf =\n%s\nwant\n%s", got, want)
	}
}

func TestSaveRejectsInvalidBehavior(t *testing.T) {
	cases := map[string][2]string{
		"bad choice":         {"focus_follows_mouse", "sometimes"},
		"too few colours":    {"client.focused", "#ffffff #000000"},
		"not a colour":       {"client.urgent", "#fff red #000"},
		"font newline":       {"font", "pango:Sans 10\nexec evil"},
		"padding not number": {"titlebar_padding", "big"},
		"negative thickness": {"titlebar_border_thickness", "-1"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			setupHome(t)
			cfg, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			cfg.Behavior[c[0]] = c[1]
			if err := cfg.Save(); err == nil {
				t.Fatalf("expected error for %s %q", c[0], c[1])
			}
		})
	}
}

func TestBorderStyleChangeAndKeep(t *testing.T) {
	setupHome(t)
	if err := os.MkdirAll(filepath.Dir(AppearancePath()), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(AppearancePath(), []byte("default_border pixel 2\ndefault_floating_border normal 2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BorderStyle != "pixel" || cfg.FloatingBorderStyle != "normal" {
		t.Fatalf("styles = %q %q", cfg.BorderStyle, cfg.FloatingBorderStyle)
	}
	cfg.BorderStyle = "normal"
	cfg.FloatingBorderStyle = "none"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	app := readFile(t, AppearancePath())
	mustContain(t, app, "default_border normal 2\n")
	mustContain(t, app, "default_floating_border none\n")

	again, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if again.FloatingBorderStyle != "none" {
		t.Fatalf("floating style = %q", again.FloatingBorderStyle)
	}
	again.FloatingBorderStyle = "pixel"
	again.BorderSize = 3
	if err := again.Save(); err != nil {
		t.Fatal(err)
	}
	mustContain(t, readFile(t, AppearancePath()), "default_floating_border pixel 3\n")
}
