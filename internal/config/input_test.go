package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// realKeyboardBlock is copied from the development machine's LXQt config.
const realKeyboardBlock = `    input "type:keyboard"  {
    xkb_layout us
    xkb_options grp:alt_shift_toggle,grp_led:scroll
   }
`

func writeMain(t *testing.T, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(MainConfigPath()), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(MainConfigPath(), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func writeInputConf(t *testing.T, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(InputPath()), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(InputPath(), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadReadsInputBlockFromMainConfig(t *testing.T) {
	setupHome(t)
	writeMain(t, "set $mod Mod4\n"+realKeyboardBlock)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	kb := cfg.Input[Keyboard]
	if kb["xkb_layout"] != "us" || kb["xkb_options"] != "grp:alt_shift_toggle,grp_led:scroll" {
		t.Fatalf("keyboard settings = %v", kb)
	}
}

func TestSaveWithoutInputChangesWritesNoInputLines(t *testing.T) {
	setupHome(t)
	writeMain(t, "set $mod Mod4\n"+realKeyboardBlock)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, InputPath()); strings.Contains(got, "input") {
		t.Fatalf("unchanged input settings must not be written:\n%s", got)
	}
	mustContain(t, readFile(t, MainConfigPath()), realKeyboardBlock)
}

func TestSaveWritesChangedInputSettingAndOverridesMain(t *testing.T) {
	setupHome(t)
	writeMain(t, "set $mod Mod4\n"+realKeyboardBlock)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Input[Keyboard]["xkb_layout"] = "us,de"
	cfg.Input[Keyboard]["repeat_rate"] = "40"
	cfg.Input[Touchpad]["tap"] = "enabled"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	want := "input type:keyboard xkb_layout us,de\ninput type:keyboard repeat_rate 40\ninput type:touchpad tap enabled\n"
	if got := readFile(t, InputPath()); got != want {
		t.Fatalf("input.conf =\n%s\nwant\n%s", got, want)
	}
	mustContain(t, readFile(t, MainConfigPath()), realKeyboardBlock)

	again, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if again.Input[Keyboard]["xkb_layout"] != "us,de" || again.Input[Touchpad]["tap"] != "enabled" {
		t.Fatalf("input.conf should override main config: %v", again.Input)
	}
}

func TestSaveUpdatesInputLineInPlaceAndRemovesCleared(t *testing.T) {
	setupHome(t)
	writeInputConf(t, "# mine\ninput type:touchpad tap enabled\ninput type:touchpad dwt enabled\ninput 1267:12797:Touchpad tap enabled\n")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Input[Touchpad]["tap"] = "disabled"
	cfg.Input[Touchpad]["dwt"] = ""
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	want := "# mine\ninput type:touchpad tap disabled\ninput 1267:12797:Touchpad tap enabled\n"
	if got := readFile(t, InputPath()); got != want {
		t.Fatalf("input.conf =\n%s\nwant\n%s", got, want)
	}
}

func TestSaveRejectsInvalidInputValues(t *testing.T) {
	cases := map[string][3]string{
		"tap yes":            {Touchpad, "tap", "yes"},
		"accel too high":     {Touchpad, "pointer_accel", "2"},
		"accel not number":   {Pointer, "pointer_accel", "fast"},
		"negative rate":      {Keyboard, "repeat_rate", "-1"},
		"layout injection":   {Keyboard, "xkb_layout", "us\nexec evil"},
		"options with space": {Keyboard, "xkb_options", "ctrl:nocaps exec evil"},
		"bad scroll method":  {Touchpad, "scroll_method", "wheel"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			setupHome(t)
			cfg, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			cfg.Input[c[0]][c[1]] = c[2]
			if err := cfg.Save(); err == nil {
				t.Fatalf("expected error for %s %s %q", c[0], c[1], c[2])
			}
		})
	}
}

func TestValidInputValuesAccepted(t *testing.T) {
	setupHome(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Input[Keyboard]["xkb_variant"] = ",nodeadkeys"
	cfg.Input[Keyboard]["xkb_options"] = "grp:alt_shift_toggle,ctrl:nocaps"
	cfg.Input[Keyboard]["repeat_delay"] = "250"
	cfg.Input[Touchpad]["pointer_accel"] = "-0.25"
	cfg.Input[Touchpad]["scroll_factor"] = "0.5"
	cfg.Input[Touchpad]["click_method"] = "clickfinger"
	cfg.Input[Pointer]["accel_profile"] = "flat"
	if err := cfg.Save(); err != nil {
		t.Fatalf("valid input values rejected: %v", err)
	}
}

func TestDetectInputTypes(t *testing.T) {
	orig := swayInputs
	t.Cleanup(func() { swayInputs = orig })

	swayInputs = func() ([]byte, error) {
		return []byte(`[{"type":"keyboard"},{"type":"touchpad"},{"type":"pointer"},{"type":"keyboard"},{"type":"switch"}]`), nil
	}
	got, err := DetectInputTypes()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{Keyboard: true, Touchpad: true, Pointer: true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	swayInputs = func() ([]byte, error) { return nil, errors.New("no socket") }
	if _, err := DetectInputTypes(); err == nil {
		t.Fatal("expected error")
	}
}

func TestDefaultHasUsableInputMaps(t *testing.T) {
	cfg := Default()
	for _, ty := range InputTypes {
		if cfg.Input[ty] == nil {
			t.Fatalf("Default().Input[%s] is nil", ty)
		}
	}
	if cfg.ModKey != "Mod4" || cfg.GapsInner != 4 {
		t.Fatalf("Default() missing defaults: %+v", cfg)
	}
}

func TestParseXkbLayouts(t *testing.T) {
	lst := "! model\n  pc105           Generic 105-key PC\n\n! layout\n  us              English (US)\n  de              German\n\n! variant\n  dvorak          us: English (Dvorak)\n"
	got := parseXkbLayouts(lst)
	want := []string{"us", "de"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
