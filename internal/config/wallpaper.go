package config

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// svgLoaderGlobs locate an SVG image loader for gdk-pixbuf, which swaybg
// uses: either a glycin loader (gdk-pixbuf >= 2.44) or a classic pixbuf
// loader module from librsvg. A variable so tests can redirect it.
var svgLoaderGlobs = []string{
	"/usr/share/glycin-loaders/*/conf.d/*svg*",
	"/usr/lib/gdk-pixbuf-2.0/*/loaders/*svg*",
	"/usr/lib64/gdk-pixbuf-2.0/*/loaders/*svg*",
}

// WallpaperImageExts lists file extensions offered in the wallpaper picker.
// SVG is included only when an SVG loader is installed.
func WallpaperImageExts() []string {
	exts := []string{".png", ".jpg", ".jpeg", ".webp", ".bmp", ".gif"}
	for _, g := range svgLoaderGlobs {
		if matches, _ := filepath.Glob(g); len(matches) > 0 {
			return append(exts, ".svg", ".svgz")
		}
	}
	return exts
}

// desktopRunning reports whether LXQt's desktop (pcmanfm-qt --desktop) is
// running. A variable so tests can replace it.
var desktopRunning = func() (bool, error) {
	err := exec.Command("pgrep", "-f", `pcmanfm-qt.*--desktop`).Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil // no matching process
	}
	if err != nil {
		return false, fmt.Errorf("pgrep: %w", err)
	}
	return true, nil
}

// LXQtDesktopRunning reports whether pcmanfm-qt is drawing the desktop, in
// which case it covers the wallpaper drawn by sway (swaybg).
func LXQtDesktopRunning() (bool, error) { return desktopRunning() }

// PcmanfmWallpaperMode maps a sway bg mode to pcmanfm-qt's --wallpaper-mode.
func PcmanfmWallpaperMode(swayMode string) (string, error) {
	switch swayMode {
	case "fill":
		return "zoom", nil
	case "fit", "stretch", "center", "tile":
		return swayMode, nil
	}
	return "", fmt.Errorf("wallpaper mode %q can't be applied to the LXQt desktop", swayMode)
}

// runPcmanfm runs pcmanfm-qt with args (no shell). A variable so tests can
// replace it.
var runPcmanfm = func(args ...string) error {
	out, err := exec.Command("pcmanfm-qt", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("pcmanfm-qt %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// SyncLXQtWallpaper sets the LXQt desktop wallpaper through the running
// pcmanfm-qt, which applies it immediately and saves it in its own settings.
func SyncLXQtWallpaper(path, swayMode string) error {
	mode, err := PcmanfmWallpaperMode(swayMode)
	if err != nil {
		return err
	}
	return runPcmanfm("--set-wallpaper", path, "--wallpaper-mode", mode)
}

// WallpaperChanged reports whether the wallpaper differs from what was last
// loaded or saved.
func (c *Config) WallpaperChanged() bool {
	return c.Wallpaper != c.savedWallpaper || c.WallpaperMode != c.savedWallpaperMode
}

func (c *Config) markWallpaperSaved() {
	c.savedWallpaper, c.savedWallpaperMode = c.Wallpaper, c.WallpaperMode
}
