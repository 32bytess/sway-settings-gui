package ui

import "testing"

func TestSplitAndJoinCombo(t *testing.T) {
	cases := []struct {
		combo string
		mods  []string
		key   string
		out   string
	}{
		{"$mod+Shift+e", []string{"$mod", "Shift"}, "e", "$mod+Shift+e"},
		{"Shift+$mod+q", []string{"$mod", "Shift"}, "q", "$mod+Shift+q"},
		{"Mod4+Control+Left", []string{"Super", "Ctrl"}, "Left", "Mod4+Ctrl+Left"},
		{"Mod1+space", []string{"Alt"}, "space", "Alt+space"},
		{"XF86AudioMute", nil, "XF86AudioMute", "XF86AudioMute"},
		{"Mod3+$mod+x", []string{"$mod"}, "Mod3+x", "$mod+Mod3+x"},
	}
	for _, c := range cases {
		mods, key := splitCombo(c.combo)
		for _, m := range c.mods {
			if !mods[m] {
				t.Errorf("splitCombo(%q) missing modifier %s: %v", c.combo, m, mods)
			}
		}
		if len(mods) != len(c.mods) || key != c.key {
			t.Errorf("splitCombo(%q) = %v %q, want %v %q", c.combo, mods, key, c.mods, c.key)
		}
		if got := joinCombo(mods, key); got != c.out {
			t.Errorf("joinCombo(%v, %q) = %q, want %q", mods, key, got, c.out)
		}
	}
}
