package ui

import (
	"image/color"
	"reflect"
	"testing"
)

func TestHexColorRoundTrip(t *testing.T) {
	c, err := hexToColor("#0000007F")
	if err != nil {
		t.Fatal(err)
	}
	if c != (color.NRGBA{0, 0, 0, 0x7f}) {
		t.Fatalf("hexToColor = %v", c)
	}
	if got := colorToHex(c, true); got != "#0000007F" {
		t.Fatalf("colorToHex alpha = %q", got)
	}
	c, err = hexToColor("#4c7899")
	if err != nil {
		t.Fatal(err)
	}
	if got := colorToHex(c, false); got != "#4C7899" {
		t.Fatalf("colorToHex opaque = %q", got)
	}
	// An opaque colour still gets alpha when the field requires it.
	if got := colorToHex(c, true); got != "#4C7899FF" {
		t.Fatalf("colorToHex forced alpha = %q", got)
	}
	// Translucent colours keep their alpha even where it's optional.
	if got := colorToHex(color.NRGBA{1, 2, 3, 4}, false); got != "#01020304" {
		t.Fatalf("colorToHex translucent = %q", got)
	}
	if _, err := hexToColor("red"); err == nil {
		t.Fatal("expected error for non-hex colour")
	}
}

func TestClientColorValues(t *testing.T) {
	// Unset: sway's defaults for the class.
	got := clientColorValues("", "client.focused")
	want := []string{"#4c7899", "#285577", "#ffffff", "#2e9ef4", "#285577"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("defaults = %v", got)
	}
	// Three values: indicator from the class default, child border = background.
	got = clientColorValues("#111111 #222222 #333333", "client.unfocused")
	want = []string{"#111111", "#222222", "#333333", "#292d2e", "#222222"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("partial = %v", got)
	}
}

func TestSetClientColorWritesAllFive(t *testing.T) {
	got := setClientColor("", "client.urgent", 1, "#AA0000")
	if got != "#2f343a #AA0000 #ffffff #900000 #900000" {
		t.Fatalf("setClientColor = %q", got)
	}
	got = setClientColor("#111111 #222222 #333333 #444444 #555555", "client.focused", 4, "#666666")
	if got != "#111111 #222222 #333333 #444444 #666666" {
		t.Fatalf("setClientColor = %q", got)
	}
}

func TestFontDescription(t *testing.T) {
	cases := []struct {
		desc                string
		family, style, size string
	}{
		{"pango:Noto Sans 10", "Noto Sans", "", "10"},
		{"pango:DejaVu Sans Mono Bold Italic 9.5", "DejaVu Sans Mono", "Bold Italic", "9.5"},
		{"monospace 12", "monospace", "", "12"},
		{"pango:Cantarell", "Cantarell", "", ""},
		{"", "", "", ""},
	}
	for _, c := range cases {
		f, s, z := parseFontDesc(c.desc)
		if f != c.family || s != c.style || z != c.size {
			t.Errorf("parseFontDesc(%q) = %q %q %q, want %q %q %q", c.desc, f, s, z, c.family, c.style, c.size)
		}
	}
	if got := formatFontDesc("DejaVu Sans Mono", "Bold", "9"); got != "pango:DejaVu Sans Mono Bold 9" {
		t.Fatalf("formatFontDesc = %q", got)
	}
	if got := formatFontDesc("Noto Sans", "", "10"); got != "pango:Noto Sans 10" {
		t.Fatalf("formatFontDesc = %q", got)
	}
}
