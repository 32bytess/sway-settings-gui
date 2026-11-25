package config

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

func LayersPath() string { return filepath.Join(SwayConfigDir(), "config.d", "layers.conf") }

// LayerEffect holds SwayFX effects for one layer-shell namespace (e.g. a
// panel). Values maps effect -> value; "" means not set.
type LayerEffect struct {
	Namespace string
	Values    map[string]string
}

// LayerEffectSpecs are the effects SwayFX accepts in layer_effects.
var LayerEffectSpecs = []SettingSpec{
	{Key: "blur", Label: "Blur", Options: []string{"enable", "disable"}},
	{Key: "blur_xray", Label: "Blur x-ray", Options: []string{"enable", "disable"}},
	{Key: "blur_ignore_transparent", Label: "Blur ignores transparent parts", Options: []string{"enable", "disable"}},
	{Key: "shadows", Label: "Shadows", Options: []string{"enable", "disable"}},
	{Key: "corner_radius", Label: "Corner radius (px)", Hint: "e.g. 8", valid: validNonNegInt},
}

var namespaceRe = regexp.MustCompile(`^[A-Za-z0-9_.:-]+$`)

// Layer returns the effects for namespace, adding an empty entry if needed.
func (c *Config) Layer(namespace string) *LayerEffect {
	for i := range c.Layers {
		if c.Layers[i].Namespace == namespace {
			return &c.Layers[i]
		}
	}
	c.Layers = append(c.Layers, LayerEffect{Namespace: namespace, Values: map[string]string{}})
	return &c.Layers[len(c.Layers)-1]
}

func (l LayerEffect) empty() bool {
	for _, v := range l.Values {
		if v != "" {
			return false
		}
	}
	return true
}

var layerHeadRe = regexp.MustCompile(`^layer_effects\s+("[^"]*"|\S+)\s*(.*)$`)

// loadLayers reads layers.conf in block form (layer_effects "ns" { blur
// enable; ... }) or one-line form (layer_effects ns blur enable). Other
// lines are kept as written.
func (c *Config) loadLayers() error {
	data, _, err := readOptional(LayersPath())
	if err != nil {
		return err
	}
	c.layersExtra = nil
	block := ""
	for _, raw := range splitLines(string(data)) {
		line := strings.TrimSpace(raw)
		if block != "" {
			if line == "}" {
				block = ""
				continue
			}
			for _, part := range strings.Split(line, ";") {
				if f := strings.Fields(part); len(f) >= 2 {
					c.Layer(block).Values[f[0]] = strings.Join(f[1:], " ")
				}
			}
			continue
		}
		m := layerHeadRe.FindStringSubmatch(line)
		if m == nil || strings.HasPrefix(line, "#") {
			c.layersExtra = append(c.layersExtra, raw)
			continue
		}
		ns := unquote(m[1])
		rest := strings.TrimSpace(m[2])
		if rest == "{" {
			block = ns
			c.Layer(ns)
			continue
		}
		if f := strings.Fields(strings.TrimSuffix(rest, ";")); len(f) >= 2 {
			c.Layer(ns).Values[f[0]] = strings.Join(f[1:], " ")
			continue
		}
		c.layersExtra = append(c.layersExtra, raw)
	}
	return nil
}

func (c *Config) validateLayers() error {
	for _, l := range c.Layers {
		if l.empty() {
			continue
		}
		if !namespaceRe.MatchString(l.Namespace) {
			return fmt.Errorf("layer namespace %q may only contain letters, digits, _ . : -", l.Namespace)
		}
		for k, v := range l.Values {
			if v == "" {
				continue
			}
			s, ok := specFor(LayerEffectSpecs, k)
			if !ok {
				return fmt.Errorf("layer %s: unknown effect %q", l.Namespace, k)
			}
			if !s.Valid(v) {
				return fmt.Errorf("layer %s: invalid %s value %q", l.Namespace, k, v)
			}
		}
	}
	return nil
}

// saveLayers writes kept lines and one block per namespace with effects.
func (c *Config) saveLayers() error {
	lines := append([]string(nil), c.layersExtra...)
	for _, l := range c.Layers {
		if l.empty() {
			continue
		}
		lines = append(lines, fmt.Sprintf("layer_effects %q {", l.Namespace))
		for _, s := range LayerEffectSpecs {
			if v := l.Values[s.Key]; v != "" {
				lines = append(lines, fmt.Sprintf("    %s %s;", s.Key, v))
			}
		}
		lines = append(lines, "}")
	}
	return writeLines(LayersPath(), lines)
}

// LayerSurface is a layer-shell surface reported by SwayFX.
type LayerSurface struct {
	Namespace string
	Layer     string // background, bottom, top or overlay
}

// SupportsEffects reports whether SwayFX can apply effects to the surface;
// background and bottom layers can't use them.
func (s LayerSurface) SupportsEffects() bool {
	return s.Layer != "background" && s.Layer != "bottom"
}

// DetectLayerSurfaces lists layer-shell surfaces on all outputs, without
// duplicates.
func DetectLayerSurfaces() ([]LayerSurface, error) {
	data, err := swayOutputs()
	if err != nil {
		return nil, err
	}
	var outs []struct {
		LayerShellSurfaces []struct {
			Namespace string `json:"namespace"`
			Layer     string `json:"layer"`
		} `json:"layer_shell_surfaces"`
	}
	if err := json.Unmarshal(data, &outs); err != nil {
		return nil, fmt.Errorf("parsing swaymsg output: %w", err)
	}
	seen := map[LayerSurface]bool{}
	var surfaces []LayerSurface
	for _, o := range outs {
		for _, s := range o.LayerShellSurfaces {
			ls := LayerSurface{Namespace: s.Namespace, Layer: s.Layer}
			if !seen[ls] {
				seen[ls] = true
				surfaces = append(surfaces, ls)
			}
		}
	}
	return surfaces, nil
}
