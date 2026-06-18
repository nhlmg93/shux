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

func TestDefaultKeymapsIncludeCopyModeBindings(t *testing.T) {
	k := DefaultKeymaps()

	if _, ok := k.Lookup("prefix", "["); !ok {
		t.Fatal("expected prefix '[' binding")
	}
	if _, ok := k.Lookup("prefix", "]"); !ok {
		t.Fatal("expected prefix ']' binding")
	}
	if _, ok := k.Lookup("copy_mode", "h"); !ok {
		t.Fatal("expected copy_mode 'h' binding")
	}
	if _, ok := k.Lookup("copy_mode", "y"); !ok {
		t.Fatal("expected copy_mode 'y' binding")
	}
	if _, ok := k.Lookup("copy_mode", "shift+g"); !ok {
		t.Fatal("expected copy_mode 'shift+g' binding")
	}
}
