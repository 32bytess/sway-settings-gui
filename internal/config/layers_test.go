package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeLayersConf(t *testing.T, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(LayersPath()), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(LayersPath(), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

const sampleLayers = `# panel effects
layer_effects "dock" {
    blur enable;
    shadows enable;
    corner_radius 8;
}
layer_effects "rofi" {
    blur_xray disable;
}
`

func TestLoadLayerEffects(t *testing.T) {
	setupHome(t)
	writeLayersConf(t, sampleLayers+"layer_effects notifications blur enable\n")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := []LayerEffect{
		{Namespace: "dock", Values: map[string]string{"blur": "enable", "shadows": "enable", "corner_radius": "8"}},
		{Namespace: "rofi", Values: map[string]string{"blur_xray": "disable"}},
		{Namespace: "notifications", Values: map[string]string{"blur": "enable"}},
	}
	if !reflect.DeepEqual(cfg.Layers, want) {
		t.Fatalf("layers =\n%+v\nwant\n%+v", cfg.Layers, want)
	}
}

func TestLayerEffectsRoundTripAndEdit(t *testing.T) {
	setupHome(t)
	writeLayersConf(t, sampleLayers)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, LayersPath()); got != sampleLayers {
		t.Fatalf("round trip changed layers.conf:\n%s", got)
	}

	cfg.Layer("dock").Values["corner_radius"] = "12"
	cfg.Layer("rofi").Values["blur_xray"] = "" // clearing the only setting drops the block
	cfg.Layer("waybar").Values["shadows"] = "disable"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	want := `# panel effects
layer_effects "dock" {
    blur enable;
    shadows enable;
    corner_radius 12;
}
layer_effects "waybar" {
    shadows disable;
}
`
	if got := readFile(t, LayersPath()); got != want {
		t.Fatalf("layers.conf =\n%s\nwant\n%s", got, want)
	}
}

func TestLayerEffectsEmptyByDefault(t *testing.T) {
	setupHome(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Layer("dock") // looking at a namespace without setting anything writes nothing
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, LayersPath()); got != "" {
		t.Fatalf("layers.conf = %q", got)
	}
}

func TestLayerEffectsValidation(t *testing.T) {
	cases := []struct{ ns, key, val string }{
		{"dock", "blur", "yes"},
		{"dock", "corner_radius", "-1"},
		{"dock", "corner_radius", "big"},
		{`dock" {`, "blur", "enable"},
		{"dock", "wobble", "enable"},
	}
	for _, c := range cases {
		setupHome(t)
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		cfg.Layer(c.ns).Values[c.key] = c.val
		if err := cfg.Save(); err == nil {
			t.Errorf("expected error for %q %s %q", c.ns, c.key, c.val)
		}
	}
}

func TestDetectLayerSurfaces(t *testing.T) {
	orig := swayOutputs
	t.Cleanup(func() { swayOutputs = orig })
	swayOutputs = func() ([]byte, error) {
		return []byte(`[{"name":"eDP-1","layer_shell_surfaces":[
			{"namespace":"wallpaper","layer":"background"},
			{"namespace":"dock","layer":"top"},
			{"namespace":"dock","layer":"top"}]}]`), nil
	}
	got, err := DetectLayerSurfaces()
	if err != nil {
		t.Fatal(err)
	}
	want := []LayerSurface{{Namespace: "wallpaper", Layer: "background"}, {Namespace: "dock", Layer: "top"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("surfaces = %+v", got)
	}
	swayOutputs = func() ([]byte, error) { return nil, errors.New("no sway") }
	if _, err := DetectLayerSurfaces(); err == nil {
		t.Fatal("expected error")
	}
}
