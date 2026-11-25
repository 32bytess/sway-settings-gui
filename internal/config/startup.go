package config

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func StartupPath() string { return filepath.Join(SwayConfigDir(), "config.d", "startup.conf") }

// StartupEntry is an exec (run once at login) or exec_always (run at login
// and on every reload). Command is kept exactly as written.
type StartupEntry struct {
	Kind    string // exec or exec_always
	Command string
	Source  string
}

// Editable reports whether the entry is in startup.conf. Sway can't cancel
// an exec from another file, so entries elsewhere are read-only.
func (s StartupEntry) Editable() bool { return s.Source == StartupPath() }

var execRe = regexp.MustCompile(`^(exec|exec_always)\s+(.+)$`)

func parseExecLine(line string) (StartupEntry, bool) {
	m := execRe.FindStringSubmatch(line)
	if m == nil {
		return StartupEntry{}, false
	}
	return StartupEntry{Kind: m[1], Command: strings.TrimSpace(m[2])}, true
}

func validateStartup(s StartupEntry) error {
	if s.Kind != "exec" && s.Kind != "exec_always" {
		return fmt.Errorf("startup type %q must be exec or exec_always", s.Kind)
	}
	if strings.TrimSpace(s.Command) == "" {
		return fmt.Errorf("startup entry has no command")
	}
	if hasControlChars(s.Command) {
		return fmt.Errorf("startup command contains control characters")
	}
	return nil
}

func (c *Config) readStartup(lines []string, source string) {
	for _, line := range parseTopLevel(lines) {
		if s, ok := parseExecLine(line); ok {
			s.Source = source
			c.Startup = append(c.Startup, s)
		}
	}
}

// loadStartup reads exec lines from config.d files (read-only) and
// startup.conf (editable). The main config's are read in loadMain.
func (c *Config) loadStartup() error {
	files, err := filepath.Glob(filepath.Join(SwayConfigDir(), "config.d", "*"))
	if err != nil {
		return err
	}
	for _, f := range files {
		if f == StartupPath() {
			continue
		}
		data, _, err := readOptional(f)
		if err != nil {
			return err
		}
		c.readStartup(splitLines(string(data)), f)
	}
	data, _, err := readOptional(StartupPath())
	if err != nil {
		return err
	}
	c.startupExtra = nil
	for _, line := range splitLines(string(data)) {
		if s, ok := parseExecLine(strings.TrimSpace(line)); ok {
			s.Source = StartupPath()
			c.Startup = append(c.Startup, s)
		} else {
			c.startupExtra = append(c.startupExtra, line)
		}
	}
	return nil
}

func (c *Config) AddStartup(s StartupEntry) error {
	if err := validateStartup(s); err != nil {
		return err
	}
	s.Source = StartupPath()
	c.Startup = append(c.Startup, s)
	return nil
}

func (c *Config) UpdateStartup(i int, s StartupEntry) error {
	if !c.Startup[i].Editable() {
		return fmt.Errorf("this entry is in %s; sway can't cancel it from another file, so edit it there", c.Startup[i].Source)
	}
	if err := validateStartup(s); err != nil {
		return err
	}
	s.Source = StartupPath()
	c.Startup[i] = s
	return nil
}

func (c *Config) DeleteStartup(i int) error {
	if !c.Startup[i].Editable() {
		return fmt.Errorf("this entry is in %s; sway can't cancel it from another file, so delete it there", c.Startup[i].Source)
	}
	c.Startup = append(c.Startup[:i], c.Startup[i+1:]...)
	return nil
}

func (c *Config) validateStartupEntries() error {
	for _, s := range c.Startup {
		if s.Editable() {
			if err := validateStartup(s); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Config) saveStartup() error {
	lines := append([]string(nil), c.startupExtra...)
	for _, s := range c.Startup {
		if s.Editable() {
			lines = append(lines, s.Kind+" "+s.Command)
		}
	}
	return writeLines(StartupPath(), lines)
}

// runSwaymsg runs swaymsg with args. A variable so tests can replace it.
var runSwaymsg = func(args ...string) error {
	out, err := exec.Command("swaymsg", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("swaymsg %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// RunNow starts command through sway, as an exec line would.
func RunNow(command string) error {
	if err := validateStartup(StartupEntry{Kind: "exec", Command: command}); err != nil {
		return err
	}
	return runSwaymsg("exec", command)
}
