package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// realOutputsJSON is trimmed from `swaymsg -t get_outputs --raw` on the
// development laptop.
const realOutputsJSON = `[
  {
    "name": "eDP-1", "make": "AU Optronics", "model": "0x7DB2", "active": true,
    "scale": 1.5, "transform": "normal", "adaptive_sync_status": "disabled",
    "rect": {"x": 0, "y": 0, "width": 1706, "height": 1066},
    "current_mode": {"width": 2560, "height": 1600, "refresh": 240001},
    "modes": [
      {"width": 2560, "height": 1600, "refresh": 60002},
      {"width": 2560, "height": 1600, "refresh": 240001}
    ]
  }
]`

func writeOutputsConf(t *testing.T, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(OutputsPath()), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(OutputsPath(), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func stubOutputs(t *testing.T, json string, err error) {
	t.Helper()
	orig := swayOutputs
	swayOutputs = func() ([]byte, error) { return []byte(json), err }
	t.Cleanup(func() { swayOutputs = orig })
}

func findOutput(t *testing.T, cfg *Config, name string) *Output {
	t.Helper()
	for i := range cfg.Outputs {
		if cfg.Outputs[i].Name == name {
			return &cfg.Outputs[i]
		}
	}
	t.Fatalf("output %q not found in %+v", name, cfg.Outputs)
	return nil
}

func TestLoadReadsSingleSettingOutputLine(t *testing.T) {
	setupHome(t)
	writeOutputsConf(t, "output eDP-1 scale 1.5\n")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if out := findOutput(t, cfg, "eDP-1"); out.Scale != "1.5" {
		t.Fatalf("Scale = %q, want 1.5", out.Scale)
	}
}

func TestSaveUpdatesScaleLineInPlace(t *testing.T) {
	setupHome(t)
	writeOutputsConf(t, "output eDP-1 scale 1.5\n")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	findOutput(t, cfg, "eDP-1").Scale = "2"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, OutputsPath()); got != "output eDP-1 scale 2\n" {
		t.Fatalf("outputs.conf = %q", got)
	}
}

func TestSaveLeavesUnmanagedOutputSettingsAlone(t *testing.T) {
	setupHome(t)
	writeOutputsConf(t, "# my outputs\noutput eDP-1 scale 1.5 subpixel rgb\noutput HDMI-A-1 color_profile icc /x.icc\n")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	findOutput(t, cfg, "eDP-1").Scale = "1.25"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	want := "# my outputs\noutput eDP-1 scale 1.25 subpixel rgb\noutput HDMI-A-1 color_profile icc /x.icc\n"
	if got := readFile(t, OutputsPath()); got != want {
		t.Fatalf("outputs.conf =\n%s\nwant\n%s", got, want)
	}
}

func TestSaveAddsMissingSettingForExistingOutput(t *testing.T) {
	setupHome(t)
	writeOutputsConf(t, "output eDP-1 scale 1.5\n")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	findOutput(t, cfg, "eDP-1").AdaptiveSync = "on"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	want := "output eDP-1 scale 1.5\noutput eDP-1 adaptive_sync on\n"
	if got := readFile(t, OutputsPath()); got != want {
		t.Fatalf("outputs.conf = %q, want %q", got, want)
	}
}

func TestDetectOutputsParsesModes(t *testing.T) {
	stubOutputs(t, realOutputsJSON, nil)
	outs, err := DetectOutputs()
	if err != nil {
		t.Fatal(err)
	}
	if len(outs) != 1 {
		t.Fatalf("got %d outputs", len(outs))
	}
	o := outs[0]
	if o.Name != "eDP-1" || o.CurrentMode != "2560x1600@240.001Hz" || o.Scale != 1.5 || o.Transform != "normal" {
		t.Fatalf("unexpected detected output: %+v", o)
	}
	if len(o.Modes) != 2 || o.Modes[0] != "2560x1600@60.002Hz" || o.Modes[1] != "2560x1600@240.001Hz" {
		t.Fatalf("unexpected modes: %v", o.Modes)
	}
}

func TestDetectOutputsSurfacesErrors(t *testing.T) {
	stubOutputs(t, "", errors.New("no sway socket"))
	if _, err := DetectOutputs(); err == nil {
		t.Fatal("expected error from failing swaymsg")
	}
	stubOutputs(t, "not json", nil)
	if _, err := DetectOutputs(); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestDetectedOutputsAreNotPinnedOnSave(t *testing.T) {
	setupHome(t)
	stubOutputs(t, realOutputsJSON, nil)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	detected, err := DetectOutputs()
	if err != nil {
		t.Fatal(err)
	}
	cfg.MergeDetected(detected)
	out := findOutput(t, cfg, "eDP-1")
	if out.Detected == nil || out.Detected.CurrentMode != "2560x1600@240.001Hz" {
		t.Fatalf("detected info not attached: %+v", out)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, OutputsPath()); strings.Contains(got, "eDP-1") {
		t.Fatalf("untouched detected output must not be written:\n%s", got)
	}
}

func TestMergeDetectedKeepsConfiguredOutputs(t *testing.T) {
	setupHome(t)
	writeOutputsConf(t, "output HDMI-A-1 scale 1\n")
	stubOutputs(t, realOutputsJSON, nil)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	detected, err := DetectOutputs()
	if err != nil {
		t.Fatal(err)
	}
	cfg.MergeDetected(detected)
	if h := findOutput(t, cfg, "HDMI-A-1"); h.Detected != nil || h.Scale != "1" {
		t.Fatalf("configured-but-disconnected output wrong: %+v", h)
	}
	findOutput(t, cfg, "eDP-1")
}

func TestNewOutputWrittenAsOneLineWithMode(t *testing.T) {
	setupHome(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Outputs = append(cfg.Outputs, Output{Name: "eDP-1", Resolution: "2560x1600@240.001Hz", Scale: "1.5"})
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, OutputsPath()); got != "output eDP-1 resolution 2560x1600@240.001Hz scale 1.5\n" {
		t.Fatalf("outputs.conf = %q", got)
	}
}

func TestWallpaperFromMainConfigNotDuplicated(t *testing.T) {
	setupHome(t)
	if err := os.MkdirAll(filepath.Dir(MainConfigPath()), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(MainConfigPath(), []byte("output * bg /usr/share/waves.png fill\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Wallpaper != "/usr/share/waves.png" || cfg.WallpaperMode != "fill" {
		t.Fatalf("wallpaper = %q %q", cfg.Wallpaper, cfg.WallpaperMode)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, OutputsPath()); strings.Contains(got, "bg") {
		t.Fatalf("unchanged wallpaper must not be copied to outputs.conf:\n%s", got)
	}

	cfg.Wallpaper = "/home/me/My Pictures/sky.jpg"
	cfg.WallpaperMode = "stretch"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	mustContain(t, readFile(t, OutputsPath()), `output * bg "/home/me/My Pictures/sky.jpg" stretch`+"\n")

	again, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if again.Wallpaper != "/home/me/My Pictures/sky.jpg" || again.WallpaperMode != "stretch" {
		t.Fatalf("outputs.conf wallpaper should override main: %q %q", again.Wallpaper, again.WallpaperMode)
	}
}

func TestWallpaperUpdatedInPlaceKeepsFallbackColor(t *testing.T) {
	setupHome(t)
	writeOutputsConf(t, "output * bg /a.png fill #000000\n")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.WallpaperMode = "fit"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, OutputsPath()); got != "output * bg /a.png fit #000000\n" {
		t.Fatalf("outputs.conf = %q", got)
	}
}

func TestSaveRejectsInvalidWallpaperMode(t *testing.T) {
	setupHome(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Wallpaper = "/a.png"
	cfg.WallpaperMode = "zoom"
	if err := cfg.Save(); err == nil {
		t.Fatal("expected error for invalid wallpaper mode")
	}
}

func TestSaveRejectsInvalidScale(t *testing.T) {
	for _, bad := range []string{"abc", "0", "-1"} {
		setupHome(t)
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		cfg.Outputs = []Output{{Name: "eDP-1", Scale: bad}}
		if err := cfg.Save(); err == nil {
			t.Fatalf("expected error for scale %q", bad)
		}
	}
}

func TestSaveRejectsControlCharactersAndBadFormats(t *testing.T) {
	cases := map[string]func(*Config){
		"newline in wallpaper": func(c *Config) { c.Wallpaper = "/a.png\nexec rm -rf ~"; c.WallpaperMode = "fill" },
		"newline in terminal":  func(c *Config) { c.Terminal = "foot\nexec evil" },
		"newline in menu":      func(c *Config) { c.Menu = "wofi\rexec evil" },
		"NUL in scale":         func(c *Config) { c.Outputs = []Output{{Name: "eDP-1", Scale: "1\x00"}} },
		"bad resolution":       func(c *Config) { c.Outputs = []Output{{Name: "eDP-1", Resolution: "big exec evil"}} },
		"bad position":         func(c *Config) { c.Outputs = []Output{{Name: "eDP-1", Position: "0,0 exec evil"}} },
		"bad transform":        func(c *Config) { c.Outputs = []Output{{Name: "eDP-1", Transform: "sideways"}} },
		"bad adaptive sync":    func(c *Config) { c.Outputs = []Output{{Name: "eDP-1", AdaptiveSync: "maybe"}} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			setupHome(t)
			cfg, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			mutate(cfg)
			if err := cfg.Save(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestValidOutputFormatsAccepted(t *testing.T) {
	setupHome(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Outputs = []Output{
		{Name: "eDP-1", Resolution: "2560x1600@240.001Hz", Position: "0,0", Transform: "90", AdaptiveSync: "on"},
		{Name: "DP-1", Resolution: "--custom 1920x1080@60Hz", Position: "-1920 0", Transform: "flipped-270 clockwise"},
		{Name: "DP-2", Resolution: "1920x1080"},
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("valid outputs rejected: %v", err)
	}
}
