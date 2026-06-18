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

func TestConfigWithDefaults_appliesUI(t *testing.T) {
	c := Config{UI: UIConfig{StatuslineStyle: "plain"}}.WithDefaults()
	if c.UI.StatuslineStyle != "plain" {
		t.Fatalf("statusline_style = %q, want plain", c.UI.StatuslineStyle)
	}
}
