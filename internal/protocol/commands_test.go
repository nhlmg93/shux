package protocol

import "testing"

func TestValidateCommandWindowSelectLayout(t *testing.T) {
	cmd := CommandWindowSelectLayout{
		Meta:         CommandMeta{ClientID: "c-1", RequestID: 1},
		SessionID:    "s-1",
		WindowID:     "w-1",
		ActivePaneID: "p-1",
		Preset:       LayoutPresetEvenVertical,
	}
	if err := ValidateCommand(cmd); err != nil {
		t.Fatalf("ValidateCommand(CommandWindowSelectLayout) error: %v", err)
	}
	cmd.Preset = LayoutPreset("unknown")
	if err := ValidateCommand(cmd); err == nil {
		t.Fatal("expected unknown layout preset to fail validation")
	}
}

func TestValidateCommandPaneSwap(t *testing.T) {
	cmd := CommandPaneSwap{
		Meta:      CommandMeta{ClientID: "c-1", RequestID: 1},
		SessionID: "s-1",
		WindowID:  "w-1",
		PaneID:    "p-1",
		Direction: PaneDirectionLeft,
	}
	if err := ValidateCommand(cmd); err != nil {
		t.Fatalf("ValidateCommand(CommandPaneSwap) error: %v", err)
	}
	cmd.Direction = PaneDirection(99)
	if err := ValidateCommand(cmd); err == nil {
		t.Fatal("expected invalid pane swap direction to fail validation")
	}
}

func TestRouteNewLayoutCommands(t *testing.T) {
	cases := []Command{
		CommandWindowSelectLayout{
			Meta:         CommandMeta{ClientID: "c-1", RequestID: 1},
			SessionID:    "s-1",
			WindowID:     "w-1",
			ActivePaneID: "p-1",
			Preset:       LayoutPresetEvenHorizontal,
		},
		CommandPaneSwap{
			Meta:      CommandMeta{ClientID: "c-1", RequestID: 2},
			SessionID: "s-1",
			WindowID:  "w-1",
			PaneID:    "p-1",
			Direction: PaneDirectionRight,
		},
	}
	for _, cmd := range cases {
		sid, ok := RouteSessionID(cmd)
		if !ok || sid != "s-1" {
			t.Fatalf("RouteSessionID(%T) = (%q,%v), want (s-1,true)", cmd, sid, ok)
		}
		wid, ok := RouteWindowID(cmd)
		if !ok || wid != "w-1" {
			t.Fatalf("RouteWindowID(%T) = (%q,%v), want (w-1,true)", cmd, wid, ok)
		}
		if _, ok := RoutePaneID(cmd); ok {
			t.Fatalf("RoutePaneID(%T) should be false", cmd)
		}
	}
}

func TestValidateCommand_paneZoomToggle(t *testing.T) {
	valid := CommandPaneZoomToggle{
		SessionID: "s-1",
		WindowID:  "w-1",
		PaneID:    "p-1",
	}
	if err := ValidateCommand(valid); err != nil {
		t.Fatalf("ValidateCommand(valid zoom) error = %v", err)
	}

	invalid := valid
	invalid.PaneID = ""
	if err := ValidateCommand(invalid); err == nil {
		t.Fatal("ValidateCommand(invalid zoom) expected error")
	}
}

func TestValidateCommandWindowSwap(t *testing.T) {
	cmd := CommandWindowSwap{
		SessionID:    "s-1",
		WindowID:     "w-1",
		WithWindowID: "w-2",
	}
	if err := ValidateCommand(cmd); err != nil {
		t.Fatalf("ValidateCommand(CommandWindowSwap) error: %v", err)
	}
	sid, ok := RouteSessionID(cmd)
	if !ok || sid != "s-1" {
		t.Fatalf("RouteSessionID = (%q, %v)", sid, ok)
	}
	if _, ok := RouteWindowID(cmd); ok {
		t.Fatal("CommandWindowSwap should not route to a window actor")
	}
}

func TestRouteIDs_includeKillWindow(t *testing.T) {
	cmd := CommandKillWindow{
		SessionID: "s-1",
		WindowID:  "w-1",
	}
	if sid, ok := RouteSessionID(cmd); !ok || sid != "s-1" {
		t.Fatalf("RouteSessionID(kill window) = (%q, %v), want (s-1, true)", sid, ok)
	}
	if wid, ok := RouteWindowID(cmd); !ok || wid != "w-1" {
		t.Fatalf("RouteWindowID(kill window) = (%q, %v), want (w-1, true)", wid, ok)
	}
	if pid, ok := RoutePaneID(cmd); ok || pid != "" {
		t.Fatalf("RoutePaneID(kill window) = (%q, %v), want (\"\", false)", pid, ok)
	}
}

func TestRouteIDs_includePaneZoomToggle(t *testing.T) {
	cmd := CommandPaneZoomToggle{
		SessionID: "s-1",
		WindowID:  "w-1",
		PaneID:    "p-1",
	}

	if sid, ok := RouteSessionID(cmd); !ok || sid != "s-1" {
		t.Fatalf("RouteSessionID(zoom) = (%q, %v), want (%q, true)", sid, ok, SessionID("s-1"))
	}
	if wid, ok := RouteWindowID(cmd); !ok || wid != "w-1" {
		t.Fatalf("RouteWindowID(zoom) = (%q, %v), want (%q, true)", wid, ok, WindowID("w-1"))
	}
	if pid, ok := RoutePaneID(cmd); ok || pid != "" {
		t.Fatalf("RoutePaneID(zoom) = (%q, %v), want (\"\", false)", pid, ok)
	}
}
