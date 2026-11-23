package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Input identifiers managed by the app. Settings for specific devices are
// left to the user.
const (
	Keyboard = "type:keyboard"
	Touchpad = "type:touchpad"
	Pointer  = "type:pointer"
)

// InputTypes lists the managed identifiers in display order.
var InputTypes = []string{Keyboard, Touchpad, Pointer}

func InputPath() string { return filepath.Join(SwayConfigDir(), "config.d", "input.conf") }

// InputSpec describes one `input <identifier> <key> <value>` setting.
type InputSpec struct {
	Key     string
	Label   string
	Options []string // fixed choices; nil for free-form values
	Hint    string   // placeholder for free-form values
	valid   func(string) bool
}

func (s InputSpec) Valid(v string) bool {
	if hasControlChars(v) {
		return false
	}
	if s.Options != nil {
		for _, o := range s.Options {
			if v == o {
				return true
			}
		}
		return false
	}
	return s.valid(v)
}

var (
	enabledDisabled = []string{"enabled", "disabled"}
	xkbValueRe      = regexp.MustCompile(`^[A-Za-z0-9_,:()+.-]+$`)
)

func validXkb(v string) bool { return xkbValueRe.MatchString(v) }

func validAccel(v string) bool {
	f, err := strconv.ParseFloat(v, 64)
	return err == nil && f >= -1 && f <= 1
}

func validPositiveFloat(v string) bool {
	f, err := strconv.ParseFloat(v, 64)
	return err == nil && f > 0
}

func validPositiveInt(v string) bool {
	n, err := strconv.Atoi(v)
	return err == nil && n > 0
}

// InputSpecs lists the settings the app manages for each identifier, in the
// order they are shown and written.
var InputSpecs = map[string][]InputSpec{
	Keyboard: {
		{Key: "xkb_layout", Label: "Layout", Hint: "us or us,de", valid: validXkb},
		{Key: "xkb_variant", Label: "Variant", Hint: "e.g. dvorak, or ,nodeadkeys", valid: validXkb},
		{Key: "xkb_options", Label: "Options", Hint: "e.g. ctrl:nocaps,grp:alt_shift_toggle", valid: validXkb},
		{Key: "repeat_delay", Label: "Repeat delay (ms)", Hint: "e.g. 600", valid: validPositiveInt},
		{Key: "repeat_rate", Label: "Repeat rate (per s)", Hint: "e.g. 25", valid: validPositiveInt},
		{Key: "xkb_numlock", Label: "Num Lock at start", Options: enabledDisabled},
		{Key: "xkb_capslock", Label: "Caps Lock at start", Options: enabledDisabled},
	},
	Touchpad: {
		{Key: "tap", Label: "Tap to click", Options: enabledDisabled},
		{Key: "natural_scroll", Label: "Natural scrolling", Options: enabledDisabled},
		{Key: "dwt", Label: "Disable while typing", Options: enabledDisabled},
		{Key: "pointer_accel", Label: "Pointer speed (-1 to 1)", Hint: "e.g. 0.2", valid: validAccel},
		{Key: "accel_profile", Label: "Acceleration profile", Options: []string{"adaptive", "flat"}},
		{Key: "scroll_method", Label: "Scroll method", Options: []string{"two_finger", "edge", "on_button_down", "none"}},
		{Key: "scroll_factor", Label: "Scroll speed factor", Hint: "e.g. 0.5", valid: validPositiveFloat},
		{Key: "click_method", Label: "Click method", Options: []string{"button_areas", "clickfinger", "none"}},
		{Key: "middle_emulation", Label: "Middle-click emulation", Options: enabledDisabled},
		{Key: "left_handed", Label: "Left-handed", Options: enabledDisabled},
	},
	Pointer: {
		{Key: "pointer_accel", Label: "Pointer speed (-1 to 1)", Hint: "e.g. 0", valid: validAccel},
		{Key: "accel_profile", Label: "Acceleration profile", Options: []string{"adaptive", "flat"}},
		{Key: "natural_scroll", Label: "Natural scrolling", Options: enabledDisabled},
		{Key: "scroll_factor", Label: "Scroll speed factor", Hint: "e.g. 1", valid: validPositiveFloat},
		{Key: "middle_emulation", Label: "Middle-click emulation", Options: enabledDisabled},
		{Key: "left_handed", Label: "Left-handed", Options: enabledDisabled},
	},
}

func inputSpec(ident, key string) (InputSpec, bool) {
	for _, s := range InputSpecs[ident] {
		if s.Key == key {
			return s, true
		}
	}
	return InputSpec{}, false
}

func newInputMaps() map[string]map[string]string {
	m := make(map[string]map[string]string, len(InputTypes))
	for _, t := range InputTypes {
		m[t] = map[string]string{}
	}
	return m
}

func copyInputMaps(src map[string]map[string]string) map[string]map[string]string {
	dst := newInputMaps()
	for t, kv := range src {
		if dst[t] == nil {
			dst[t] = map[string]string{}
		}
		for k, v := range kv {
			dst[t][k] = v
		}
	}
	return dst
}

