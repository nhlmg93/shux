package cfg

import "testing"

func TestDefaultUIConfig(t *testing.T) {
	u := DefaultUIConfig()
	if !u.Statusline {
		t.Fatal("expected statusline enabled by default")
	}
	if !u.PaneBorders {
		t.Fatal("expected pane borders enabled by default")
	}
	if u.PaneBorderLines != PaneBorderLinesSingle {
		t.Fatalf("pane_border_lines = %q, want %q", u.PaneBorderLines, PaneBorderLinesSingle)
	}
	if !u.PaneLabels {
		t.Fatal("expected pane labels enabled by default")
	}
	if u.StatuslineStyle != DefaultStatuslineStyle {
		t.Fatalf("statusline_style = %q, want %q", u.StatuslineStyle, DefaultStatuslineStyle)
	}
}

func TestUIConfigWithDefaults_fillsStatuslineStyle(t *testing.T) {
	u := UIConfig{StatuslineStyle: ""}.WithDefaults()
	if u.StatuslineStyle != DefaultStatuslineStyle {
		t.Fatalf("statusline_style = %q, want %q", u.StatuslineStyle, DefaultStatuslineStyle)
	}
}

func TestNormalizePaneBorderLines(t *testing.T) {
	cases := map[string]string{
		"single": PaneBorderLinesSingle,
		"SIMPLE": PaneBorderLinesSimple,
		"none":   PaneBorderLinesNone,
		"off":    PaneBorderLinesNone,
		"full":   PaneBorderLinesSingle,
		"bogus":  "",
	}
	for in, want := range cases {
		if got := NormalizePaneBorderLines(in); got != want {
			t.Fatalf("NormalizePaneBorderLines(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUIConfigEffectivePaneBorderLines(t *testing.T) {
	cases := []struct {
		name string
		ui   UIConfig
		want string
	}{
		{"tmux default", UIConfig{PaneBorders: true}, PaneBorderLinesSingle},
		{"legacy pane_borders false", UIConfig{PaneBorders: false}, PaneBorderLinesNone},
		{"explicit simple", UIConfig{PaneBorderLines: PaneBorderLinesSimple}, PaneBorderLinesSimple},
		{"explicit overrides legacy false", UIConfig{PaneBorders: false, PaneBorderLines: PaneBorderLinesSingle}, PaneBorderLinesSingle},
	}
	for _, tc := range cases {
		if got := tc.ui.WithDefaults().EffectivePaneBorderLines(); got != tc.want {
			t.Fatalf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestUIConfigDrawsPaneBorders(t *testing.T) {
	cases := []struct {
		name string
		ui   UIConfig
		want bool
	}{
		{"single full box", UIConfig{PaneBorderLines: PaneBorderLinesSingle, PaneOuterBorder: true}, true},
		{"single split only", UIConfig{PaneBorderLines: PaneBorderLinesSingle, PaneOuterBorder: false}, false},
		{"simple", UIConfig{PaneBorderLines: PaneBorderLinesSimple, PaneOuterBorder: true}, true},
		{"none", UIConfig{PaneBorderLines: PaneBorderLinesNone}, false},
		{"spaces", UIConfig{PaneBorderLines: PaneBorderLinesSpaces}, false},
	}
	for _, tc := range cases {
		if got := tc.ui.DrawsPaneBorders(); got != tc.want {
			t.Fatalf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestUIConfigSplitLinesOnly(t *testing.T) {
	ui := UIConfig{PaneBorderLines: PaneBorderLinesSingle, PaneOuterBorder: false}
	if !ui.SplitLinesOnly() {
		t.Fatal("expected split-lines-only mode")
	}
	full := UIConfig{PaneBorderLines: PaneBorderLinesSingle, PaneOuterBorder: true}
	if full.SplitLinesOnly() {
		t.Fatal("full outer border should not use split-lines-only")
	}
}

func TestUIConfigDrawsWindowBorders(t *testing.T) {
	cases := []struct {
		name string
		ui   UIConfig
		want bool
	}{
		{"single full box", UIConfig{PaneBorderLines: PaneBorderLinesSingle, PaneOuterBorder: true}, true},
		{"single split only", UIConfig{PaneBorderLines: PaneBorderLinesSingle, PaneOuterBorder: false}, false},
		{"none", UIConfig{PaneBorderLines: PaneBorderLinesNone}, false},
		{"spaces", UIConfig{PaneBorderLines: PaneBorderLinesSpaces}, true},
	}
	for _, tc := range cases {
		if got := tc.ui.DrawsWindowBorders(); got != tc.want {
			t.Fatalf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestConfigWithDefaults_appliesUI(t *testing.T) {
	c := Config{UI: UIConfig{StatuslineStyle: "plain"}}.WithDefaults()
	if c.UI.StatuslineStyle != "plain" {
		t.Fatalf("statusline_style = %q, want plain", c.UI.StatuslineStyle)
	}
}
