package config

import (
	"errors"
	"strings"
	"testing"
)

// realBindingsMain is trimmed from the development machine's LXQt config.
const realBindingsMain = `set $mod Mod4
set $left h
set $term /usr/bin/qterminal
    bindsym $mod+Return exec $term
    bindsym $mod+Shift+e exec swaynag -t warning -m 'You pressed the exit shortcut. Do you really want to exit sway? This will end your Wayland session.' -B 'Yes, exit sway' 'swaymsg exit'
    bindsym $mod+$left focus left
mode "resize" {
    bindsym Left resize shrink width 10px
    bindsym Return mode "default"
}
bindsym $mod+r mode "resize"
    bindsym --locked XF86AudioMute exec pactl set-sink-mute \@DEFAULT_SINK@ toggle
# exec swayidle -w \
#          timeout 300 'swaylock -f -c 000000'
bindsym $mod+x exec foo \
    --bar
`

const exitCmd = `exec swaynag -t warning -m 'You pressed the exit shortcut. Do you really want to exit sway? This will end your Wayland session.' -B 'Yes, exit sway' 'swaymsg exit'`

func loadBindings(t *testing.T) *Config {
	t.Helper()
	setupHome(t)
	writeMain(t, realBindingsMain)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func findBinding(t *testing.T, cfg *Config, combo string) int {
	t.Helper()
	for i, e := range cfg.Bindings {
		if e.Combo == combo {
			return i
		}
	}
	t.Fatalf("binding %q not found", combo)
	return -1
}

func TestLoadParsesDefaultModeBindingsRaw(t *testing.T) {
	cfg := loadBindings(t)
	var combos []string
	for _, e := range cfg.Bindings {
		combos = append(combos, e.Combo)
		if e.Status() != StatusMain {
			t.Fatalf("%s status = %s", e.Combo, e.Status())
		}
	}
	want := "$mod+Return|$mod+Shift+e|$mod+$left|$mod+r|XF86AudioMute|$mod+x"
	if got := strings.Join(combos, "|"); got != want {
		t.Fatalf("combos = %s, want %s (mode block bindings must be skipped)", got, want)
	}
	if c := cfg.Bindings[findBinding(t, cfg, "$mod+Shift+e")].Command; c != exitCmd {
		t.Fatalf("exit command not kept raw:\n%s", c)
	}
	mute := cfg.Bindings[findBinding(t, cfg, "XF86AudioMute")]
	if mute.Command != `exec pactl set-sink-mute \@DEFAULT_SINK@ toggle` || strings.Join(mute.Flags, " ") != "--locked" {
		t.Fatalf("mute binding = %+v", mute.Binding)
	}
	if c := cfg.Bindings[findBinding(t, cfg, "$mod+r")].Command; c != `mode "resize"` {
		t.Fatalf("mode command = %q", c)
	}
	if c := cfg.Bindings[findBinding(t, cfg, "$mod+x")].Command; c != "exec foo --bar" {
		t.Fatalf("continued command = %q", c)
	}
}

func TestSaveWithoutBindingChangesLeavesFileEmpty(t *testing.T) {
	cfg := loadBindings(t)
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, BindingsPath()); got != "" {
		t.Fatalf("bindings.conf should be empty:\n%s", got)
	}
}

func TestOverrideAndRevertMainBinding(t *testing.T) {
	cfg := loadBindings(t)
	i := findBinding(t, cfg, "$mod+Return")
	b := cfg.Bindings[i].Binding
	b.Command = "exec foot"
	if err := cfg.UpdateBinding(i, b); err != nil {
		t.Fatal(err)
	}
	if cfg.Bindings[i].Status() != StatusOverridden {
		t.Fatalf("status = %s", cfg.Bindings[i].Status())
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, BindingsPath()); got != "bindsym --no-warn $mod+Return exec foot\n" {
		t.Fatalf("bindings.conf = %q", got)
	}

	again, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	j := findBinding(t, again, "$mod+Return")
	if again.Bindings[j].Command != "exec foot" || again.Bindings[j].Status() != StatusOverridden {
		t.Fatalf("reloaded override = %+v", again.Bindings[j])
	}
	b = again.Bindings[j].Binding
	b.Command = "exec $term"
	if err := again.UpdateBinding(j, b); err != nil {
		t.Fatal(err)
	}
	if err := again.Save(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, BindingsPath()); got != "" {
		t.Fatalf("reverting should remove the override, got %q", got)
	}
}

