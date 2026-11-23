package config

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func RulesPath() string { return filepath.Join(SwayConfigDir(), "config.d", "rules.conf") }

// WindowRule is a for_window or assign rule. Criteria is the text inside
// the brackets and Command the rest, both as written. For assign, Command
// is the target without the optional arrow, e.g. "workspace number 2".
type WindowRule struct {
	Kind     string // for_window or assign
	Criteria string
	Command  string
}

// RuleEntry is a rule and the file it comes from. Only rules in rules.conf
// can be changed: sway has no command to undo a for_window or assign, so
// rules elsewhere can't be overridden.
type RuleEntry struct {
	WindowRule
	Source string
}

func (r RuleEntry) Editable() bool { return r.Source == RulesPath() }

func (r WindowRule) line() string {
	return fmt.Sprintf("%s [%s] %s", r.Kind, r.Criteria, r.Command)
}

// splitCriteria returns the text inside the leading [ ... ] of s (honouring
// quotes and backslash escapes) and what follows it.
func splitCriteria(s string) (criteria, rest string, ok bool) {
	if !strings.HasPrefix(s, "[") {
		return "", "", false
	}
	inQuote := false
	for i := 1; i < len(s); i++ {
		switch c := s[i]; {
		case c == '\\':
			i++
		case c == '"':
			inQuote = !inQuote
		case c == ']' && !inQuote:
			return s[1:i], strings.TrimSpace(s[i+1:]), true
		}
	}
	return "", "", false
}

func parseRuleLine(line string) (WindowRule, bool) {
	kind, rest, found := strings.Cut(line, " ")
	if !found || (kind != "for_window" && kind != "assign") {
		return WindowRule{}, false
	}
	criteria, cmd, ok := splitCriteria(strings.TrimSpace(rest))
	if !ok || cmd == "" {
		return WindowRule{}, false
	}
	if kind == "assign" {
		cmd = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(cmd, "→"), "->"))
	}
	return WindowRule{Kind: kind, Criteria: criteria, Command: cmd}, true
}

var criteriaKeys = map[string]bool{
	"all": true, "app_id": true, "class": true, "con_id": true, "con_mark": true,
	"floating": true, "id": true, "instance": true, "pid": true, "shell": true,
	"tiling": true, "title": true, "urgent": true, "window_role": true,
	"window_type": true, "workspace": true, "sandbox_engine": true,
	"sandbox_app_id": true, "sandbox_instance_id": true, "tag": true,
}

var assignTargetRe = regexp.MustCompile(`^(workspace( number)? \S.*|output \S+)$`)

