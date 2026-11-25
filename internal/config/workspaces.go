package config

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func WorkspacesPath() string { return filepath.Join(SwayConfigDir(), "config.d", "workspaces.conf") }

// Workspace holds settings for a numbered workspace. Its sway name is the
// number, or "number:label" when it has a label.
type Workspace struct {
	Number    int
	Label     string
	Outputs   string // output names, space separated
	GapsInner string
	GapsOuter string
}

func (w Workspace) Name() string {
	if w.Label == "" {
		return strconv.Itoa(w.Number)
	}
	return fmt.Sprintf("%d:%s", w.Number, w.Label)
}

type wsValues struct{ outputs, gapsInner, gapsOuter string }

var (
	// wsCommandRe matches binding commands that switch to or move to a
	// numbered workspace, capturing the prefix, number and label.
	wsCommandRe = regexp.MustCompile(`^((?:move (?:container |window )?(?:to )?)?workspace number) (\d+)(?::(\S+))?$`)
	wsNameRe    = regexp.MustCompile(`^(\d+)(?::(\S+))?$`)
	wsLabelRe   = regexp.MustCompile(`^[^\s"'\\]*$`)
)

// parseWorkspaceLine reads `workspace <name> output <outputs...>` and
// `workspace <name> gaps inner|outer <n>`.
func parseWorkspaceLine(line string) (name, field, value string, ok bool) {
	toks := tokenize(line)
	if len(toks) < 4 || toks[0] != "workspace" {
		return "", "", "", false
	}
	for i := 2; i < len(toks); i++ {
		switch toks[i] {
		case "output":
			return unquote(strings.Join(toks[1:i], " ")), "output", joinArgs(toks[i+1:]), i+1 < len(toks)
		case "gaps":
			if i+2 < len(toks) && (toks[i+1] == "inner" || toks[i+1] == "outer") && i+3 == len(toks) {
				return unquote(strings.Join(toks[1:i], " ")), "gaps " + toks[i+1], toks[i+2], true
			}
			return "", "", "", false
		}
	}
	return "", "", "", false
}

func setWsValue(m map[string]wsValues, name, field, value string) {
	v := m[name]
	switch field {
	case "output":
		v.outputs = value
	case "gaps inner":
		v.gapsInner = value
	case "gaps outer":
		v.gapsOuter = value
	}
	m[name] = v
}

// readMainWorkspaces records workspace settings in the main config, which
// are only written again when changed.
func (c *Config) readMainWorkspaces(lines []string) {
	for _, line := range parseTopLevel(lines) {
		if name, field, value, ok := parseWorkspaceLine(line); ok {
			setWsValue(c.wsMain, name, field, value)
		}
	}
}

// Workspace returns the settings for workspace n, adding it if needed.
func (c *Config) Workspace(n int) *Workspace {
	for i := range c.Workspaces {
		if c.Workspaces[i].Number == n {
			return &c.Workspaces[i]
		}
	}
	c.Workspaces = append(c.Workspaces, Workspace{Number: n})
	sort.Slice(c.Workspaces, func(i, j int) bool { return c.Workspaces[i].Number < c.Workspaces[j].Number })
	for i := range c.Workspaces {
		if c.Workspaces[i].Number == n {
			return &c.Workspaces[i]
		}
	}
	return nil
}

