package config

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func BindingsPath() string { return filepath.Join(SwayConfigDir(), "config.d", "bindings.conf") }

// Binding is one bindsym/bindcode. Combo and Command are kept exactly as
// written (quotes, escapes and $variables intact).
type Binding struct {
	Mode    string   // "" for the default mode, else e.g. "resize"
	Kind    string   // bindsym or bindcode
	Flags   []string // e.g. --locked; --no-warn is not kept
	Combo   string   // e.g. $mod+Shift+e
	Command string   // raw text after the combo
}

// BindingEntry is a binding as the user sees it. Main is the binding from the
// main config (read-only), or nil for bindings the app added.
type BindingEntry struct {
	Binding
	Main    *Binding
	Removed bool
}

const (
	StatusMain       = "main"
	StatusOverridden = "overridden"
	StatusRemoved    = "removed"
	StatusAdded      = "added"
)

func (e BindingEntry) Status() string {
	switch {
	case e.Main == nil:
		return StatusAdded
	case e.Removed:
		return StatusRemoved
	case e.Command != e.Main.Command:
		return StatusOverridden
	}
	return StatusMain
}

var (
	bindRe   = regexp.MustCompile(`^(bindsym|bindcode)((?:\s+--\S+)*)\s+(\S+)\s+(.+)$`)
	unbindRe = regexp.MustCompile(`^(unbindsym|unbindcode)((?:\s+--\S+)*)\s+(\S+)$`)
	comboRe  = regexp.MustCompile(`^[A-Za-z0-9_$:+-]+$`)
	setRe    = regexp.MustCompile(`^set\s+\$(\S+)\s+(.+)$`)
	modeRe   = regexp.MustCompile(`^mode\s+(?:--pango_markup\s+)?("[^"]*"|\S+)\s*\{$`)
)

var allowedBindFlags = map[string]bool{
	"--release": true, "--locked": true, "--to-code": true, "--no-repeat": true,
	"--inhibited": true, "--whole-window": true, "--border": true, "--exclude-titlebar": true,
}

func parseFlags(s string) []string {
	var flags []string
	for _, f := range strings.Fields(s) {
		if f != "--no-warn" {
			flags = append(flags, f)
		}
	}
	return flags
}

func (b Binding) line(noWarn bool) string {
	parts := []string{b.Kind}
	if noWarn {
		parts = append(parts, "--no-warn")
	}
	parts = append(parts, b.Flags...)
	parts = append(parts, b.Combo, b.Command)
	return strings.Join(parts, " ")
}

func (b Binding) unbindLine() string {
	parts := append([]string{"un" + b.Kind}, b.Flags...)
	return strings.Join(append(parts, b.Combo), " ")
}

func validateBinding(b Binding) error {
	if hasControlChars(b.Mode) || strings.Contains(b.Mode, `"`) {
		return fmt.Errorf("mode name %q may not contain quotes or control characters", b.Mode)
	}
	if b.Kind != "bindsym" && b.Kind != "bindcode" {
		return fmt.Errorf("binding type %q must be bindsym or bindcode", b.Kind)
	}
	for _, f := range b.Flags {
		if !allowedBindFlags[f] && !strings.HasPrefix(f, "--input-device=") {
			return fmt.Errorf("unsupported binding flag %q", f)
		}
	}
	if !comboRe.MatchString(b.Combo) {
		return fmt.Errorf("key combination %q must look like $mod+Shift+q", b.Combo)
	}
	if strings.TrimSpace(b.Command) == "" {
		return fmt.Errorf("binding %s has no command", b.Combo)
	}
	if hasControlChars(b.Command) {
		return fmt.Errorf("binding %s: command contains control characters", b.Combo)
	}
	return nil
}