func validateCriteria(c string) error {
	if strings.TrimSpace(c) == "" {
		return fmt.Errorf("criteria can't be empty (use \"all\" for every window)")
	}
	if hasControlChars(c) {
		return fmt.Errorf("criteria contain control characters")
	}
	// Split into key[=value] tokens, honouring quotes.
	var tokens []string
	var cur strings.Builder
	inQuote := false
	for i := 0; i < len(c); i++ {
		ch := c[i]
		switch {
		case ch == '\\' && i+1 < len(c):
			cur.WriteByte(ch)
			i++
			cur.WriteByte(c[i])
		case ch == '"':
			inQuote = !inQuote
			cur.WriteByte(ch)
		case (ch == '[' || ch == ']') && !inQuote:
			return fmt.Errorf("criteria contain an unquoted %q", ch)
		case ch == ' ' && !inQuote:
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(ch)
		}
	}
	if inQuote {
		return fmt.Errorf("criteria have an unclosed quote")
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	for _, t := range tokens {
		key, _, _ := strings.Cut(t, "=")
		if !criteriaKeys[key] {
			return fmt.Errorf("unknown criterion %q", key)
		}
	}
	return nil
}

func validateRule(r WindowRule) error {
	if r.Kind != "for_window" && r.Kind != "assign" {
		return fmt.Errorf("rule type %q must be for_window or assign", r.Kind)
	}
	if err := validateCriteria(r.Criteria); err != nil {
		return err
	}
	cmd := strings.TrimSpace(r.Command)
	if cmd == "" {
		return fmt.Errorf("rule has no command")
	}
	if hasControlChars(cmd) {
		return fmt.Errorf("rule command contains control characters")
	}
	if r.Kind == "assign" && !assignTargetRe.MatchString(cmd) {
		return fmt.Errorf("assign target %q must be \"workspace <name>\", \"workspace number <n>\" or \"output <name>\"", cmd)
	}
	return nil
}

func (c *Config) readRules(lines []string, source string) {
	for _, line := range parseTopLevel(lines) {
		if r, ok := parseRuleLine(line); ok {
			c.Rules = append(c.Rules, RuleEntry{WindowRule: r, Source: source})
		}
	}
}

// loadRules reads rules from config.d files (other than rules.conf, which
// the app manages) as read-only, then rules.conf as editable. The main
// config's rules are read in loadMain.
func (c *Config) loadRules() error {
	files, err := filepath.Glob(filepath.Join(SwayConfigDir(), "config.d", "*"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, f := range files {
		if f == RulesPath() {
			continue
		}
		data, _, err := readOptional(f)
		if err != nil {
			return err
		}
		c.readRules(splitLines(string(data)), f)
	}

	data, _, err := readOptional(RulesPath())
	if err != nil {
		return err
	}
	c.rulesExtra = nil
	for _, line := range splitLines(string(data)) {
		trimmed := strings.TrimSpace(line)
		if r, ok := parseRuleLine(trimmed); ok && !strings.HasPrefix(trimmed, "#") {
			c.Rules = append(c.Rules, RuleEntry{WindowRule: r, Source: RulesPath()})
		} else {
			c.rulesExtra = append(c.rulesExtra, line)
		}
	}
	return nil
}

func (c *Config) AddRule(r WindowRule) error {
	if err := validateRule(r); err != nil {
		return err
	}
	c.Rules = append(c.Rules, RuleEntry{WindowRule: r, Source: RulesPath()})
	return nil
}

func (c *Config) UpdateRule(i int, r WindowRule) error {
	if !c.Rules[i].Editable() {
		return fmt.Errorf("this rule is in %s; sway can't override it from another file, so edit it there", c.Rules[i].Source)
	}
	if err := validateRule(r); err != nil {
		return err
	}
	c.Rules[i].WindowRule = r
	return nil
}

func (c *Config) DeleteRule(i int) error {
	if !c.Rules[i].Editable() {
		return fmt.Errorf("this rule is in %s; sway can't remove it from another file, so delete it there", c.Rules[i].Source)
	}
	c.Rules = append(c.Rules[:i], c.Rules[i+1:]...)
	return nil
}

func (c *Config) validateRules() error {
	for _, r := range c.Rules {
		if r.Editable() {
			if err := validateRule(r.WindowRule); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Config) saveRules() error {
	lines := append([]string(nil), c.rulesExtra...)
	for _, r := range c.Rules {
		if r.Editable() {
			lines = append(lines, r.line())
		}
	}
	return writeLines(RulesPath(), lines)
}

// OpenWindow is a window currently open in sway.
type OpenWindow struct {
	AppID, Class, Title, Shell string
}

// Criteria matches this window by app_id (Wayland) or class (Xwayland).
func (w OpenWindow) Criteria() string {
	quote := func(v string) string { return `"` + strings.ReplaceAll(v, `"`, `\"`) + `"` }
	if w.AppID != "" {
		return "app_id=" + quote(w.AppID)
	}
	return "class=" + quote(w.Class)
}

// swayTree returns the raw JSON of `swaymsg -t get_tree`. A variable so
// tests can replace it.
var swayTree = func() ([]byte, error) {
	out, err := exec.Command("swaymsg", "-t", "get_tree", "--raw").Output()
	if err != nil {
		return nil, fmt.Errorf("swaymsg -t get_tree: %w", err)
	}
	return out, nil
}

// OpenWindows lists open windows that can be matched by app_id or class.
func OpenWindows() ([]OpenWindow, error) {
	data, err := swayTree()
	if err != nil {
		return nil, err
	}
	type node struct {
		PID              int     `json:"pid"`
		AppID            *string `json:"app_id"`
		Name             *string `json:"name"`
		Shell            string  `json:"shell"`
		WindowProperties *struct {
			Class string `json:"class"`
		} `json:"window_properties"`
		Nodes         []node `json:"nodes"`
		FloatingNodes []node `json:"floating_nodes"`
	}
	var root node
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parsing swaymsg output: %w", err)
	}
	var wins []OpenWindow
	var walk func(n node)
	walk = func(n node) {
		if n.PID > 0 {
			w := OpenWindow{Shell: n.Shell}
			if n.AppID != nil {
				w.AppID = *n.AppID
			}
			if n.Name != nil {
				w.Title = *n.Name
			}
			if n.WindowProperties != nil {
				w.Class = n.WindowProperties.Class
			}
			if w.AppID != "" || w.Class != "" {
				wins = append(wins, w)
			}
		}
		for _, c := range n.Nodes {
			walk(c)
		}
		for _, c := range n.FloatingNodes {
			walk(c)
		}
	}
	walk(root)
	return wins, nil
}
