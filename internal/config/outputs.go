package config

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Output holds the per-display settings the app manages. An empty field means
// "not managed": it is neither written nor removed unless a line for it
// already exists in outputs.conf.
type Output struct {
	Name         string
	Resolution   string // sway mode, e.g. "2560x1600@240.001Hz"
	Position     string // "X,Y"
	Scale        string
	Transform    string
	AdaptiveSync string // "on" or "off"

	// Detected is the live state from sway, or nil when the output is only
	// in the config (e.g. a disconnected monitor).
	Detected *DetectedOutput
}

// DetectedOutput is a connected display as reported by sway.
type DetectedOutput struct {
	Name         string
	Make         string
	Model        string
	Active       bool
	CurrentMode  string
	Modes        []string
	Scale        float64
	Transform    string
	AdaptiveSync string
}

// WallpaperModes are the modes accepted by `output * bg <file> <mode>`.
var WallpaperModes = []string{"fill", "fit", "stretch", "center", "tile", "solid_color"}

// outputFieldOrder is the order settings are written in for a new output.
var outputFieldOrder = []string{"resolution", "position", "scale", "transform", "adaptive_sync"}

func (o *Output) field(key string) *string {
	switch key {
	case "resolution":
		return &o.Resolution
	case "position":
		return &o.Position
	case "scale":
		return &o.Scale
	case "transform":
		return &o.Transform
	case "adaptive_sync":
		return &o.AdaptiveSync
	}
	return nil
}

// swayOutputs returns the raw JSON of `swaymsg -t get_outputs`. A variable
// so tests can replace it.
var swayOutputs = func() ([]byte, error) {
	out, err := exec.Command("swaymsg", "-t", "get_outputs", "--raw").Output()
	if err != nil {
		return nil, fmt.Errorf("swaymsg -t get_outputs: %w", err)
	}
	return out, nil
}

func formatMode(width, height, refreshMHz int) string {
	if refreshMHz <= 0 {
		return fmt.Sprintf("%dx%d", width, height)
	}
	hz := strconv.FormatFloat(float64(refreshMHz)/1000, 'f', -1, 64)
	return fmt.Sprintf("%dx%d@%sHz", width, height, hz)
}

// DetectOutputs asks the running sway for its connected displays.
func DetectOutputs() ([]DetectedOutput, error) {
	data, err := swayOutputs()
	if err != nil {
		return nil, err
	}
	type mode struct{ Width, Height, Refresh int }
	var raw []struct {
		Name               string
		Make               string
		Model              string
		Active             bool
		Scale              float64
		Transform          string
		AdaptiveSyncStatus string `json:"adaptive_sync_status"`
		CurrentMode        mode   `json:"current_mode"`
		Modes              []mode
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing swaymsg output: %w", err)
	}
	outs := make([]DetectedOutput, 0, len(raw))
	for _, r := range raw {
		d := DetectedOutput{
			Name: r.Name, Make: r.Make, Model: r.Model, Active: r.Active,
			Scale: r.Scale, Transform: r.Transform, AdaptiveSync: r.AdaptiveSyncStatus,
			CurrentMode: formatMode(r.CurrentMode.Width, r.CurrentMode.Height, r.CurrentMode.Refresh),
		}
		for _, m := range r.Modes {
			d.Modes = append(d.Modes, formatMode(m.Width, m.Height, m.Refresh))
		}
		outs = append(outs, d)
	}
	return outs, nil
}

// MergeDetected attaches live state to configured outputs and adds connected
// outputs that have no config yet (with no managed fields set).
func (c *Config) MergeDetected(detected []DetectedOutput) {
	for i := range detected {
		d := detected[i]
		found := false
		for j := range c.Outputs {
			if c.Outputs[j].Name == d.Name {
				c.Outputs[j].Detected = &d
				found = true
			}
		}
		if !found {
			c.Outputs = append(c.Outputs, Output{Name: d.Name, Detected: &d})
		}
	}
}