// inputEntry is one setting found in a config file.
type inputEntry struct {
	ident, key, value string
	line              int  // index into the file's lines
	single            bool // `input <ident> <key> <value>` on one line
}

// parseInputEntries finds input settings in both single-line and block form.
func parseInputEntries(lines []string) []inputEntry {
	var entries []inputEntry
	block := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		toks := tokenize(trimmed)
		if block != "" {
			if strings.HasPrefix(trimmed, "}") {
				block = ""
				continue
			}
			if len(toks) >= 2 {
				entries = append(entries, inputEntry{ident: block, key: toks[0], value: joinArgs(toks[1:]), line: i})
			}
			continue
		}
		if len(toks) < 3 || toks[0] != "input" {
			continue
		}
		ident := unquote(toks[1])
		if toks[2] == "{" {
			block = ident
			continue
		}
		entries = append(entries, inputEntry{ident: ident, key: toks[2], value: joinArgs(toks[3:]), line: i, single: true})
	}
	return entries
}

// readInputs copies managed settings from a config file's lines into c.Input.
func (c *Config) readInputs(lines []string) {
	for _, e := range parseInputEntries(lines) {
		if _, ok := inputSpec(e.ident, e.key); ok {
			c.Input[e.ident][e.key] = e.value
		}
	}
}

func (c *Config) loadInputs() error {
	data, _, err := readOptional(InputPath())
	if err != nil {
		return err
	}
	c.readInputs(splitLines(string(data)))
	return nil
}

func (c *Config) validateInputs() error {
	for _, t := range InputTypes {
		for _, s := range InputSpecs[t] {
			v := c.Input[t][s.Key]
			if v != "" && !s.Valid(v) {
				return fmt.Errorf("%s %s: invalid value %q", t, s.Key, v)
			}
		}
	}
	return nil
}

// saveInputs updates single-line settings in input.conf in place (removing
// ones that were cleared) and appends settings the user changed. Blocks and
// device-specific lines are left untouched.
func (c *Config) saveInputs() error {
	data, _, err := readOptional(InputPath())
	if err != nil {
		return err
	}
	lines := splitLines(string(data))
	type identKey struct{ ident, key string }
	seen := make(map[identKey]bool)
	drop := make(map[int]bool)
	for _, e := range parseInputEntries(lines) {
		if !e.single {
			continue
		}
		if _, ok := inputSpec(e.ident, e.key); !ok {
			continue
		}
		seen[identKey{e.ident, e.key}] = true
		if v := c.Input[e.ident][e.key]; v == "" {
			drop[e.line] = true
		} else {
			lines[e.line] = leadingIndent(lines[e.line]) + fmt.Sprintf("input %s %s %s", e.ident, e.key, v)
		}
	}
	var out []string
	for i, l := range lines {
		if !drop[i] {
			out = append(out, l)
		}
	}
	for _, t := range InputTypes {
		for _, s := range InputSpecs[t] {
			v := c.Input[t][s.Key]
			if seen[identKey{t, s.Key}] || v == "" || v == c.inputBaseline[t][s.Key] {
				continue
			}
			out = append(out, fmt.Sprintf("input %s %s %s", t, s.Key, v))
		}
	}
	return writeLines(InputPath(), out)
}

// swayInputs returns the raw JSON of `swaymsg -t get_inputs`. A variable so
// tests can replace it.
var swayInputs = func() ([]byte, error) {
	out, err := exec.Command("swaymsg", "-t", "get_inputs", "--raw").Output()
	if err != nil {
		return nil, fmt.Errorf("swaymsg -t get_inputs: %w", err)
	}
	return out, nil
}

// DetectInputTypes reports which managed device types are connected.
func DetectInputTypes() (map[string]bool, error) {
	data, err := swayInputs()
	if err != nil {
		return nil, err
	}
	var raw []struct{ Type string }
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing swaymsg output: %w", err)
	}
	found := map[string]bool{}
	for _, r := range raw {
		ident := "type:" + r.Type
		if _, managed := InputSpecs[ident]; managed {
			found[ident] = true
		}
	}
	return found, nil
}

// XkbRulesList is the xkb rules listing that names the available layouts.
var XkbRulesList = "/usr/share/X11/xkb/rules/evdev.lst"

// XkbLayouts returns the layout codes (us, de, ...) known to xkb.
func XkbLayouts() ([]string, error) {
	data, err := os.ReadFile(XkbRulesList)
	if err != nil {
		return nil, err
	}
	return parseXkbLayouts(string(data)), nil
}

func parseXkbLayouts(lst string) []string {
	var layouts []string
	inLayouts := false
	for _, line := range strings.Split(lst, "\n") {
		if strings.HasPrefix(line, "!") {
			inLayouts = strings.TrimSpace(strings.TrimPrefix(line, "!")) == "layout"
			continue
		}
		if f := strings.Fields(line); inLayouts && len(f) > 0 {
			layouts = append(layouts, f[0])
		}
	}
	return layouts
}
