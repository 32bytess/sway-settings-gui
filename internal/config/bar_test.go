package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeBarConf(t *testing.T, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(BarPath()), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(BarPath(), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

const sampleBarConf = `bar {
    id sway-settings
    position top
    font pango:Noto Sans 10
    status_command while date +'%Y-%m-%d %X'; do sleep 1; done
    workspace_buttons yes
    bindsym button4 nop
    colors {
        background #000000
        focused_workspace #4c7899 #285577 #ffffff
    }
}
`

func TestBarDisabledByDefaultWritesNothing(t *testing.T) {
	setupHome(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Bar.Enabled {
		t.Fatal("bar should be disabled when bar.conf has no bar block")
	}
	cfg.Bar.Settings["position"] = "bottom" // settings alone don't enable it
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, BarPath()); got != "" {
		t.Fatalf("bar.conf = %q, want empty while disabled", got)
	}
}

func TestBarLoadAndRoundTrip(t *testing.T) {
	setupHome(t)
	writeBarConf(t, sampleBarConf)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	b := cfg.Bar
	if !b.Enabled || b.Settings["position"] != "top" || b.Settings["font"] != "pango:Noto Sans 10" ||
		b.Settings["status_command"] != "while date +'%Y-%m-%d %X'; do sleep 1; done" ||
		b.Colors["background"] != "#000000" || b.Colors["focused_workspace"] != "#4c7899 #285577 #ffffff" {
		t.Fatalf("bar = %+v", b)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, BarPath()); got != sampleBarConf {
		t.Fatalf("round trip changed bar.conf:\n%s\nwant\n%s", got, sampleBarConf)
	}
}

func TestBarEnableWritesBlockAndDisableRemovesIt(t *testing.T) {
	setupHome(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Bar.Enabled = true
	cfg.Bar.Settings["position"] = "bottom"
	cfg.Bar.Settings["tray_output"] = "none"
	cfg.Bar.Colors["statusline"] = "#FFFFFF"
	cfg.Bar.Colors["urgent_workspace"] = "#2f343a #900000 #ffffff"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	want := `bar {
    id sway-settings
    position bottom
    tray_output none
    colors {
        statusline #FFFFFF
        urgent_workspace #2f343a #900000 #ffffff
    }
}
`
	if got := readFile(t, BarPath()); got != want {
		t.Fatalf("bar.conf =\n%s\nwant\n%s", got, want)
	}
	cfg.Bar.Enabled = false
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, BarPath()); got != "" {
		t.Fatalf("disabled bar still written: %q", got)
	}
}

func TestBarCountsMainConfigBars(t *testing.T) {
	setupHome(t)
	writeMain(t, "bar {\n    position top\n}\nbar other {\n    mode hide\n}\n")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Bar.OtherBars != 2 {
		t.Fatalf("OtherBars = %d, want 2", cfg.Bar.OtherBars)
	}
}

func TestBarValidation(t *testing.T) {
	cases := []struct{ settings, colors map[string]string }{
		{map[string]string{"position": "left"}, nil},
		{map[string]string{"height": "-3"}, nil},
		{map[string]string{"status_command": "date\nexec evil"}, nil},
		{map[string]string{"gaps": "big"}, nil},
		{nil, map[string]string{"background": "black"}},
		{nil, map[string]string{"focused_workspace": "#000000 #111111"}},
	}
	for _, c := range cases {
		setupHome(t)
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		cfg.Bar.Enabled = true
		for k, v := range c.settings {
			cfg.Bar.Settings[k] = v
		}
		for k, v := range c.colors {
			cfg.Bar.Colors[k] = v
		}
		if err := cfg.Save(); err == nil {
			t.Errorf("expected error for %v %v", c.settings, c.colors)
		}
	}
}
