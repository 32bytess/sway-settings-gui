package config

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

func BarPath() string { return filepath.Join(SwayConfigDir(), "config.d", "bar.conf") }

// BarID names the bar this app manages, so it can't clash with others.
const BarID = "sway-settings"

// Bar is the swaybar managed in bar.conf. While disabled, bar.conf is empty.
type Bar struct {
	Enabled  bool
	Settings map[string]string // bar subcommand -> value; "" = not set
	Colors   map[string]string // colors { } entry -> value
	// extra holds lines inside the bar block the app doesn't manage.
	extra []string
	// OtherBars counts bar blocks in other config files.
	OtherBars int
}

var (
	barGapsRe   = regexp.MustCompile(`^\d+( \d+){0,3}$`)
	yesNo       = []string{"yes", "no"}
	color3Hint  = "#border #background #text"
	singleColor = func(v string) bool { return ValidHexColor(v) }
	threeColors = regexp.MustCompile(`^#[0-9A-Fa-f]{6}([0-9A-Fa-f]{2})?( #[0-9A-Fa-f]{6}([0-9A-Fa-f]{2})?){2}$`)
)

func validFreeText(v string) bool { return strings.TrimSpace(v) != "" }

// BarSpecs are the bar settings, in display and write order.
var BarSpecs = []SettingSpec{
	{Key: "position", Label: "Position", Options: []string{"top", "bottom"}},
	{Key: "mode", Label: "Mode", Options: []string{"dock", "hide", "invisible", "overlay"}},
	{Key: "hidden_state", Label: "Hidden state", Options: []string{"hide", "show"}},
	{Key: "modifier", Label: "Show-bar modifier", Hint: "e.g. $mod or none", valid: validXkb},
	{Key: "height", Label: "Height (px, 0 = auto)", Hint: "e.g. 24", valid: validNonNegInt},
	{Key: "gaps", Label: "Gaps (px)", Hint: "all, or top/bottom left/right, ...", valid: barGapsRe.MatchString},
	{Key: "font", Label: "Font", Hint: "pango:Noto Sans 10", valid: validFont},
	{Key: "status_command", Label: "Status command", Hint: "while date +'%Y-%m-%d %X'; do sleep 1; done", valid: validFreeText},
	{Key: "status_padding", Label: "Status padding (px)", Hint: "e.g. 1", valid: validNonNegInt},
	{Key: "separator_symbol", Label: "Separator symbol", Hint: "e.g. |", valid: validFreeText},
	{Key: "workspace_buttons", Label: "Workspace buttons", Options: yesNo},
	{Key: "strip_workspace_numbers", Label: "Strip workspace numbers", Options: yesNo},
	{Key: "binding_mode_indicator", Label: "Binding mode indicator", Options: yesNo},
	{Key: "pango_markup", Label: "Pango markup", Options: []string{"enabled", "disabled"}},
	{Key: "tray_output", Label: "Tray output", Hint: "none, *, or an output name", valid: validXkb},
	{Key: "tray_padding", Label: "Tray padding (px)", Hint: "e.g. 2", valid: validNonNegInt},
	{Key: "icon_theme", Label: "Tray icon theme", Hint: "e.g. Papirus", valid: validFreeText},
}

// BarColorSpecs are the colors { } entries. Single colours have no Hint;
// workspace/binding-mode entries take border, background and text.
var BarColorSpecs = []SettingSpec{
	{Key: "background", Label: "Background", valid: singleColor},
	{Key: "statusline", Label: "Status text", valid: singleColor},
	{Key: "separator", Label: "Separator", valid: singleColor},
	{Key: "focused_workspace", Label: "Focused workspace", Hint: color3Hint, valid: threeColors.MatchString},
	{Key: "active_workspace", Label: "Active workspace", Hint: color3Hint, valid: threeColors.MatchString},
	{Key: "inactive_workspace", Label: "Inactive workspace", Hint: color3Hint, valid: threeColors.MatchString},
	{Key: "urgent_workspace", Label: "Urgent workspace", Hint: color3Hint, valid: threeColors.MatchString},
	{Key: "binding_mode", Label: "Binding mode", Hint: color3Hint, valid: threeColors.MatchString},
}

