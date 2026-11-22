package config

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Config struct {
	ModKey   string
	Terminal string
	Menu     string
	Opacity  float64

	GapsInner    int
	GapsOuter    int
	CornerRadius int
	BorderSize   int
	BlurEnabled  bool
	BlurPasses   int
	BlurRadius   int
	DimInactive  float64

	Outputs []Output

	// SwayFX reports whether the compositor accepts SwayFX-only commands
	// (corner_radius, blur*, default_dim_inactive). When false they are
	// neither written nor kept, since plain sway rejects them.
	SwayFX bool
}

// swayVersion returns the output of `sway --version`. A variable so tests
// can replace it.
var swayVersion = func() (string, error) {
	out, err := exec.Command("sway", "--version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("sway --version: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func isSwayFX(versionOutput string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(versionOutput)), "swayfx")
}

// DetectSwayFX reports whether the installed sway binary is SwayFX.
func DetectSwayFX() (bool, error) {
	out, err := swayVersion()
	if err != nil {
		return false, err
	}
	return isSwayFX(out), nil
}

type Output struct {
	Name       string
	Resolution string
	Position   string
	Transform  string
}

// SystemConfigPath is sway's system-wide default config, used when the user
// has no config of their own. A variable so tests can redirect it.
var SystemConfigPath = "/etc/sway/config"

// ConfigDInclude is added to a newly created main config so the files this
// tool writes under config.d are loaded.
const ConfigDInclude = "include ~/.config/sway/config.d/*"

func SwayConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "sway")
}

// LXQtConfigPath is the sway config an LXQt Wayland session loads instead of
// ~/.config/sway/config.
func LXQtConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "lxqt", "wayland", "lxqt-sway.config")
}

// MainConfigPath returns the LXQt sway config when it exists, since that is
// the file the running session reads, and ~/.config/sway/config otherwise.
func MainConfigPath() string {
	if info, err := os.Stat(LXQtConfigPath()); err == nil && info.Mode().IsRegular() {
		return LXQtConfigPath()
	}
	return filepath.Join(SwayConfigDir(), "config")
}

func AppearancePath() string { return filepath.Join(SwayConfigDir(), "config.d", "appearance.conf") }
func OutputsPath() string    { return filepath.Join(SwayConfigDir(), "config.d", "outputs.conf") }

func Load() (*Config, error) {
	cfg := &Config{
		ModKey:       "Mod4",
		Terminal:     "kitty",
		Opacity:      1.0,
		GapsInner:    4,
		GapsOuter:    2,
		CornerRadius: 15,
		BorderSize:   1,
		BlurEnabled:  true,
		BlurPasses:   2,
		BlurRadius:   8,
		DimInactive:  0.1,
	}
	if err := cfg.loadMain(); err != nil {
		return nil, fmt.Errorf("main config: %w", err)
	}
	if err := cfg.loadAppearance(); err != nil {
		return nil, fmt.Errorf("appearance: %w", err)
	}
	if err := cfg.loadOutputs(); err != nil {
		return nil, fmt.Errorf("outputs: %w", err)
	}
	return cfg, nil
}

func (c *Config) loadMain() error {
	data, exists, err := readOptional(MainConfigPath())
	if err != nil {
		return err
	}
	if !exists {
		// Sway itself falls back to the system config, so read defaults there.
		data, _, err = readOptional(SystemConfigPath)
		if err != nil {
			return fmt.Errorf("reading system config: %w", err)
		}
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if m := regexp.MustCompile(`^set \$mod (\S+)`).FindStringSubmatch(line); m != nil {
			c.ModKey = m[1]
		}
		if m := regexp.MustCompile(`^set \$term (.+)`).FindStringSubmatch(line); m != nil {
			c.Terminal = strings.TrimSpace(m[1])
		}
		if m := regexp.MustCompile(`^set \$menu (.+)`).FindStringSubmatch(line); m != nil {
			c.Menu = strings.TrimSpace(m[1])
		}
		if m := regexp.MustCompile(`^set \$opacity ([0-9.]+)`).FindStringSubmatch(line); m != nil {
			c.Opacity, _ = strconv.ParseFloat(m[1], 64)
		}
	}
	return nil
}

