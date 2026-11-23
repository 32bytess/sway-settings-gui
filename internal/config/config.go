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

	ShadowsEnabled      bool
	ShadowsOnCSD        bool
	ShadowBlurRadius    int
	ShadowColor         string
	ShadowInactiveColor string
	ShadowOffsetX       int
	ShadowOffsetY       int

	SmartCornerRadius bool
	BlurXray          bool
	BlurNoise         float64
	BlurBrightness    float64
	BlurContrast      float64
	BlurSaturation    float64
	DimUnfocusedColor string
	DimUrgentColor    string
	TitlebarSeparator bool

	Outputs []Output

	// Wallpaper and WallpaperMode are `output * bg <file> <mode>`. The main
	// config's value is remembered so an unchanged wallpaper isn't copied
	// into outputs.conf.
	Wallpaper         string
	WallpaperMode     string
	mainWallpaper     string
	mainWallpaperMode string
	// savedWallpaper* are the values as last loaded or saved, to tell
	// whether the user changed the wallpaper.
	savedWallpaper     string
	savedWallpaperMode string

	// Input holds `input <type:...> <key> <value>` settings by identifier
	// and key; "" means not set. inputBaseline is the state as last loaded
	// or saved, so only changed settings are written.
	Input         map[string]map[string]string
	inputBaseline map[string]map[string]string

	// Behavior holds top-level look/behaviour commands (font, client.*,
	// focus_follows_mouse, ...) by command; "" means not set.
	Behavior         map[string]string
	behaviorBaseline map[string]string

	// Bindings are default-mode key bindings: the main config's (read-only)
	// with changes from bindings.conf applied. vars holds `set $name value`
	// from the main config, for comparing combos.
	Bindings      []BindingEntry
	bindingsExtra []string
	vars          map[string]string

	// Rules are for_window/assign rules from every config file; only those
	// in rules.conf are editable.
	Rules      []RuleEntry
	rulesExtra []string

	// Startup holds exec/exec_always lines from every config file; only
	// those in startup.conf are editable.
	Startup      []StartupEntry
	startupExtra []string

	// Bar is the swaybar managed in bar.conf.
	Bar Bar

	// Idle is swayidle/swaylock, managed via idle.conf and idle.sh.
	Idle Idle

	// Layers holds SwayFX layer_effects by namespace (layers.conf).
	Layers      []LayerEffect
	layersExtra []string

	// BorderStyle and FloatingBorderStyle are normal, pixel or none; ""
	// keeps whatever style the config already uses.
	BorderStyle         string
	FloatingBorderStyle string

	// LXQtDesktop is true when pcmanfm-qt draws the desktop over sway's
	// wallpaper; set by the caller (see LXQtDesktopRunning).
	LXQtDesktop bool

	// SwayFX reports whether the compositor accepts SwayFX-only commands
	// (corner_radius, blur*, shadow*, dim*, titlebar_separator, ...). When false they are
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

// Default returns a Config holding the app's defaults, before any file is read.
func Default() *Config {
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

		ShadowBlurRadius:    20,
		ShadowColor:         "#0000007F",
		ShadowInactiveColor: "#0000007F",

		SmartCornerRadius: true,
		BlurNoise:         0.02,
		BlurBrightness:    0.9,
		BlurContrast:      0.9,
		BlurSaturation:    1.1,
		DimUnfocusedColor: "#000000FF",
		DimUrgentColor:    "#900000FF",
		TitlebarSeparator: true,
	}
	cfg.Input = newInputMaps()
	cfg.Behavior = map[string]string{}
	cfg.vars = map[string]string{}
	cfg.Bar = newBar()
	cfg.Idle = defaultIdle()
	return cfg
}

