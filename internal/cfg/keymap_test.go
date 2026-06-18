package cfg

import "testing"

func TestDefaultKeymaps_hasPaneZoomToggle(t *testing.T) {
	k := DefaultKeymaps()
	got, ok := k.Lookup("prefix", "z")
	if !ok {
		t.Fatal("missing prefix z binding")
	}
	if got.Builtin != ActionTogglePaneZoom {
		t.Fatalf("prefix z builtin = %q, want %q", got.Builtin, ActionTogglePaneZoom)
	}
}