func (c *Config) loadAppearance() error {
	data, _, err := readOptional(AppearancePath())
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		if m := regexp.MustCompile(`^gaps inner (\d+)`).FindStringSubmatch(line); m != nil {
			c.GapsInner, _ = strconv.Atoi(m[1])
		}
		if m := regexp.MustCompile(`^gaps outer (\d+)`).FindStringSubmatch(line); m != nil {
			c.GapsOuter, _ = strconv.Atoi(m[1])
		}
		if m := regexp.MustCompile(`^corner_radius (\d+)`).FindStringSubmatch(line); m != nil {
			c.CornerRadius, _ = strconv.Atoi(m[1])
		}
		if m := regexp.MustCompile(`^default_border pixel (\d+)`).FindStringSubmatch(line); m != nil {
			c.BorderSize, _ = strconv.Atoi(m[1])
		}
		if m := regexp.MustCompile(`^blur (enable|disable)`).FindStringSubmatch(line); m != nil {
			c.BlurEnabled = m[1] == "enable"
		}
		if m := regexp.MustCompile(`^blur_passes (\d+)`).FindStringSubmatch(line); m != nil {
			c.BlurPasses, _ = strconv.Atoi(m[1])
		}
		if m := regexp.MustCompile(`^blur_radius (\d+)`).FindStringSubmatch(line); m != nil {
			c.BlurRadius, _ = strconv.Atoi(m[1])
		}
		if m := regexp.MustCompile(`^default_dim_inactive ([0-9.]+)`).FindStringSubmatch(line); m != nil {
			c.DimInactive, _ = strconv.ParseFloat(m[1], 64)
		}
	}
	return nil
}

func (c *Config) loadOutputs() error {
	data, _, err := readOptional(OutputsPath())
	if err != nil {
		return err
	}
	re := regexp.MustCompile(`^output (\S+) resolution (\S+) position (\S+)(?:\s+transform\s+(\S+))?`)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		if m := re.FindStringSubmatch(line); m != nil {
			c.Outputs = append(c.Outputs, Output{
				Name:       m[1],
				Resolution: m[2],
				Position:   m[3],
				Transform:  m[4],
			})
		}
	}
	return nil
}

type replacement struct {
	re  *regexp.Regexp
	val string
}

// readOptional reads path, reporting exists=false (and no error) when the
// file does not exist. Any other error is returned.
func readOptional(path string) (data []byte, exists bool, err error) {
	data, err = os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// splitLines splits content into lines, dropping the empty element produced
// by a trailing newline.
func splitLines(content string) []string {
	if content == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(content, "\n"), "\n")
}

func leadingIndent(line string) string {
	return line[:len(line)-len(strings.TrimLeft(line, " \t"))]
}

// applyReplacements rewrites every non-comment line matching a replacement
// and returns the values of replacements that matched no line.
func applyReplacements(lines []string, replacements []replacement) []string {
	matched := make([]bool, len(replacements))
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		for j, r := range replacements {
			if r.re.MatchString(trimmed) {
				lines[i] = leadingIndent(line) + r.val
				matched[j] = true
				break
			}
		}
	}
	var missing []string
	for j, r := range replacements {
		if !matched[j] {
			missing = append(missing, r.val)
		}
	}
	return missing
}