func Load() (*Config, error) {
	cfg := Default()
	if err := cfg.loadMain(); err != nil {
		return nil, fmt.Errorf("main config: %w", err)
	}
	if err := cfg.loadAppearance(); err != nil {
		return nil, fmt.Errorf("appearance: %w", err)
	}
	if err := cfg.loadOutputs(); err != nil {
		return nil, fmt.Errorf("outputs: %w", err)
	}
	if err := cfg.loadInputs(); err != nil {
		return nil, fmt.Errorf("input: %w", err)
	}
	if err := cfg.loadBehavior(); err != nil {
		return nil, fmt.Errorf("behavior: %w", err)
	}
	cfg.behaviorBaseline = copyStringMap(cfg.Behavior)
	if err := cfg.loadBindings(); err != nil {
		return nil, fmt.Errorf("bindings: %w", err)
	}
	if err := cfg.loadRules(); err != nil {
		return nil, fmt.Errorf("rules: %w", err)
	}
	if err := cfg.loadStartup(); err != nil {
		return nil, fmt.Errorf("startup: %w", err)
	}
	if err := cfg.loadBar(); err != nil {
		return nil, fmt.Errorf("bar: %w", err)
	}
	if err := cfg.loadIdle(); err != nil {
		return nil, fmt.Errorf("idle: %w", err)
	}
	if err := cfg.loadLayers(); err != nil {
		return nil, fmt.Errorf("layers: %w", err)
	}
	cfg.markWallpaperSaved()
	cfg.inputBaseline = copyInputMaps(cfg.Input)
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
		// Only the wallpaper is read from the main config; per-output
		// settings there are left to the user.
		if ol, ok := parseOutputLine(line); ok && ol.name == "*" {
			for _, seg := range ol.segments {
				if canonicalKey(seg.key) == "bg" && len(seg.args) >= 2 {
					c.Wallpaper, c.WallpaperMode = unquote(seg.args[0]), unquote(seg.args[1])
					c.mainWallpaper, c.mainWallpaperMode = c.Wallpaper, c.WallpaperMode
				}
			}
		}
	}
	c.readInputs(splitLines(string(data)))
	c.readBehavior(splitLines(string(data)))
	c.readMainBindings(splitLines(string(data)))
	c.readRules(splitLines(string(data)), MainConfigPath())
	c.readStartup(splitLines(string(data)), MainConfigPath())
	c.Bar.OtherBars += countBars(splitLines(string(data)))
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
		if m := regexp.MustCompile(`^default_border (normal|pixel|none)\b`).FindStringSubmatch(line); m != nil {
			c.BorderStyle = m[1]
		}
		if m := regexp.MustCompile(`^default_floating_border (normal|pixel|none)\b`).FindStringSubmatch(line); m != nil {
			c.FloatingBorderStyle = m[1]
		}
		if m := regexp.MustCompile(`^default_border (?:normal|pixel) (\d+)`).FindStringSubmatch(line); m != nil {
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
		if m := regexp.MustCompile(`^shadows (enable|disable)$`).FindStringSubmatch(line); m != nil {
			c.ShadowsEnabled = m[1] == "enable"
		}
		if m := regexp.MustCompile(`^shadows_on_csd (enable|disable)$`).FindStringSubmatch(line); m != nil {
			c.ShadowsOnCSD = m[1] == "enable"
		}
		if m := regexp.MustCompile(`^shadow_blur_radius (\d+)`).FindStringSubmatch(line); m != nil {
			c.ShadowBlurRadius, _ = strconv.Atoi(m[1])
		}
		if m := regexp.MustCompile(`^shadow_color (#[0-9A-Fa-f]+)`).FindStringSubmatch(line); m != nil {
			c.ShadowColor = m[1]
		}
		if m := regexp.MustCompile(`^shadow_inactive_color (#[0-9A-Fa-f]+)`).FindStringSubmatch(line); m != nil {
			c.ShadowInactiveColor = m[1]
		}
		for name, dst := range map[string]*bool{
			"smart_corner_radius": &c.SmartCornerRadius,
			"blur_xray":           &c.BlurXray,
			"titlebar_separator":  &c.TitlebarSeparator,
		} {
			if m := regexp.MustCompile(`^` + name + ` (enable|disable)$`).FindStringSubmatch(line); m != nil {
				*dst = m[1] == "enable"
			}
		}
		for name, dst := range map[string]*float64{
			"blur_noise":      &c.BlurNoise,
			"blur_brightness": &c.BlurBrightness,
			"blur_contrast":   &c.BlurContrast,
			"blur_saturation": &c.BlurSaturation,
		} {
			if m := regexp.MustCompile(`^` + name + ` ([0-9.]+)`).FindStringSubmatch(line); m != nil {
				*dst, _ = strconv.ParseFloat(m[1], 64)
			}
		}
		for name, dst := range map[string]*string{
			`dim_inactive_colors\.unfocused`: &c.DimUnfocusedColor,
			`dim_inactive_colors\.urgent`:    &c.DimUrgentColor,
		} {
			if m := regexp.MustCompile(`^` + name + ` (#[0-9A-Fa-f]+)`).FindStringSubmatch(line); m != nil {
				*dst = m[1]
			}
		}
		if m := regexp.MustCompile(`^shadow_offset (-?\d+) (-?\d+)`).FindStringSubmatch(line); m != nil {
			c.ShadowOffsetX, _ = strconv.Atoi(m[1])
			c.ShadowOffsetY, _ = strconv.Atoi(m[2])
		}
	}
	return nil
}

