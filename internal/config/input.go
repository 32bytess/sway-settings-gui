package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
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

// SettingSpec describes one `input <identifier> <key> <value>` setting.
type SettingSpec struct {
	Key     string
	Label   string
	Options []string // fixed choices; nil for free-form values
	Hint    string   // placeholder for free-form values
	valid   func(string) bool
}

func (s SettingSpec) Valid(v string) bool {
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
var InputSpecs = map[string][]SettingSpec{
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

// allInputSpecs is every input setting once, in keyboard, touchpad, pointer
// order; device identifiers can use any of them.
var allInputSpecs = DeviceSpecs(InputTypes)

// DeviceSpecs lists the settings for a device of the given types, without
// duplicates, in type order.
func DeviceSpecs(types []string) []SettingSpec {
	seen := map[string]bool{}
	var specs []SettingSpec
	for _, t := range types {
		for _, s := range InputSpecs[t] {
			if !seen[s.Key] {
				seen[s.Key] = true
				specs = append(specs, s)
			}
		}
	}
	return specs
}

func isTypeIdent(ident string) bool { return strings.HasPrefix(ident, "type:") }

// inputSpec finds the spec for key: from the type's list for type:
// identifiers, or from all settings for a specific device.
func inputSpec(ident, key string) (SettingSpec, bool) {
	list := allInputSpecs
	if isTypeIdent(ident) {
		list = InputSpecs[ident]
	}
	for _, s := range list {
		if s.Key == key {
			return s, true
		}
	}
	return SettingSpec{}, false
}

var deviceIdentRe = regexp.MustCompile(`^[A-Za-z0-9_:./+-]+$`)

// InputDevice returns the settings map for a device identifier, creating it.
func (c *Config) InputDevice(ident string) map[string]string {
	if c.Input[ident] == nil {
		c.Input[ident] = map[string]string{}
	}
	return c.Input[ident]
}

// deviceIdents lists configured device identifiers (not type:), sorted.
func (c *Config) deviceIdents() []string {
	var ids []string
	for id := range c.Input {
		if !isTypeIdent(id) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
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
		if isTypeIdent(e.ident) && InputSpecs[e.ident] == nil {
			continue // a type the app doesn't manage, e.g. type:tablet_tool
		}
		if _, ok := inputSpec(e.ident, e.key); ok {
			c.InputDevice(e.ident)[e.key] = e.value
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
	for _, id := range c.deviceIdents() {
		used := false
		for _, s := range allInputSpecs {
			v := c.Input[id][s.Key]
			if v == "" {
				continue
			}
			used = true
			if !s.Valid(v) {
				return fmt.Errorf("input %s %s: invalid value %q", id, s.Key, v)
			}
		}
		if used && !deviceIdentRe.MatchString(id) {
			return fmt.Errorf("input identifier %q is not valid", id)
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
		if isTypeIdent(e.ident) && InputSpecs[e.ident] == nil {
			continue
		}
		if _, ok := inputSpec(e.ident, e.key); !ok {
			continue
		}
		seen[identKey{e.ident, e.key}] = true
		if v := c.Input[e.ident][e.key]; v == "" {
			drop[e.line] = true
		} else {
			lines[e.line] = leadingIndent(lines[e.line]) + fmt.Sprintf("input %s %s %s", quoteArg(e.ident), e.key, v)
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
	for _, id := range c.deviceIdents() {
		for _, s := range allInputSpecs {
			v := c.Input[id][s.Key]
			if seen[identKey{id, s.Key}] || v == "" || v == c.inputBaseline[id][s.Key] {
				continue
			}
			out = append(out, fmt.Sprintf("input %s %s %s", quoteArg(id), s.Key, v))
		}
	}
	return writeLines(InputPath(), out)
}

// InputDevice is a connected input device. One identifier can be several
// types (e.g. a keyboard with a built-in pointer).
type InputDevice struct {
	Identifier string
	Name       string
	Types      []string // managed types, e.g. type:keyboard
}

// DetectInputDevices lists connected devices of managed types, merged by
// identifier, in the order sway reports them.
func DetectInputDevices() ([]InputDevice, error) {
	data, err := swayInputs()
	if err != nil {
		return nil, err
	}
	var raw []struct{ Identifier, Name, Type string }
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing swaymsg output: %w", err)
	}
	var devices []InputDevice
	index := map[string]int{}
	for _, r := range raw {
		t := "type:" + r.Type
		if InputSpecs[t] == nil {
			continue
		}
		i, ok := index[r.Identifier]
		if !ok {
			index[r.Identifier] = len(devices)
			devices = append(devices, InputDevice{Identifier: r.Identifier, Name: r.Name})
			i = len(devices) - 1
		}
		found := false
		for _, x := range devices[i].Types {
			found = found || x == t
		}
		if !found {
			devices[i].Types = append(devices[i].Types, t)
		}
	}
	return devices, nil
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
