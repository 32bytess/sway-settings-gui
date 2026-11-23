package config

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func BehaviorPath() string { return filepath.Join(SwayConfigDir(), "config.d", "behavior.conf") }

var (
	colorsRe  = regexp.MustCompile(`^#[0-9A-Fa-f]{6}([0-9A-Fa-f]{2})?( #[0-9A-Fa-f]{6}([0-9A-Fa-f]{2})?){2,4}$`)
	paddingRe = regexp.MustCompile(`^\d+( \d+)?$`)
)

func validColors(v string) bool  { return colorsRe.MatchString(v) }
func validPadding(v string) bool { return paddingRe.MatchString(v) }
func validFont(v string) bool    { return strings.TrimSpace(v) != "" }
func validNonNegInt(v string) bool {
	n, err := strconv.Atoi(v)
	return err == nil && n >= 0
}

const colorsHint = "#border #background #text [#indicator [#child_border]]"

// LookSpecs and BehaviorSpecs are top-level sway commands managed in
// behavior.conf, in display and write order.
var (
	LookSpecs = []SettingSpec{
		{Key: "font", Label: "Font", Hint: "pango:Noto Sans 10", valid: validFont},
		{Key: "title_align", Label: "Title alignment", Options: []string{"left", "center", "right"}},
		{Key: "titlebar_padding", Label: "Title bar padding", Hint: "horizontal [vertical], e.g. 5 1", valid: validPadding},
		{Key: "titlebar_border_thickness", Label: "Title bar border", Hint: "e.g. 1", valid: validNonNegInt},
		{Key: "hide_edge_borders", Label: "Hide edge borders", Options: []string{"none", "vertical", "horizontal", "both", "smart", "smart_no_gaps"}},
		{Key: "smart_borders", Label: "Smart borders", Options: []string{"on", "no_gaps", "off"}},
		{Key: "smart_gaps", Label: "Smart gaps", Options: []string{"on", "off", "inverse_outer"}},
		{Key: "client.focused", Label: "Focused window colours", Hint: colorsHint, valid: validColors},
		{Key: "client.focused_inactive", Label: "Focused inactive colours", Hint: colorsHint, valid: validColors},
		{Key: "client.unfocused", Label: "Unfocused colours", Hint: colorsHint, valid: validColors},
		{Key: "client.urgent", Label: "Urgent colours", Hint: colorsHint, valid: validColors},
		{Key: "client.placeholder", Label: "Placeholder colours", Hint: colorsHint, valid: validColors},
	}
	BehaviorSpecs = []SettingSpec{
		{Key: "focus_follows_mouse", Label: "Focus follows mouse", Options: []string{"yes", "no", "always"}},
		{Key: "mouse_warping", Label: "Mouse warping", Options: []string{"output", "container", "none"}},
		{Key: "focus_wrapping", Label: "Focus wrapping", Options: []string{"yes", "no", "force", "workspace"}},
		{Key: "focus_on_window_activation", Label: "On window activation", Options: []string{"smart", "urgent", "focus", "none"}},
		{Key: "workspace_layout", Label: "New workspace layout", Options: []string{"default", "stacking", "tabbed"}},
		{Key: "workspace_auto_back_and_forth", Label: "Workspace back and forth", Options: []string{"yes", "no"}},
		{Key: "popup_during_fullscreen", Label: "Popups during fullscreen", Options: []string{"smart", "ignore", "leave_fullscreen"}},
		{Key: "tiling_drag", Label: "Drag tiled windows", Options: []string{"enable", "disable"}},
		{Key: "xwayland", Label: "Xwayland (needs restart)", Options: []string{"enable", "disable", "force"}},
	}
)

func behaviorSpec(key string) (SettingSpec, bool) {
	for _, list := range [][]SettingSpec{LookSpecs, BehaviorSpecs} {
		for _, s := range list {
			if s.Key == key {
				return s, true
			}
		}
	}
	return SettingSpec{}, false
}

type behaviorEntry struct {
	key, value string
	line       int
}

// parseBehaviorEntries finds managed top-level commands, skipping anything
// inside blocks (bar { font ... }, mode "resize" { ... }, input ... { }).
func parseBehaviorEntries(lines []string) []behaviorEntry {
	var entries []behaviorEntry
	depth := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "}") {
			if depth > 0 {
				depth--
			}
			continue
		}
		if strings.HasSuffix(trimmed, "{") {
			depth++
			continue
		}
		if depth > 0 {
			continue
		}
		toks := tokenize(trimmed)
		if len(toks) < 2 {
			continue
		}
		if _, ok := behaviorSpec(toks[0]); ok {
			entries = append(entries, behaviorEntry{key: toks[0], value: joinArgs(toks[1:]), line: i})
		}
	}
	return entries
}

func (c *Config) readBehavior(lines []string) {
	for _, e := range parseBehaviorEntries(lines) {
		c.Behavior[e.key] = e.value
	}
}

func (c *Config) loadBehavior() error {
	data, _, err := readOptional(BehaviorPath())
	if err != nil {
		return err
	}
	c.readBehavior(splitLines(string(data)))
	return nil
}

func (c *Config) validateBehavior() error {
	for _, list := range [][]SettingSpec{LookSpecs, BehaviorSpecs} {
		for _, s := range list {
			if v := c.Behavior[s.Key]; v != "" && !s.Valid(v) {
				return fmt.Errorf("%s: invalid value %q", s.Key, v)
			}
		}
	}
	for name, style := range map[string]string{"border style": c.BorderStyle, "floating border style": c.FloatingBorderStyle} {
		switch style {
		case "", "normal", "pixel", "none":
		default:
			return fmt.Errorf("%s %q must be normal, pixel or none", name, style)
		}
	}
	return nil
}

func copyStringMap(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// saveBehavior updates managed lines in behavior.conf in place (removing
// cleared ones) and appends settings the user changed.
func (c *Config) saveBehavior() error {
	data, _, err := readOptional(BehaviorPath())
	if err != nil {
		return err
	}
	lines := splitLines(string(data))
	seen := map[string]bool{}
	drop := map[int]bool{}
	for _, e := range parseBehaviorEntries(lines) {
		seen[e.key] = true
		if v := c.Behavior[e.key]; v == "" {
			drop[e.line] = true
		} else {
			lines[e.line] = leadingIndent(lines[e.line]) + e.key + " " + v
		}
	}
	var out []string
	for i, l := range lines {
		if !drop[i] {
			out = append(out, l)
		}
	}
	for _, list := range [][]SettingSpec{LookSpecs, BehaviorSpecs} {
		for _, s := range list {
			v := c.Behavior[s.Key]
			if seen[s.Key] || v == "" || v == c.behaviorBaseline[s.Key] {
				continue
			}
			out = append(out, s.Key+" "+v)
		}
	}
	return writeLines(BehaviorPath(), out)
}