type replacement struct {
	re  *regexp.Regexp
	val string // the line to write; also appended when no line matches
	// rewrite, when set, computes the new line from the matched one instead
	// of using val, so parts the app doesn't manage can be preserved.
	rewrite func(trimmed string) string
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
	content = strings.TrimSuffix(content, "\n")
	if content == "" {
		return nil
	}
	return strings.Split(content, "\n")
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
				val := r.val
				if r.rewrite != nil {
					val = r.rewrite(trimmed)
				}
				lines[i] = leadingIndent(line) + val
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
	content := ""
	if len(lines) > 0 {
		content = strings.Join(lines, "\n") + "\n"
	}
	return os.WriteFile(path, []byte(content), 0644)
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

// OpacityRule applies $opacity to every window. Without it, setting
// $opacity has no effect.
const OpacityRule = "for_window [all] opacity $opacity"

var hexColorRe = regexp.MustCompile(`^#[0-9A-Fa-f]{6}([0-9A-Fa-f]{2})?$`)

// ValidHexColor reports whether s is a #RRGGBB or #RRGGBBAA color.
func ValidHexColor(s string) bool { return hexColorRe.MatchString(s) }

func (c *Config) validate() error {
	for name, v := range map[string]string{"modifier key": c.ModKey, "terminal": c.Terminal, "menu": c.Menu} {
		if hasControlChars(v) {
			return fmt.Errorf("%s contains control characters (e.g. a line break)", name)
		}
	}
	if err := c.validateOutputs(); err != nil {
		return err
	}
	if err := c.validateInputs(); err != nil {
		return err
	}
	if err := c.validateBehavior(); err != nil {
		return err
	}
	if err := c.validateBindings(); err != nil {
		return err
	}
	if err := c.validateRules(); err != nil {
		return err
	}
	if err := c.validateStartupEntries(); err != nil {
		return err
	}
	if err := c.validateBar(); err != nil {
		return err
	}
	if err := c.validateIdle(); err != nil {
		return err
	}
	if err := c.validateLayers(); err != nil {
		return err
	}
	if !c.SwayFX {
		return nil
	}
	for name, v := range map[string]string{
		"shadow color":          c.ShadowColor,
		"inactive shadow color": c.ShadowInactiveColor,
		"inactive dim color":    c.DimUnfocusedColor,
		"urgent dim color":      c.DimUrgentColor,
	} {
		if !ValidHexColor(v) {
			return fmt.Errorf("%s %q must be #RRGGBB or #RRGGBBAA", name, v)
		}
	}
	return nil
}

func (c *Config) Save() error {
	if err := c.validate(); err != nil {
		return err
	}
	if err := c.saveMain(); err != nil {
		return fmt.Errorf("main config: %w", err)
	}
	if err := c.saveAppearance(); err != nil {
		return fmt.Errorf("appearance: %w", err)
	}
	if err := c.saveOutputs(); err != nil {
		return fmt.Errorf("outputs: %w", err)
	}
	if err := c.saveInputs(); err != nil {
		return fmt.Errorf("input: %w", err)
	}
	if err := c.saveBehavior(); err != nil {
		return fmt.Errorf("behavior: %w", err)
	}
	c.behaviorBaseline = copyStringMap(c.Behavior)
	if err := c.saveBindings(); err != nil {
		return fmt.Errorf("bindings: %w", err)
	}
	if err := c.saveRules(); err != nil {
		return fmt.Errorf("rules: %w", err)
	}
	if err := c.saveStartup(); err != nil {
		return fmt.Errorf("startup: %w", err)
	}
	if err := c.saveBar(); err != nil {
		return fmt.Errorf("bar: %w", err)
	}
	if err := c.saveIdle(); err != nil {
		return fmt.Errorf("idle: %w", err)
	}
	if err := c.saveLayers(); err != nil {
		return fmt.Errorf("layers: %w", err)
	}
	c.markWallpaperSaved()
	c.inputBaseline = copyInputMaps(c.Input)
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
		{regexp.MustCompile(`^set \$mod `), fmt.Sprintf("set $mod %s", c.ModKey), nil},
		{regexp.MustCompile(`^set \$term `), fmt.Sprintf("set $term %s", c.Terminal), nil},
		{regexp.MustCompile(`^set \$menu `), fmt.Sprintf("set $menu %s", c.Menu), nil},
		{regexp.MustCompile(`^set \$opacity `), "set $opacity " + formatFloat(c.Opacity), nil},
	})
	// Variables must be defined before use, so missing ones go at the top.
	lines = append(missing, lines...)
	lines = ensureOpacityRule(lines)
	if !exists {
		lines = append(lines, "", ConfigDInclude)
	}
	return writeLines(path, lines)
}

