package ui

import "testing"

func TestReplaceProgram(t *testing.T) {
	cases := []struct{ cmd, path, want string }{
		{"", "/usr/bin/foot", "/usr/bin/foot"},
		{"kitty", "/usr/bin/foot", "/usr/bin/foot"},
		{"wofi --show drun", "/usr/bin/fuzzel", "/usr/bin/fuzzel --show drun"},
		{"  lxqt-runner  ", "/usr/bin/rofi", "/usr/bin/rofi"},
		{"kitty -1", "/opt/My Apps/term", "'/opt/My Apps/term' -1"},
	}
	for _, c := range cases {
		if got := replaceProgram(c.cmd, c.path); got != c.want {
			t.Errorf("replaceProgram(%q, %q) = %q, want %q", c.cmd, c.path, got, c.want)
		}
	}
}