// logicalLines joins lines ending in a backslash with the next line.
// Comments are never joined, so a commented-out multi-line example stays
// a comment. Each result carries the index of its first physical line.
func logicalLines(lines []string) (out []string, starts []int) {
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		start := i
		if !strings.HasPrefix(line, "#") {
			for strings.HasSuffix(line, `\`) && i+1 < len(lines) {
				i++
				line = strings.TrimRight(strings.TrimSuffix(line, `\`), " \t") + " " + strings.TrimSpace(lines[i])
			}
		}
		out = append(out, line)
		starts = append(starts, start)
	}
	return out, starts
}

// parseTopLevel returns top-level (default mode) logical lines, skipping
// comments and anything inside { } blocks.
func parseTopLevel(lines []string) []string {
	logical, _ := logicalLines(lines)
	var top []string
	depth := 0
	for _, line := range logical {
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case strings.HasPrefix(line, "}"):
			if depth > 0 {
				depth--
			}
		case strings.HasSuffix(line, "{"):
			depth++
		case depth == 0:
			top = append(top, line)
		}
	}
	return top
}

func parseBindLine(line string) (Binding, bool) {
	m := bindRe.FindStringSubmatch(line)
	if m == nil {
		return Binding{}, false
	}
	return Binding{Kind: m[1], Flags: parseFlags(m[2]), Combo: m[3], Command: strings.TrimSpace(m[4])}, true
}

func parseUnbindLine(line string) (Binding, bool) {
	m := unbindRe.FindStringSubmatch(line)
	if m == nil {
		return Binding{}, false
	}
	return Binding{Kind: strings.TrimPrefix(m[1], "un"), Flags: parseFlags(m[2]), Combo: m[3]}, true
}

var modifierAliases = map[string]string{
	"control": "ctrl", "mod1": "alt", "super": "mod4", "caps": "lock",
}

// bindingKey identifies a binding the way sway compares them: variables
// expanded, modifiers case-insensitive and unordered, flags unordered.
func (c *Config) bindingKey(b Binding) string {
	combo := b.Combo
	names := make([]string, 0, len(c.vars))
	for n := range c.vars {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool { return len(names[i]) > len(names[j]) })
	for _, n := range names {
		combo = strings.ReplaceAll(combo, "$"+n, c.vars[n])
	}
	parts := strings.Split(combo, "+")
	key := parts[len(parts)-1]
	var mods []string
	for _, m := range parts[:len(parts)-1] {
		m = strings.ToLower(m)
		if a, ok := modifierAliases[m]; ok {
			m = a
		}
		mods = append(mods, m)
	}
	sort.Strings(mods)
	flags := append([]string(nil), b.Flags...)
	sort.Strings(flags)
	return b.Mode + "|" + b.Kind + "|" + strings.Join(flags, " ") + "|" + strings.Join(mods, "+") + "+" + key
}

// scanBindings walks a file's logical lines, calling onBind for bindsym,
// bindcode and unbind lines at the top level or directly inside a mode
// block, and onOther for every other line with its mode ("" at top level)
// and whether it opens or closes a mode block. Other blocks (bar { },
// input { }) are skipped.
func scanBindings(lines []string, onLine func(mode, line string, start int, kind lineKind)) {
	logical, starts := logicalLines(lines)
	depth := 0
	mode := ""
	for li, line := range logical {
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
			onLine(mode, line, starts[li], lineOther)
		case strings.HasPrefix(line, "}"):
			depth--
			if depth <= 0 {
				depth = 0
				if mode != "" {
					onLine(mode, line, starts[li], lineModeClose)
					mode = ""
					continue
				}
			}
			onLine(mode, line, starts[li], lineOther)
		case depth == 0 && modeRe.MatchString(line):
			mode = unquote(modeRe.FindStringSubmatch(line)[1])
			depth = 1
			onLine(mode, line, starts[li], lineModeOpen)
		case strings.HasSuffix(line, "{"):
			depth++
			onLine(mode, line, starts[li], lineOther)
		case depth == 0 || (depth == 1 && mode != ""):
			onLine(mode, line, starts[li], lineBinding)
		default:
			onLine(mode, line, starts[li], lineOther)
		}
	}
}

type lineKind int

const (
	lineOther lineKind = iota
	lineBinding
	lineModeOpen
	lineModeClose
)

// readMainBindings records variables and bindings (default mode and mode
// blocks) from the main config.
func (c *Config) readMainBindings(lines []string) {
	scanBindings(lines, func(mode, line string, _ int, kind lineKind) {
		if kind != lineBinding {
			return
		}
		if m := setRe.FindStringSubmatch(line); m != nil && mode == "" {
			c.vars[m[1]] = strings.TrimSpace(m[2])
		}
		if b, ok := parseBindLine(line); ok {
			b.Mode = mode
			main := b
			c.Bindings = append(c.Bindings, BindingEntry{Binding: b, Main: &main})
		}
	})
}

// Modes lists the binding modes in use, starting with "" (default).
func (c *Config) Modes() []string {
	modes := []string{""}
	seen := map[string]bool{"": true}
	for _, e := range c.Bindings {
		if !seen[e.Mode] {
			seen[e.Mode] = true
			modes = append(modes, e.Mode)
		}
	}
	return modes
}

func (c *Config) findEntry(key string, includeRemoved bool, skip int) int {
	for i, e := range c.Bindings {
		if i != skip && (includeRemoved || !e.Removed) && c.bindingKey(e.Binding) == key {
			return i
		}
	}
	return -1
}

func (c *Config) findMainEntry(key string) int {
	for i, e := range c.Bindings {
		if e.Main != nil && c.bindingKey(*e.Main) == key {
			return i
		}
	}
	return -1
}

// loadBindings applies bindings.conf on top of the main config's bindings.
// Lines it doesn't understand are kept and written back unchanged (inside
// their mode block if they were in one).
func (c *Config) loadBindings() error {
	data, _, err := readOptional(BindingsPath())
	if err != nil {
		return err
	}
	lines := splitLines(string(data))
	c.bindingsExtra = nil
	c.modeExtra = map[string][]string{}
	var starts []int
	consumed := map[int]bool{}
	scanBindings(lines, func(mode, line string, start int, kind lineKind) {
		starts = append(starts, start)
		switch kind {
		case lineModeOpen, lineModeClose:
			consumed[start] = true
			return
		case lineBinding:
			if ub, ok := parseUnbindLine(line); ok {
				ub.Mode = mode
				if i := c.findMainEntry(c.bindingKey(ub)); i >= 0 {
					c.Bindings[i].Removed = true
					consumed[start] = true
					return
				}
			} else if b, ok := parseBindLine(line); ok {
				b.Mode = mode
				if i := c.findMainEntry(c.bindingKey(b)); i >= 0 {
					c.Bindings[i].Binding = b
					c.Bindings[i].Removed = false
				} else {
					c.Bindings = append(c.Bindings, BindingEntry{Binding: b})
				}
				consumed[start] = true
				return
			}
		}
		if mode != "" && line != "" {
			c.modeExtra[mode] = append(c.modeExtra[mode], line)
			consumed[start] = true
		}
	})
	// Keep other top-level lines as written.
	for k, start := range starts {
		if consumed[start] {
			continue
		}
		end := len(lines)
		if k+1 < len(starts) {
			end = starts[k+1]
		}
		c.bindingsExtra = append(c.bindingsExtra, lines[start:end]...)
	}
	return nil
}

// AddBinding adds a new binding. Re-adding a removed main binding's keys
// turns it back on with the new command.
func (c *Config) AddBinding(b Binding) error {
	if err := validateBinding(b); err != nil {
		return err
	}
	key := c.bindingKey(b)
	if i := c.findEntry(key, false, -1); i >= 0 {
		return fmt.Errorf("%s is already bound to %q", b.Combo, c.Bindings[i].Command)
	}
	if i := c.findMainEntry(key); i >= 0 {
		c.Bindings[i].Binding = b
		c.Bindings[i].Removed = false
		return nil
	}
	c.Bindings = append(c.Bindings, BindingEntry{Binding: b})
	return nil
}

// UpdateBinding replaces binding i. Changing the keys or flags of a main
// binding removes the original and adds the edited one as a new binding.
func (c *Config) UpdateBinding(i int, b Binding) error {
	if err := validateBinding(b); err != nil {
		return err
	}
	key := c.bindingKey(b)
	if j := c.findEntry(key, false, i); j >= 0 {
		return fmt.Errorf("%s is already bound to %q", b.Combo, c.Bindings[j].Command)
	}
	e := &c.Bindings[i]
	if e.Main == nil {
		// An added binding edited to the keys of a removed original turns
		// that original back on instead of unbinding and rebinding it.
		if j := c.findMainEntry(key); j >= 0 && c.Bindings[j].Removed {
			c.Bindings[j].Binding = b
			c.Bindings[j].Removed = false
			c.Bindings = append(c.Bindings[:i], c.Bindings[i+1:]...)
			return nil
		}
		e.Binding = b
		return nil
	}
	if key == c.bindingKey(*e.Main) {
		e.Binding = b
		e.Removed = false
		return nil
	}
	e.Removed = true
	e.Binding = *e.Main
	c.Bindings = append(c.Bindings, BindingEntry{Binding: b})
	return nil
}

// RemoveBinding disables a main binding (written as unbind) or deletes one
// the app added.
func (c *Config) RemoveBinding(i int) {
	if e := &c.Bindings[i]; e.Main != nil {
		e.Removed = true
		e.Binding = *e.Main
		return
	}
	c.Bindings = append(c.Bindings[:i], c.Bindings[i+1:]...)
}

// RestoreBinding turns a main binding back to its original form.
func (c *Config) RestoreBinding(i int) {
	if e := &c.Bindings[i]; e.Main != nil {
		e.Removed = false
		e.Binding = *e.Main
	}
}

func (c *Config) validateBindings() error {
	for _, e := range c.Bindings {
		if s := e.Status(); s == StatusAdded || s == StatusOverridden {
			if err := validateBinding(e.Binding); err != nil {
				return err
			}
		}
	}
	return nil
}

// saveBindings writes bindings.conf: kept lines, default-mode unbinds and
// binds, then one block per mode (kept lines, unbinds, binds).
func (c *Config) saveBindings() error {
	linesFor := func(mode string) []string {
		var out []string
		for _, e := range c.Bindings {
			if e.Mode == mode && e.Status() == StatusRemoved {
				out = append(out, e.Main.unbindLine())
			}
		}
		for _, e := range c.Bindings {
			if e.Mode != mode {
				continue
			}
			switch e.Status() {
			case StatusOverridden:
				out = append(out, e.line(true))
			case StatusAdded:
				out = append(out, e.line(false))
			}
		}
		return out
	}
	lines := append([]string(nil), c.bindingsExtra...)
	lines = append(lines, linesFor("")...)
	modes := c.Modes()[1:]
	for m := range c.modeExtra {
		found := false
		for _, x := range modes {
			found = found || x == m
		}
		if !found {
			modes = append(modes, m)
		}
	}
	for _, mode := range modes {
		inner := append(append([]string(nil), c.modeExtra[mode]...), linesFor(mode)...)
		if len(inner) == 0 {
			continue
		}
		lines = append(lines, fmt.Sprintf("mode %q {", mode))
		for _, l := range inner {
			lines = append(lines, "    "+l)
		}
		lines = append(lines, "}")
	}
	return writeLines(BindingsPath(), lines)
}

// swayCheck runs `sway -C -c path` and returns its output. A variable so
// tests can replace it.
var swayCheck = func(path string) (string, error) {
	out, err := exec.Command("sway", "-C", "-c", path).CombinedOutput()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		// Validation failures exit non-zero; the output says why.
		return string(out), err
	}
	if err != nil {
		return "", fmt.Errorf("running sway -C: %w", err)
	}
	return string(out), nil
}

// CheckConfig validates the main config (and everything it includes) with
// sway, returning an error listing any problems.
func CheckConfig() error {
	out, runErr := swayCheck(MainConfigPath())
	var problems []string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "[ERROR]") {
			if i := strings.Index(line, "Error on"); i >= 0 {
				line = line[i:]
			}
			problems = append(problems, strings.TrimSpace(line))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("sway reports config errors:\n%s", strings.Join(problems, "\n"))
	}
	if runErr != nil {
		return fmt.Errorf("sway -C failed: %w", runErr)
	}
	return nil
}
