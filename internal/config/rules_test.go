package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const realRulesMain = `for_window [all] opacity $opacity
    for_window [app_id="^lxqt-.*$"] floating enable
    for_window [app_id="cmst"] floating enable
bar {
    for_window [app_id="ignored"] floating enable
}
assign [class="Steam"] → workspace number 9
`

const realRulesAppearance = `gaps inner 4
for_window [floating] title_format "[F] %title"
for_window [tiling] title_format "%title"
`

func loadRules(t *testing.T) *Config {
	t.Helper()
	setupHome(t)
	writeMain(t, realRulesMain)
	if err := os.MkdirAll(filepath.Dir(AppearancePath()), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(AppearancePath(), []byte(realRulesAppearance), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestParseRuleLine(t *testing.T) {
	cases := []struct {
		line string
		want WindowRule
	}{
		{`for_window [app_id="^lxqt-.*$"] floating enable`, WindowRule{Kind: "for_window", Criteria: `app_id="^lxqt-.*$"`, Command: "floating enable"}},
		{`for_window [title="\[x\] ]"] opacity 0.8`, WindowRule{Kind: "for_window", Criteria: `title="\[x\] ]"`, Command: "opacity 0.8"}},
		{`assign [class="Steam"] → workspace number 9`, WindowRule{Kind: "assign", Criteria: `class="Steam"`, Command: "workspace number 9"}},
		{`assign [app_id="foot"] output DP-1`, WindowRule{Kind: "assign", Criteria: `app_id="foot"`, Command: "output DP-1"}},
	}
	for _, c := range cases {
		got, ok := parseRuleLine(c.line)
		if !ok || got != c.want {
			t.Errorf("parseRuleLine(%q) = %+v, %v; want %+v", c.line, got, ok, c.want)
		}
	}
	for _, bad := range []string{`for_window app_id=foo floating enable`, `for_window [app_id="x"`, `gaps inner 4`} {
		if _, ok := parseRuleLine(bad); ok {
			t.Errorf("parseRuleLine(%q) should fail", bad)
		}
	}
}

func TestLoadShowsExistingRulesReadOnly(t *testing.T) {
	cfg := loadRules(t)
	var got []string
	for _, r := range cfg.Rules {
		if r.Editable() {
			t.Fatalf("existing rule %+v must be read-only", r)
		}
		got = append(got, r.Criteria)
	}
	want := []string{"all", `app_id="^lxqt-.*$"`, `app_id="cmst"`, `class="Steam"`, "floating", "tiling"}
	if len(got) != len(want) {
		t.Fatalf("rules = %q, want %q (bar block must be skipped)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("rules = %q, want %q", got, want)
		}
	}
	if cfg.Rules[0].Source != MainConfigPath() || cfg.Rules[4].Source != AppearancePath() {
		t.Fatalf("sources = %q, %q", cfg.Rules[0].Source, cfg.Rules[4].Source)
	}
}

func TestSaveWithoutRuleChangesLeavesRulesFileEmpty(t *testing.T) {
	cfg := loadRules(t)
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, RulesPath()); got != "" {
		t.Fatalf("rules.conf = %q", got)
	}
}

func TestAddEditDeleteRules(t *testing.T) {
	cfg := loadRules(t)
	if err := cfg.AddRule(WindowRule{Kind: "for_window", Criteria: `app_id="foot"`, Command: "floating enable"}); err != nil {
		t.Fatal(err)
	}
	if err := cfg.AddRule(WindowRule{Kind: "assign", Criteria: `app_id="firefox"`, Command: "workspace number 2"}); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	want := "for_window [app_id=\"foot\"] floating enable\nassign [app_id=\"firefox\"] workspace number 2\n"
	if got := readFile(t, RulesPath()); got != want {
		t.Fatalf("rules.conf =\n%s\nwant\n%s", got, want)
	}

	again, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	var foot = -1
	for i, r := range again.Rules {
		if r.Criteria == `app_id="foot"` {
			foot = i
		}
	}
	if foot < 0 || !again.Rules[foot].Editable() {
		t.Fatalf("app rule not loaded as editable: %+v", again.Rules)
	}
	if err := again.UpdateRule(foot, WindowRule{Kind: "for_window", Criteria: `app_id="foot"`, Command: "opacity 0.9"}); err != nil {
		t.Fatal(err)
	}
	if err := again.DeleteRule(foot + 1); err != nil {
		t.Fatal(err)
	}
	if err := again.Save(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, RulesPath()); got != "for_window [app_id=\"foot\"] opacity 0.9\n" {
		t.Fatalf("rules.conf = %q", got)
	}
}

func TestReadOnlyRulesCannotBeChanged(t *testing.T) {
	cfg := loadRules(t)
	if err := cfg.UpdateRule(1, WindowRule{Kind: "for_window", Criteria: "all", Command: "kill"}); err == nil {
		t.Fatal("expected error updating a main-config rule")
	}
	if err := cfg.DeleteRule(1); err == nil {
		t.Fatal("expected error deleting a main-config rule")
	}
}

func TestRulesFileKeepsOtherLines(t *testing.T) {
	setupHome(t)
	if err := os.MkdirAll(filepath.Dir(RulesPath()), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(RulesPath(), []byte("# my rules\nfor_window [app_id=\"a\"] floating enable\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, RulesPath()); got != "# my rules\nfor_window [app_id=\"a\"] floating enable\n" {
		t.Fatalf("rules.conf = %q", got)
	}
}

func TestRuleValidation(t *testing.T) {
	cfg := loadRules(t)
	bad := []WindowRule{
		{Kind: "for_window", Criteria: "", Command: "floating enable"},
		{Kind: "for_window", Criteria: `app_id="x`, Command: "floating enable"},
		{Kind: "for_window", Criteria: `app_id="x"] kill [`, Command: "floating enable"},
		{Kind: "for_window", Criteria: `bogus="x"`, Command: "floating enable"},
		{Kind: "for_window", Criteria: `app_id="x"`, Command: ""},
		{Kind: "for_window", Criteria: `app_id="x"`, Command: "floating enable\nexec evil"},
		{Kind: "assign", Criteria: `app_id="x"`, Command: "floating enable"},
		{Kind: "nope", Criteria: `app_id="x"`, Command: "kill"},
	}
	for _, r := range bad {
		if err := cfg.AddRule(r); err == nil {
			t.Errorf("expected error for %+v", r)
		}
	}
	good := []WindowRule{
		{Kind: "for_window", Criteria: `app_id="^lxqt-.*$" title="Settings"`, Command: "floating enable, resize set 800 600"},
		{Kind: "for_window", Criteria: "floating", Command: "border normal"},
		{Kind: "assign", Criteria: `class="Steam"`, Command: "output HDMI-A-1"},
		{Kind: "assign", Criteria: `class="Steam"`, Command: "workspace 9:games"},
	}
	for _, r := range good {
		if err := cfg.AddRule(r); err != nil {
			t.Errorf("valid rule %+v rejected: %v", r, err)
		}
	}
}

func TestOpenWindows(t *testing.T) {
	orig := swayTree
	t.Cleanup(func() { swayTree = orig })
	swayTree = func() ([]byte, error) {
		return []byte(`{"nodes":[{"nodes":[{"pid":1,"app_id":"foot","name":"shell","shell":"xdg_shell"}],
			"floating_nodes":[{"pid":2,"app_id":null,"name":"Steam","shell":"xwayland","window_properties":{"class":"Steam","instance":"steam"}}]}]}`), nil
	}
	wins, err := OpenWindows()
	if err != nil {
		t.Fatal(err)
	}
	if len(wins) != 2 || wins[0].Criteria() != `app_id="foot"` || wins[1].Criteria() != `class="Steam"` {
		t.Fatalf("windows = %+v", wins)
	}
	swayTree = func() ([]byte, error) { return nil, errors.New("no socket") }
	if _, err := OpenWindows(); err == nil {
		t.Fatal("expected error")
	}
}
