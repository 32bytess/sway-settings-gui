package main

import (
	"encoding/xml"
	"os"
	"strings"
	"testing"
)

func readDesktopFile(t *testing.T) map[string]string {
	t.Helper()
	data, err := os.ReadFile("assets/" + appID + ".desktop")
	if err != nil {
		t.Fatalf("desktop file must be named after the app ID so taskbars match the window: %v", err)
	}
	entries := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			entries[k] = v
		}
	}
	return entries
}

func TestDesktopFile(t *testing.T) {
	e := readDesktopFile(t)
	want := map[string]string{
		"Type":           "Application",
		"Name":           "Sway Settings",
		"Exec":           "sway-settings",
		"Icon":           "sway-settings",
		"Terminal":       "false",
		"StartupWMClass": appID,
	}
	for k, v := range want {
		if e[k] != v {
			t.Errorf("%s = %q, want %q", k, e[k], v)
		}
	}
	cats := ";" + e["Categories"]
	for _, c := range []string{"Settings", "DesktopSettings"} {
		if !strings.Contains(cats, ";"+c+";") {
			t.Errorf("Categories %q must include %s (LXQt's Preferences menu and other DEs' settings sections)", e["Categories"], c)
		}
	}
}

func TestEmbeddedIconIsSVG(t *testing.T) {
	if len(iconSVG) == 0 {
		t.Fatal("icon not embedded")
	}
	var root struct {
		XMLName xml.Name
	}
	if err := xml.Unmarshal(iconSVG, &root); err != nil || root.XMLName.Local != "svg" {
		t.Fatalf("embedded icon is not SVG: %v (root %q)", err, root.XMLName.Local)
	}
}