func TestRemoveAndRestoreMainBinding(t *testing.T) {
	cfg := loadBindings(t)
	i := findBinding(t, cfg, "XF86AudioMute")
	cfg.RemoveBinding(i)
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, BindingsPath()); got != "unbindsym --locked XF86AudioMute\n" {
		t.Fatalf("bindings.conf = %q", got)
	}
	again, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	j := findBinding(t, again, "XF86AudioMute")
	if again.Bindings[j].Status() != StatusRemoved {
		t.Fatalf("status = %s", again.Bindings[j].Status())
	}
	again.RestoreBinding(j)
	if err := again.Save(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, BindingsPath()); got != "" {
		t.Fatalf("restoring should remove the unbind, got %q", got)
	}
}

func TestAddEditAndDeleteAppBinding(t *testing.T) {
	cfg := loadBindings(t)
	if err := cfg.AddBinding(Binding{Kind: "bindsym", Combo: "$mod+b", Command: "exec firefox"}); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	again, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	i := findBinding(t, again, "$mod+b")
	if again.Bindings[i].Status() != StatusAdded {
		t.Fatalf("status = %s", again.Bindings[i].Status())
	}
	b := again.Bindings[i].Binding
	b.Command = "exec chromium"
	if err := again.UpdateBinding(i, b); err != nil {
		t.Fatal(err)
	}
	if err := again.Save(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, BindingsPath()); got != "bindsym $mod+b exec chromium\n" {
		t.Fatalf("bindings.conf = %q", got)
	}
	again.RemoveBinding(i)
	if err := again.Save(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, BindingsPath()); got != "" {
		t.Fatalf("deleted app binding still written: %q", got)
	}
}

func TestChangingMainComboUnbindsOldAndBindsNew(t *testing.T) {
	cfg := loadBindings(t)
	i := findBinding(t, cfg, "$mod+Shift+e")
	b := cfg.Bindings[i].Binding
	b.Combo = "$mod+Shift+x"
	if err := cfg.UpdateBinding(i, b); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	want := "unbindsym $mod+Shift+e\nbindsym $mod+Shift+x " + exitCmd + "\n"
	if got := readFile(t, BindingsPath()); got != want {
		t.Fatalf("bindings.conf =\n%s\nwant\n%s", got, want)
	}
}

func TestDuplicateDetectionExpandsVarsAndSortsModifiers(t *testing.T) {
	cfg := loadBindings(t)
	for _, combo := range []string{"Mod4+Return", "mod4+return", "$mod+Return"} {
		err := cfg.AddBinding(Binding{Kind: "bindsym", Combo: combo, Command: "exec x"})
		if combo == "mod4+return" {
			// Key names are case-sensitive keysyms: "return" is not "Return".
			if err != nil {
				t.Fatalf("%s should not collide: %v", combo, err)
			}
			continue
		}
		if err == nil {
			t.Fatalf("%s should collide with $mod+Return", combo)
		}
	}
	if err := cfg.AddBinding(Binding{Kind: "bindsym", Combo: "Shift+Mod4+e", Command: "exec x"}); err == nil {
		t.Fatal("Shift+Mod4+e should collide with $mod+Shift+e")
	}
	if err := cfg.AddBinding(Binding{Kind: "bindsym", Flags: []string{"--release"}, Combo: "$mod+Return", Command: "exec x"}); err != nil {
		t.Fatalf("--release binding is distinct: %v", err)
	}
}

func TestBindingValidation(t *testing.T) {
	cfg := loadBindings(t)
	bad := []Binding{
		{Kind: "bindsym", Combo: "$mod+y", Command: ""},
		{Kind: "bindsym", Combo: "$mod+y", Command: "exec a\nexec b"},
		{Kind: "bindsym", Combo: "$mod y", Command: "exec a"},
		{Kind: "bindsym", Flags: []string{"--evil"}, Combo: "$mod+y", Command: "exec a"},
		{Kind: "bindfoo", Combo: "$mod+y", Command: "exec a"},
	}
	for _, b := range bad {
		if err := cfg.AddBinding(b); err == nil {
			t.Errorf("expected error for %+v", b)
		}
	}
}

func TestCheckConfigReportsErrors(t *testing.T) {
	orig := swayCheck
	t.Cleanup(func() { swayCheck = orig })

	swayCheck = func(path string) (string, error) {
		return "00:00:00.1 [ERROR] [sway/config.c:814] Error on line 3 'foo': Unknown/invalid command 'foo'\n00:00:00.1 [INFO] fine\n", nil
	}
	if err := CheckConfig(); err == nil || !strings.Contains(err.Error(), "Unknown/invalid command 'foo'") {
		t.Fatalf("CheckConfig error = %v", err)
	}
	swayCheck = func(path string) (string, error) { return "00:00:00.1 [INFO] ok\n", nil }
	if err := CheckConfig(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	swayCheck = func(path string) (string, error) { return "", errors.New("sway not found") }
	if err := CheckConfig(); err == nil {
		t.Fatal("expected error when sway -C can't run")
	}
}
