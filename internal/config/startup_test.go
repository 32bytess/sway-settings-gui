package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const realStartupMain = `set $mod Mod4
    exec dbus-update-activation-environment --systemd DISPLAY WAYLAND_DISPLAY
    exec lxqt-session && sway exit
    bindsym $mod+Return exec $term
for_window [app_id="x"] exec notify-send hi
bar {
    status_command exec i3status
}
exec_always --no-startup-id kanshi
`

func loadStartup(t *testing.T) *Config {
	t.Helper()
	setupHome(t)
	writeMain(t, realStartupMain)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestLoadStartupFromMainReadOnly(t *testing.T) {
	cfg := loadStartup(t)
	var got []string
	for _, s := range cfg.Startup {
		if s.Editable() {
			t.Fatalf("main-config exec %+v must be read-only", s)
		}
		got = append(got, s.Kind+" "+s.Command)
	}
	want := []string{
		"exec dbus-update-activation-environment --systemd DISPLAY WAYLAND_DISPLAY",
		"exec lxqt-session && sway exit",
		"exec_always --no-startup-id kanshi",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("startup = %q, want %q", got, want)
	}
}

func TestStartupAddEditDelete(t *testing.T) {
	cfg := loadStartup(t)
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, StartupPath()); got != "" {
		t.Fatalf("startup.conf should be empty, got %q", got)
	}
	if err := cfg.AddStartup(StartupEntry{Kind: "exec", Command: "nm-applet --indicator"}); err != nil {
		t.Fatal(err)
	}
	if err := cfg.AddStartup(StartupEntry{Kind: "exec_always", Command: `swaybg -i "/a b.png"`}); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	want := "exec nm-applet --indicator\nexec_always swaybg -i \"/a b.png\"\n"
	if got := readFile(t, StartupPath()); got != want {
		t.Fatalf("startup.conf =\n%s\nwant\n%s", got, want)
	}

	again, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	n := len(again.Startup)
	if !again.Startup[n-2].Editable() || again.Startup[n-1].Command != `swaybg -i "/a b.png"` {
		t.Fatalf("reloaded startup = %+v", again.Startup)
	}
	if err := again.UpdateStartup(n-2, StartupEntry{Kind: "exec_always", Command: "nm-applet"}); err != nil {
		t.Fatal(err)
	}
	if err := again.DeleteStartup(n - 1); err != nil {
		t.Fatal(err)
	}
	if err := again.Save(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, StartupPath()); got != "exec_always nm-applet\n" {
		t.Fatalf("startup.conf = %q", got)
	}
	if err := again.UpdateStartup(0, StartupEntry{Kind: "exec", Command: "x"}); err == nil {
		t.Fatal("expected error editing a main-config exec")
	}
	if err := again.DeleteStartup(0); err == nil {
		t.Fatal("expected error deleting a main-config exec")
	}
}

func TestStartupFileKeepsOtherLines(t *testing.T) {
	setupHome(t)
	if err := os.MkdirAll(filepath.Dir(StartupPath()), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(StartupPath(), []byte("# autostart\nexec mako\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, StartupPath()); got != "# autostart\nexec mako\n" {
		t.Fatalf("startup.conf = %q", got)
	}
}

func TestStartupValidation(t *testing.T) {
	cfg := loadStartup(t)
	for _, s := range []StartupEntry{
		{Kind: "exec", Command: ""},
		{Kind: "exec", Command: "a\nexec b"},
		{Kind: "run", Command: "mako"},
	} {
		if err := cfg.AddStartup(s); err == nil {
			t.Errorf("expected error for %+v", s)
		}
	}
}

func TestRunNowUsesSwaymsgExec(t *testing.T) {
	orig := runSwaymsg
	t.Cleanup(func() { runSwaymsg = orig })
	var got []string
	runSwaymsg = func(args ...string) error { got = args; return nil }
	if err := RunNow(`swaybg -i "/a b.png"`); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"exec", `swaybg -i "/a b.png"`}) {
		t.Fatalf("swaymsg args = %q", got)
	}
	runSwaymsg = func(args ...string) error { return errors.New("no sway") }
	if err := RunNow("mako"); err == nil {
		t.Fatal("expected error")
	}
	if err := RunNow("a\nb"); err == nil {
		t.Fatal("expected error for control characters")
	}
}