// tokenize splits a config line on whitespace, keeping quoted strings
// (single or double) together. Tokens are returned as written.
func tokenize(line string) []string {
	var toks []string
	var cur strings.Builder
	var quote rune
	for _, r := range line {
		switch {
		case quote != 0:
			cur.WriteRune(r)
			if r == quote {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
			cur.WriteRune(r)
		case r == ' ' || r == '\t':
			if cur.Len() > 0 {
				toks = append(toks, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		toks = append(toks, cur.String())
	}
	return toks
}

func unquote(tok string) string {
	if len(tok) >= 2 && (tok[0] == '"' || tok[0] == '\'') && tok[len(tok)-1] == tok[0] {
		return strings.ReplaceAll(tok[1:len(tok)-1], `\"`, `"`)
	}
	return tok
}

// quoteArg double-quotes a sway config argument when it contains whitespace
// or quotes. (Sway's own quoting, not the shell's: see replaceProgram in the
// ui package for exec commands.)
func quoteArg(s string) string {
	if !strings.ContainsAny(s, " \t\"'") {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

// canonicalKey maps output subcommand aliases to one name.
func canonicalKey(k string) string {
	switch k {
	case "res", "mode":
		return "resolution"
	case "pos":
		return "position"
	case "background":
		return "bg"
	}
	return k
}

// subcommandArity returns how many argument tokens follow an output
// subcommand, or -1 when the subcommand is not understood.
func subcommandArity(key string, rest []string) int {
	arg := func(i int) string {
		if i < len(rest) {
			return unquote(rest[i])
		}
		return ""
	}
	switch canonicalKey(key) {
	case "resolution":
		if arg(0) == "--custom" {
			return 2
		}
		return 1
	case "position":
		if strings.Contains(arg(0), ",") {
			return 1
		}
		return 2
	case "transform":
		if a := arg(1); a == "clockwise" || a == "anticlockwise" {
			return 2
		}
		return 1
	case "bg":
		if arg(1) == "solid_color" {
			return 2
		}
		if strings.HasPrefix(arg(2), "#") {
			return 3
		}
		return 2
	case "scale", "adaptive_sync", "scale_filter", "subpixel", "render_bit_depth",
		"max_render_time", "hdr", "allow_tearing", "dpms", "power":
		return 1
	case "enable", "disable", "toggle":
		return 0
	}
	return -1
}

type outputSegment struct {
	key  string   // as written
	args []string // tokens as written
}

type outputLine struct {
	name     string
	segments []outputSegment
	// complete is false when parsing stopped at a subcommand the app does
	// not understand; such lines are read but never rewritten.
	complete bool
}

func parseOutputLine(trimmed string) (outputLine, bool) {
	toks := tokenize(trimmed)
	if len(toks) < 3 || toks[0] != "output" || toks[2] == "{" {
		return outputLine{}, false
	}
	ol := outputLine{name: unquote(toks[1]), complete: true}
	rest := toks[2:]
	for len(rest) > 0 {
		n := subcommandArity(rest[0], rest[1:])
		if n < 0 || n > len(rest)-1 {
			ol.complete = false
			break
		}
		ol.segments = append(ol.segments, outputSegment{key: rest[0], args: rest[1 : 1+n]})
		rest = rest[1+n:]
	}
	return ol, true
}

func (ol outputLine) String() string {
	parts := []string{"output", quoteArg(ol.name)}
	for _, s := range ol.segments {
		parts = append(parts, s.key)
		parts = append(parts, s.args...)
	}
	return strings.Join(parts, " ")
}

func joinArgs(args []string) string {
	vals := make([]string, len(args))
	for i, a := range args {
		vals[i] = unquote(a)
	}
	return strings.Join(vals, " ")
}

func (c *Config) outputByName(name string) *Output {
	for i := range c.Outputs {
		if c.Outputs[i].Name == name {
			return &c.Outputs[i]
		}
	}
	c.Outputs = append(c.Outputs, Output{Name: name})
	return &c.Outputs[len(c.Outputs)-1]
}

// applyOutputLine copies the managed settings of one config line into c.
func (c *Config) applyOutputLine(ol outputLine) {
	for _, s := range ol.segments {
		key := canonicalKey(s.key)
		if ol.name == "*" {
			if key == "bg" && len(s.args) >= 2 {
				c.Wallpaper, c.WallpaperMode = unquote(s.args[0]), unquote(s.args[1])
			}
			continue
		}
		if f := (&Output{}).field(key); f != nil {
			*c.outputByName(ol.name).field(key) = joinArgs(s.args)
		}
	}
}

func (c *Config) loadOutputs() error {
	data, _, err := readOptional(OutputsPath())
	if err != nil {
		return err
	}
	for _, line := range splitLines(string(data)) {
		if ol, ok := parseOutputLine(strings.TrimSpace(line)); ok {
			c.applyOutputLine(ol)
		}
	}
	return nil
}

// ValidScale reports whether s is a positive number usable as an output scale.
func ValidScale(s string) bool {
	f, err := strconv.ParseFloat(s, 64)
	return err == nil && f > 0
}

var (
	resolutionRe = regexp.MustCompile(`^(--custom )?\d+x\d+(@\d+(\.\d+)?Hz)?$`)
	positionRe   = regexp.MustCompile(`^-?\d+(,| )-?\d+$`)
	transformRe  = regexp.MustCompile(`^(normal|90|180|270|flipped|flipped-90|flipped-180|flipped-270)( (clockwise|anticlockwise))?$`)
)

// hasControlChars reports whether s contains characters (newlines, NUL, ...)
// that would let a value spill onto a new config line.
func hasControlChars(s string) bool {
	return strings.IndexFunc(s, unicode.IsControl) >= 0
}

func (c *Config) validateOutputs() error {
	for _, o := range c.Outputs {
		checks := []struct {
			name, val string
			ok        func(string) bool
			want      string
		}{
			{"resolution", o.Resolution, resolutionRe.MatchString, "WIDTHxHEIGHT[@RATEHz]"},
			{"position", o.Position, positionRe.MatchString, "X,Y"},
			{"scale", o.Scale, ValidScale, "a positive number"},
			{"transform", o.Transform, transformRe.MatchString, "a sway transform"},
			{"adaptive sync", o.AdaptiveSync, func(v string) bool { return v == "on" || v == "off" }, "on or off"},
		}
		for _, ch := range checks {
			if ch.val != "" && !ch.ok(ch.val) {
				return fmt.Errorf("output %s: %s %q must be %s", o.Name, ch.name, ch.val, ch.want)
			}
		}
	}
	if hasControlChars(c.Wallpaper) {
		return fmt.Errorf("wallpaper path contains control characters")
	}
	if c.Wallpaper == "" {
		return nil
	}
	for _, m := range WallpaperModes {
		if c.WallpaperMode == m {
			return nil
		}
	}
	return fmt.Errorf("wallpaper mode %q must be one of %s", c.WallpaperMode, strings.Join(WallpaperModes, ", "))
}

func (c *Config) saveOutputs() error {
	data, _, err := readOptional(OutputsPath())
	if err != nil {
		return err
	}
	type nameKey struct{ name, key string }
	seen := make(map[nameKey]bool)
	opaque := make(map[nameKey]string) // values on lines we must not rewrite

	var lines []string
	for _, line := range splitLines(string(data)) {
		ol, ok := parseOutputLine(strings.TrimSpace(line))
		if !ok {
			lines = append(lines, line)
			continue
		}
		if !ol.complete {
			for _, s := range ol.segments {
				opaque[nameKey{ol.name, canonicalKey(s.key)}] = joinArgs(s.args)
			}
			lines = append(lines, line)
			continue
		}
		var kept []outputSegment
		for _, s := range ol.segments {
			key := canonicalKey(s.key)
			switch {
			case ol.name == "*" && key == "bg":
				seen[nameKey{"*", "bg"}] = true
				if c.Wallpaper == "" {
					continue
				}
				args := []string{quoteArg(c.Wallpaper), c.WallpaperMode}
				if len(s.args) > 2 {
					args = append(args, s.args[2:]...)
				}
				s.args = args
			case ol.name != "*" && (&Output{}).field(key) != nil:
				seen[nameKey{ol.name, key}] = true
				val := *c.outputByName(ol.name).field(key)
				if val == "" {
					continue
				}
				s.args = strings.Fields(val)
			}
			kept = append(kept, s)
		}
		if len(kept) == 0 {
			continue
		}
		ol.segments = kept
		lines = append(lines, leadingIndent(line)+ol.String())
	}

	wp := nameKey{"*", "bg"}
	if !seen[wp] && c.Wallpaper != "" &&
		(c.Wallpaper != c.mainWallpaper || c.WallpaperMode != c.mainWallpaperMode) &&
		joinArgs([]string{quoteArg(c.Wallpaper), c.WallpaperMode}) != opaque[wp] {
		lines = append(lines, fmt.Sprintf("output * bg %s %s", quoteArg(c.Wallpaper), c.WallpaperMode))
	}

	for i := range c.Outputs {
		out := &c.Outputs[i]
		hasLine := false
		for _, key := range outputFieldOrder {
			if seen[nameKey{out.Name, key}] {
				hasLine = true
			}
		}
		var combined []string
		for _, key := range outputFieldOrder {
			k := nameKey{out.Name, key}
			val := *out.field(key)
			if seen[k] || val == "" || val == opaque[k] {
				continue
			}
			if hasLine {
				lines = append(lines, fmt.Sprintf("output %s %s %s", quoteArg(out.Name), key, val))
			} else {
				combined = append(combined, key, val)
			}
		}
		if len(combined) > 0 {
			lines = append(lines, "output "+quoteArg(out.Name)+" "+strings.Join(combined, " "))
		}
	}
	return writeLines(OutputsPath(), lines)
}