func specFor(specs []SettingSpec, key string) (SettingSpec, bool) {
	for _, s := range specs {
		if s.Key == key {
			return s, true
		}
	}
	return SettingSpec{}, false
}

func newBar() Bar {
	return Bar{Settings: map[string]string{}, Colors: map[string]string{}}
}

// countBars counts top-level bar blocks in a file's lines.
func countBars(lines []string) int {
	n := 0
	depth := 0
	logical, _ := logicalLines(lines)
	for _, line := range logical {
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case strings.HasPrefix(line, "}"):
			if depth > 0 {
				depth--
			}
		case strings.HasSuffix(line, "{"):
			if depth == 0 && (line == "bar {" || strings.HasPrefix(line, "bar ")) {
				n++
			}
			depth++
		}
	}
	return n
}

// loadBar reads the managed bar from bar.conf and counts bars elsewhere.
func (c *Config) loadBar() error {
	files, err := filepath.Glob(filepath.Join(SwayConfigDir(), "config.d", "*"))
	if err != nil {
		return err
	}
	for _, f := range files {
		if f == BarPath() {
			continue
		}
		data, _, err := readOptional(f)
		if err != nil {
			return err
		}
		c.Bar.OtherBars += countBars(splitLines(string(data)))
	}

	data, _, err := readOptional(BarPath())
	if err != nil {
		return err
	}
	depth := 0
	inColors := false
	for _, raw := range splitLines(string(data)) {
		line := strings.TrimSpace(raw)
		switch {
		case depth == 0 && strings.HasPrefix(line, "bar") && strings.HasSuffix(line, "{"):
			c.Bar.Enabled = true
			depth = 1
		case depth == 0:
			// Lines outside the bar block are not kept.
		case line == "}":
			if inColors {
				inColors = false
			} else {
				depth = 0
			}
		case line == "colors {":
			inColors = true
		default:
			key, val, _ := strings.Cut(line, " ")
			val = strings.TrimSpace(val)
			if inColors {
				if _, ok := specFor(BarColorSpecs, key); ok {
					c.Bar.Colors[key] = val
					continue
				}
			} else if key == "id" {
				continue
			} else if _, ok := specFor(BarSpecs, key); ok {
				c.Bar.Settings[key] = val
				continue
			}
			if line != "" {
				c.Bar.extra = append(c.Bar.extra, line)
			}
		}
	}
	return nil
}

func (c *Config) validateBar() error {
	if !c.Bar.Enabled {
		return nil
	}
	for _, s := range BarSpecs {
		if v := c.Bar.Settings[s.Key]; v != "" && !s.Valid(v) {
			return fmt.Errorf("bar %s: invalid value %q", s.Key, v)
		}
	}
	for _, s := range BarColorSpecs {
		if v := c.Bar.Colors[s.Key]; v != "" && !s.Valid(v) {
			return fmt.Errorf("bar colour %s: invalid value %q", s.Key, v)
		}
	}
	for _, l := range c.Bar.extra {
		if hasControlChars(l) {
			return fmt.Errorf("bar contains control characters")
		}
	}
	return nil
}

// saveBar writes the whole bar block, or an empty file while disabled.
func (c *Config) saveBar() error {
	if !c.Bar.Enabled {
		return writeLines(BarPath(), nil)
	}
	lines := []string{"bar {", "    id " + BarID}
	for _, s := range BarSpecs {
		if v := c.Bar.Settings[s.Key]; v != "" {
			lines = append(lines, "    "+s.Key+" "+v)
		}
	}
	for _, l := range c.Bar.extra {
		lines = append(lines, "    "+l)
	}
	var colors []string
	for _, s := range BarColorSpecs {
		if v := c.Bar.Colors[s.Key]; v != "" {
			colors = append(colors, "        "+s.Key+" "+v)
		}
	}
	if len(colors) > 0 {
		lines = append(lines, "    colors {")
		lines = append(lines, colors...)
		lines = append(lines, "    }")
	}
	lines = append(lines, "}")
	return writeLines(BarPath(), lines)
}