// writeLines writes lines to path, creating parent directories as needed.
func writeLines(path string, lines []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

// replaceInFile updates matching lines in path, appending settings that are
// not present. A missing file is created.
func replaceInFile(path string, replacements []replacement) error {
	data, _, err := readOptional(path)
	if err != nil {
		return err
	}
	lines := splitLines(string(data))
	lines = append(lines, applyReplacements(lines, replacements)...)
	return writeLines(path, lines)
}

func (c *Config) Save() error {
	if err := c.saveMain(); err != nil {
		return fmt.Errorf("main config: %w", err)
	}
	if err := c.saveAppearance(); err != nil {
		return fmt.Errorf("appearance: %w", err)
	}
	if err := c.saveOutputs(); err != nil {
		return fmt.Errorf("outputs: %w", err)
	}
	return nil
}

func (c *Config) saveMain() error {
	path := MainConfigPath()
	data, exists, err := readOptional(path)
	if err != nil {
		return err
	}
	if !exists {
		// Seed from the system default so a new user config keeps sway's
		// default bindings instead of containing only our variables.
		data, _, err = readOptional(SystemConfigPath)
		if err != nil {
			return fmt.Errorf("reading system config: %w", err)
		}
	}
	lines := splitLines(string(data))
	missing := applyReplacements(lines, []replacement{
		{regexp.MustCompile(`^set \$mod `), fmt.Sprintf("set $mod %s", c.ModKey)},
		{regexp.MustCompile(`^set \$term `), fmt.Sprintf("set $term %s", c.Terminal)},
		{regexp.MustCompile(`^set \$menu `), fmt.Sprintf("set $menu %s", c.Menu)},
		{regexp.MustCompile(`^set \$opacity `), "set $opacity " + strconv.FormatFloat(math.Round(c.Opacity*100)/100, 'f', -1, 64)},
	})
	// Variables must be defined before use, so missing ones go at the top.
	lines = append(missing, lines...)
	if !exists {
		lines = append(lines, "", ConfigDInclude)
	}
	return writeLines(path, lines)
}

func (c *Config) saveAppearance() error {
	blurVal := "enable"
	if !c.BlurEnabled {
		blurVal = "disable"
	}
	common := []replacement{
		{regexp.MustCompile(`^gaps inner \d+`), fmt.Sprintf("gaps inner %d", c.GapsInner)},
		{regexp.MustCompile(`^gaps outer \d+`), fmt.Sprintf("gaps outer %d", c.GapsOuter)},
		{regexp.MustCompile(`^default_border pixel \d+`), fmt.Sprintf("default_border pixel %d", c.BorderSize)},
		{regexp.MustCompile(`^default_floating_border pixel \d+`), fmt.Sprintf("default_floating_border pixel %d", c.BorderSize)},
	}
	swayFX := []replacement{
		{regexp.MustCompile(`^corner_radius \d+`), fmt.Sprintf("corner_radius %d", c.CornerRadius)},
		{regexp.MustCompile(`^blur (enable|disable)$`), fmt.Sprintf("blur %s", blurVal)},
		{regexp.MustCompile(`^blur_passes \d+`), fmt.Sprintf("blur_passes %d", c.BlurPasses)},
		{regexp.MustCompile(`^blur_radius \d+`), fmt.Sprintf("blur_radius %d", c.BlurRadius)},
		{regexp.MustCompile(`^default_dim_inactive [0-9.]+`), fmt.Sprintf("default_dim_inactive %.2f", c.DimInactive)},
	}
	if c.SwayFX {
		return replaceInFile(AppearancePath(), append(common, swayFX...))
	}

	data, _, err := readOptional(AppearancePath())
	if err != nil {
		return err
	}
	var lines []string
	for _, line := range splitLines(string(data)) {
		if !matchesAny(strings.TrimSpace(line), swayFX) {
			lines = append(lines, line)
		}
	}
	lines = append(lines, applyReplacements(lines, common)...)
	return writeLines(AppearancePath(), lines)
}

func matchesAny(trimmed string, replacements []replacement) bool {
	for _, r := range replacements {
		if r.re.MatchString(trimmed) {
			return true
		}
	}
	return false
}

func (o Output) line() string {
	l := fmt.Sprintf("output %s resolution %s position %s", o.Name, o.Resolution, o.Position)
	if o.Transform != "" {
		l += " transform " + o.Transform
	}
	return l
}

func (c *Config) saveOutputs() error {
	data, _, err := readOptional(OutputsPath())
	if err != nil {
		return err
	}
	lines := splitLines(string(data))
	nameRe := regexp.MustCompile(`^output (\S+) resolution`)
	written := make(map[string]bool)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		m := nameRe.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}
		for _, out := range c.Outputs {
			if out.Name == m[1] {
				lines[i] = leadingIndent(line) + out.line()
				written[out.Name] = true
			}
		}
	}
	for _, out := range c.Outputs {
		if !written[out.Name] {
			lines = append(lines, out.line())
		}
	}
	return writeLines(OutputsPath(), lines)
}