// borderReplacement writes a default_border-style command. style is normal,
// pixel or none; "" keeps the style already in the file (styles the app
// doesn't manage, like csd, are left untouched). When the command is absent
// it is added, as pixel unless a style was chosen.
func borderReplacement(cmd, style string, size int) replacement {
	line := func(st string) string {
		if st == "none" {
			return cmd + " none"
		}
		return fmt.Sprintf("%s %s %d", cmd, st, size)
	}
	val := line("pixel")
	if style != "" {
		val = line(style)
	}
	return replacement{
		re:  regexp.MustCompile(`^` + cmd + `(\s|$)`),
		val: val,
		rewrite: func(trimmed string) string {
			fields := strings.Fields(trimmed)
			if len(fields) < 2 {
				return trimmed
			}
			st := style
			if st == "" {
				st = fields[1]
			}
			switch st {
			case "normal", "pixel", "none":
				return line(st)
			}
			return trimmed
		},
	}
}

// ensureOpacityRule inserts OpacityRule right after the $opacity definition
// unless an active (uncommented) copy already exists.
func ensureOpacityRule(lines []string) []string {
	setAt := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == OpacityRule {
			return lines
		}
		if setAt < 0 && strings.HasPrefix(trimmed, "set $opacity ") {
			setAt = i
		}
	}
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:setAt+1]...)
	out = append(out, OpacityRule)
	return append(out, lines[setAt+1:]...)
}

// formatFloat rounds to two decimals and drops trailing zeros.
func formatFloat(f float64) string {
	return strconv.FormatFloat(math.Round(f*100)/100, 'f', -1, 64)
}

func enableDisable(b bool) string {
	if b {
		return "enable"
	}
	return "disable"
}

func (c *Config) saveAppearance() error {
	blurVal := "enable"
	if !c.BlurEnabled {
		blurVal = "disable"
	}
	common := []replacement{
		{regexp.MustCompile(`^gaps inner \d+`), fmt.Sprintf("gaps inner %d", c.GapsInner), nil},
		{regexp.MustCompile(`^gaps outer \d+`), fmt.Sprintf("gaps outer %d", c.GapsOuter), nil},
		borderReplacement("default_border", c.BorderStyle, c.BorderSize),
		borderReplacement("default_floating_border", c.FloatingBorderStyle, c.BorderSize),
	}
	swayFX := []replacement{
		{regexp.MustCompile(`^corner_radius \d+`), fmt.Sprintf("corner_radius %d", c.CornerRadius), nil},
		{regexp.MustCompile(`^blur (enable|disable)$`), fmt.Sprintf("blur %s", blurVal), nil},
		{regexp.MustCompile(`^blur_passes \d+`), fmt.Sprintf("blur_passes %d", c.BlurPasses), nil},
		{regexp.MustCompile(`^blur_radius \d+`), fmt.Sprintf("blur_radius %d", c.BlurRadius), nil},
		{regexp.MustCompile(`^default_dim_inactive [0-9.]+`), fmt.Sprintf("default_dim_inactive %.2f", c.DimInactive), nil},
		{regexp.MustCompile(`^shadows (enable|disable)$`), "shadows " + enableDisable(c.ShadowsEnabled), nil},
		{regexp.MustCompile(`^shadows_on_csd (enable|disable)$`), "shadows_on_csd " + enableDisable(c.ShadowsOnCSD), nil},
		{regexp.MustCompile(`^shadow_blur_radius `), fmt.Sprintf("shadow_blur_radius %d", c.ShadowBlurRadius), nil},
		{regexp.MustCompile(`^shadow_color `), "shadow_color " + c.ShadowColor, nil},
		{regexp.MustCompile(`^shadow_inactive_color `), "shadow_inactive_color " + c.ShadowInactiveColor, nil},
		{regexp.MustCompile(`^shadow_offset `), fmt.Sprintf("shadow_offset %d %d", c.ShadowOffsetX, c.ShadowOffsetY), nil},
		{regexp.MustCompile(`^smart_corner_radius (enable|disable)$`), "smart_corner_radius " + enableDisable(c.SmartCornerRadius), nil},
		{regexp.MustCompile(`^blur_xray (enable|disable)$`), "blur_xray " + enableDisable(c.BlurXray), nil},
		{regexp.MustCompile(`^blur_noise `), "blur_noise " + formatFloat(c.BlurNoise), nil},
		{regexp.MustCompile(`^blur_brightness `), "blur_brightness " + formatFloat(c.BlurBrightness), nil},
		{regexp.MustCompile(`^blur_contrast `), "blur_contrast " + formatFloat(c.BlurContrast), nil},
		{regexp.MustCompile(`^blur_saturation `), "blur_saturation " + formatFloat(c.BlurSaturation), nil},
		{regexp.MustCompile(`^dim_inactive_colors\.unfocused `), "dim_inactive_colors.unfocused " + c.DimUnfocusedColor, nil},
		{regexp.MustCompile(`^dim_inactive_colors\.urgent `), "dim_inactive_colors.urgent " + c.DimUrgentColor, nil},
		{regexp.MustCompile(`^titlebar_separator (enable|disable)$`), "titlebar_separator " + enableDisable(c.TitlebarSeparator), nil},
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
