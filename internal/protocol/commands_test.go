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
