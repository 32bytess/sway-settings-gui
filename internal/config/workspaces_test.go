package config

import (
	"os"
	"path/filepath"
	"testing"
)

const realWorkspaceMain = `set $mod Mod4
    bindsym $mod+1 workspace number 1
    bindsym $mod+2 workspace number 2
    bindsym $mod+Shift+1 move container to workspace number 1
    bindsym $mod+Shift+2 move container to workspace number 2
workspace 2 output HDMI-A-1
`

func loadWorkspaces(t *testing.T) *Config {
	t.Helper()
	setupHome(t)
	writeMain(t, realWorkspaceMain)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestLoadWorkspaces(t *testing.T) {
	cfg := loadWorkspaces(t)
	if len(cfg.Workspaces) < 10 {
		t.Fatalf("expected workspaces 1-10, got %d", len(cfg.Workspaces))
	}
	ws2 := cfg.Workspace(2)
	if ws2.Outputs != "HDMI-A-1" || ws2.Label != "" {
		t.Fatalf("workspace 2 = %+v", ws2)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, WorkspacesPath()); got != "" {
		t.Fatalf("unchanged workspaces must not be written: %q", got)
	}
	if got := readFile(t, BindingsPath()); got != "" {
		t.Fatalf("bindings.conf = %q", got)
	}
}

func TestWorkspaceNameRewritesBindings(t *testing.T) {
	cfg := loadWorkspaces(t)
	if err := cfg.SetWorkspaceLabel(1, "web"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	want := "bindsym --no-warn $mod+1 workspace number 1:web\nbindsym --no-warn $mod+Shift+1 move container to workspace number 1:web\n"
	if got := readFile(t, BindingsPath()); got != want {
		t.Fatalf("bindings.conf =\n%s\nwant\n%s", got, want)
	}

	again, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if again.Workspace(1).Label != "web" {
		t.Fatalf("label not read back from bindings: %+v", again.Workspace(1))
	}
	if err := again.SetWorkspaceLabel(1, ""); err != nil {
		t.Fatal(err)
	}
	if err := again.Save(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, BindingsPath()); got != "" {
		t.Fatalf("clearing the name should restore the bindings, got %q", got)
	}
}

func TestWorkspaceOutputAndGaps(t *testing.T) {
	cfg := loadWorkspaces(t)
	ws1 := cfg.Workspace(1)
	ws1.Outputs = "eDP-1"
	ws1.GapsInner = "10"
	if err := cfg.SetWorkspaceLabel(1, "web"); err != nil {
		t.Fatal(err)
	}
	cfg.Workspace(2).GapsOuter = "0"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	want := "workspace 1:web output eDP-1\nworkspace 1:web gaps inner 10\nworkspace 2 gaps outer 0\n"
	if got := readFile(t, WorkspacesPath()); got != want {
		t.Fatalf("workspaces.conf =\n%s\nwant\n%s", got, want)
	}
	again, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if w := again.Workspace(1); w.Outputs != "eDP-1" || w.GapsInner != "10" || w.Label != "web" {
		t.Fatalf("reloaded workspace 1 = %+v", w)
	}
	if w := again.Workspace(2); w.Outputs != "HDMI-A-1" || w.GapsOuter != "0" {
		t.Fatalf("reloaded workspace 2 = %+v", w)
	}
}

func TestWorkspacesFileKeepsOtherLines(t *testing.T) {
	setupHome(t)
	if err := os.MkdirAll(filepath.Dir(WorkspacesPath()), 0755); err != nil {
		t.Fatal(err)
	}
	content := "# mine\nworkspace web output DP-1\nworkspace 3 output eDP-1\n"
	if err := os.WriteFile(WorkspacesPath(), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Workspace(3).Outputs != "eDP-1" {
		t.Fatalf("workspace 3 = %+v", cfg.Workspace(3))
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, WorkspacesPath()); got != content {
		t.Fatalf("workspaces.conf = %q, want %q", got, content)
	}
}

func TestWorkspaceValidation(t *testing.T) {
	cfg := loadWorkspaces(t)
	for _, label := range []string{"my web", `we"b`, "a\nb"} {
		if err := cfg.SetWorkspaceLabel(1, label); err == nil {
			t.Errorf("expected error for label %q", label)
		}
	}
	for _, mutate := range []func(*Workspace){
		func(w *Workspace) { w.GapsInner = "-1" },
		func(w *Workspace) { w.GapsOuter = "wide" },
		func(w *Workspace) { w.Outputs = "eDP-1\nexec evil" },
	} {
		c := loadWorkspaces(t)
		mutate(c.Workspace(1))
		if err := c.Save(); err == nil {
			t.Errorf("expected error for %+v", c.Workspace(1))
		}
	}
}
