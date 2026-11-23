package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPcmanfmWallpaperMode(t *testing.T) {
	want := map[string]string{"fill": "zoom", "fit": "fit", "stretch": "stretch", "center": "center", "tile": "tile"}
	for sway, pcman := range want {
		got, err := PcmanfmWallpaperMode(sway)
		if err != nil || got != pcman {
			t.Errorf("PcmanfmWallpaperMode(%q) = %q, %v; want %q", sway, got, err, pcman)
		}
	}
	if _, err := PcmanfmWallpaperMode("solid_color"); err == nil {
		t.Error("expected error for solid_color")
	}
}

func TestSyncLXQtWallpaperRunsPcmanfm(t *testing.T) {
	var gotArgs []string
	orig := runPcmanfm
	runPcmanfm = func(args ...string) error { gotArgs = args; return nil }
	t.Cleanup(func() { runPcmanfm = orig })

	if err := SyncLXQtWallpaper("/home/me/My Pictures/a.svg", "fill"); err != nil {
		t.Fatal(err)
	}
	want := []string{"--set-wallpaper", "/home/me/My Pictures/a.svg", "--wallpaper-mode", "zoom"}
	if !reflect.DeepEqual(gotArgs, want) {
		t.Fatalf("args = %q, want %q", gotArgs, want)
	}

	runPcmanfm = func(args ...string) error { return errors.New("boom") }
	if err := SyncLXQtWallpaper("/a.png", "fit"); err == nil {
		t.Fatal("expected pcmanfm-qt error to be returned")
	}
}

func TestLXQtDesktopRunningSurfacesErrors(t *testing.T) {
	orig := desktopRunning
	t.Cleanup(func() { desktopRunning = orig })

	desktopRunning = func() (bool, error) { return true, nil }
	if ok, err := LXQtDesktopRunning(); !ok || err != nil {
		t.Fatalf("got %v, %v", ok, err)
	}
	desktopRunning = func() (bool, error) { return false, errors.New("pgrep missing") }
	if _, err := LXQtDesktopRunning(); err == nil {
		t.Fatal("expected error")
	}
}

func TestWallpaperImageExtsDependsOnSVGLoader(t *testing.T) {
	dir := t.TempDir()
	orig := svgLoaderGlobs
	svgLoaderGlobs = []string{filepath.Join(dir, "*svg*")}
	t.Cleanup(func() { svgLoaderGlobs = orig })

	for _, e := range WallpaperImageExts() {
		if e == ".svg" {
			t.Fatal(".svg offered without an SVG loader")
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "glycin-svg.conf"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range WallpaperImageExts() {
		found = found || e == ".svg"
	}
	if !found {
		t.Fatal(".svg not offered although an SVG loader exists")
	}
}

func TestWallpaperChangedTracksLoadAndSave(t *testing.T) {
	setupHome(t)
	writeOutputsConf(t, "output * bg /a.png fill\n")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WallpaperChanged() {
		t.Fatal("freshly loaded wallpaper reported as changed")
	}
	cfg.WallpaperMode = "fit"
	if !cfg.WallpaperChanged() {
		t.Fatal("mode change not detected")
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if cfg.WallpaperChanged() {
		t.Fatal("wallpaper still reported changed after save")
	}
}