// loadWorkspaces builds workspaces 1-10 (plus any other numbers in use),
// taking labels from the bindings and settings from the main config with
// workspaces.conf on top. Runs after bindings are loaded.
func (c *Config) loadWorkspaces() error {
	for n := 1; n <= 10; n++ {
		c.Workspace(n)
	}
	for _, e := range c.Bindings {
		if e.Removed {
			continue
		}
		if m := wsCommandRe.FindStringSubmatch(e.Command); m != nil {
			n, _ := strconv.Atoi(m[2])
			if w := c.Workspace(n); w.Label == "" {
				w.Label = m[3]
			}
		}
	}

	data, _, err := readOptional(WorkspacesPath())
	if err != nil {
		return err
	}
	file := map[string]wsValues{}
	c.wsExtra = nil
	for _, raw := range splitLines(string(data)) {
		line := strings.TrimSpace(raw)
		name, field, value, ok := parseWorkspaceLine(line)
		if ok && !strings.HasPrefix(line, "#") {
			if m := wsNameRe.FindStringSubmatch(name); m != nil {
				n, _ := strconv.Atoi(m[1])
				c.Workspace(n)
				setWsValue(file, name, field, value)
				continue
			}
		}
		c.wsExtra = append(c.wsExtra, raw)
	}

	for i := range c.Workspaces {
		w := &c.Workspaces[i]
		v := c.wsMain[w.Name()]
		if f, ok := file[w.Name()]; ok {
			if f.outputs != "" {
				v.outputs = f.outputs
			}
			if f.gapsInner != "" {
				v.gapsInner = f.gapsInner
			}
			if f.gapsOuter != "" {
				v.gapsOuter = f.gapsOuter
			}
		}
		w.Outputs, w.GapsInner, w.GapsOuter = v.outputs, v.gapsInner, v.gapsOuter
	}
	return nil
}

// SetWorkspaceLabel names workspace n by rewriting every binding that
// switches to or moves to it (as binding overrides). An empty label returns
// it to its plain number.
func (c *Config) SetWorkspaceLabel(n int, label string) error {
	if !wsLabelRe.MatchString(label) || hasControlChars(label) {
		return fmt.Errorf("workspace name %q can't contain spaces, quotes or backslashes", label)
	}
	w := c.Workspace(n)
	name := strconv.Itoa(n)
	if label != "" {
		name += ":" + label
	}
	for i := len(c.Bindings) - 1; i >= 0; i-- {
		e := c.Bindings[i]
		if e.Removed {
			continue
		}
		m := wsCommandRe.FindStringSubmatch(e.Command)
		if m == nil || m[2] != strconv.Itoa(n) {
			continue
		}
		b := e.Binding
		b.Command = m[1] + " " + name
		if err := c.UpdateBinding(i, b); err != nil {
			return err
		}
	}
	w.Label = label
	return nil
}

func (c *Config) validateWorkspaces() error {
	for _, w := range c.Workspaces {
		if !wsLabelRe.MatchString(w.Label) || hasControlChars(w.Label) {
			return fmt.Errorf("workspace %d: invalid name %q", w.Number, w.Label)
		}
		for name, v := range map[string]string{"inner gaps": w.GapsInner, "outer gaps": w.GapsOuter} {
			if v != "" && !validNonNegInt(v) {
				return fmt.Errorf("workspace %d: %s %q must be a whole number of pixels", w.Number, name, v)
			}
		}
		if hasControlChars(w.Outputs) {
			return fmt.Errorf("workspace %d: outputs contain control characters", w.Number)
		}
	}
	return nil
}

// saveWorkspaces writes kept lines, then each workspace setting that
// differs from the main config.
func (c *Config) saveWorkspaces() error {
	lines := append([]string(nil), c.wsExtra...)
	for _, w := range c.Workspaces {
		main := c.wsMain[w.Name()]
		name := quoteArg(w.Name())
		if w.Outputs != "" && w.Outputs != main.outputs {
			lines = append(lines, fmt.Sprintf("workspace %s output %s", name, w.Outputs))
		}
		if w.GapsInner != "" && w.GapsInner != main.gapsInner {
			lines = append(lines, fmt.Sprintf("workspace %s gaps inner %s", name, w.GapsInner))
		}
		if w.GapsOuter != "" && w.GapsOuter != main.gapsOuter {
			lines = append(lines, fmt.Sprintf("workspace %s gaps outer %s", name, w.GapsOuter))
		}
	}
	return writeLines(WorkspacesPath(), lines)
}
