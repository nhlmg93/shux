package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"shux/internal/protocol"
)

func TestNewModel_viewContainsPane(t *testing.T) {
	m := NewModel(ModelConfig{
		SessionID: protocol.SessionID("s-1"),
		WindowID:  protocol.WindowID("w-1"),
		PaneID:    protocol.PaneID("p-1"),
	})
	if !strings.Contains(m.View().Content, string(m.PaneID)) {
		t.Fatalf("view should include pane %q; got %q", m.PaneID, m.View().Content)
	}
}

func TestWindowClosedSwitchesCurrentClientToAnotherWindow(t *testing.T) {
	m := NewModel(ModelConfig{
		SessionID: protocol.SessionID("s-1"),
		WindowID:  protocol.WindowID("w-1"),
		PaneID:    protocol.PaneID("p-1"),
	}).WithWindowIDs([]protocol.WindowID{"w-1", "w-2"})
	m = m.WithLayoutSnapshot(LayoutSnapshot{SessionID: "s-1", WindowID: "w-2", WindowCols: 80, WindowRows: 24, Panes: []LayoutPane{{PaneID: "p-2", Col: 0, Row: 0, Cols: 80, Rows: 24}}})
	m = m.switchWindow("w-1")

	updated, cmd := m.Update(HubEvent{E: protocol.EventWindowClosed{SessionID: "s-1", WindowID: "w-1"}})
	if cmd != nil {
		t.Fatal("unexpected command without supervisor")
	}
	got := updated.(Model)
	if got.WindowID != protocol.WindowID("w-2") {
		t.Fatalf("closed active window should switch to remaining window; got %q", got.WindowID)
	}
}

func TestLastWindowClosedQuits(t *testing.T) {
	quit := false
	m := NewModel(ModelConfig{
		SessionID: protocol.SessionID("s-1"),
		WindowID:  protocol.WindowID("w-1"),
		PaneID:    protocol.PaneID("p-1"),
		OnExit: func(intent ExitIntent) {
			quit = intent == ExitQuit
		},
	}).WithWindowIDs([]protocol.WindowID{"w-1"})

	_, cmd := m.Update(HubEvent{E: protocol.EventWindowClosed{SessionID: "s-1", WindowID: "w-1"}})
	if cmd == nil {
		t.Fatal("expected tea.Quit command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
	if !quit {
		t.Fatal("expected ExitQuit intent")
	}
}

func TestSwitchWindowByNumberIsOneBased(t *testing.T) {
	m := NewModel(ModelConfig{
		SessionID: protocol.SessionID("s-1"),
		WindowID:  protocol.WindowID("w-2"),
		PaneID:    protocol.PaneID("p-1"),
	}).WithWindowIDs([]protocol.WindowID{"w-1", "w-2", "w-3"})

	m = m.switchWindowByNumber(1)
	if m.WindowID != protocol.WindowID("w-1") {
		t.Fatalf("ctrl+b 1 should select first window; got %q", m.WindowID)
	}

	m = m.switchWindowByNumber(3)
	if m.WindowID != protocol.WindowID("w-3") {
		t.Fatalf("ctrl+b 3 should select third window; got %q", m.WindowID)
	}
}
