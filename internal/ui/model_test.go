package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"shux/internal/luabind"
	"shux/internal/protocol"
)

type testLuaRuntime struct {
	left  string
	right string
}

func (r testLuaRuntime) CallKeymapRef(_ int) {}
func (r testLuaRuntime) Statusline(_ luabind.StatuslineContext) (string, string) {
	return r.left, r.right
}
func (r testLuaRuntime) Close() {}

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

func TestViewIncludesStatusRowByDefault(t *testing.T) {
	m := NewModel(ModelConfig{
		SessionID: protocol.SessionID("s-1"),
		WindowID:  protocol.WindowID("w-1"),
		PaneID:    protocol.PaneID("p-1"),
	})
	m = m.WithWindowIDs([]protocol.WindowID{"w-1"})
	m = m.WithLayoutSnapshot(LayoutSnapshot{
		SessionID:  "s-1",
		WindowID:   "w-1",
		WindowCols: 40,
		WindowRows: 6,
		Panes: []LayoutPane{
			{PaneID: "p-1", Col: 0, Row: 0, Cols: 40, Rows: 5},
		},
		ActivePane: "p-1",
	})
	view := m.viewString()
	if !strings.Contains(view, "s-1 | 1:w-1 | p-1") {
		t.Fatalf("status row missing default segments: %q", view)
	}
}

func TestViewUsesLuaStatuslineSegments(t *testing.T) {
	m := NewModel(ModelConfig{
		SessionID: protocol.SessionID("s-1"),
		WindowID:  protocol.WindowID("w-1"),
		PaneID:    protocol.PaneID("p-1"),
		Lua:       testLuaRuntime{left: "LSEG", right: "RSEG"},
	})
	m = m.WithLayoutSnapshot(LayoutSnapshot{
		SessionID:  "s-1",
		WindowID:   "w-1",
		WindowCols: 30,
		WindowRows: 4,
		Panes: []LayoutPane{
			{PaneID: "p-1", Col: 0, Row: 0, Cols: 30, Rows: 3},
		},
		ActivePane: "p-1",
	})
	view := m.viewString()
	if !strings.Contains(view, "LSEG") || !strings.Contains(view, "RSEG") {
		t.Fatalf("status row should include lua segments, got %q", view)
	}
}

func TestPaneQuickSelectLabel(t *testing.T) {
	if got, ok := paneQuickSelectLabel(0); !ok || got != "1" {
		t.Fatalf("index 0 = %q, %v; want %q, true", got, ok, "1")
	}
	if got, ok := paneQuickSelectLabel(9); !ok || got != "0" {
		t.Fatalf("index 9 = %q, %v; want %q, true", got, ok, "0")
	}
	if got, ok := paneQuickSelectLabel(10); ok || got != "" {
		t.Fatalf("index 10 = %q, %v; want empty,false", got, ok)
	}
}

func TestPaneQuickSelectDigitStartsFocusCommand(t *testing.T) {
	m := NewModel(ModelConfig{
		ClientID:  protocol.ClientID("c-1"),
		SessionID: protocol.SessionID("s-1"),
		WindowID:  protocol.WindowID("w-1"),
		PaneID:    protocol.PaneID("p-1"),
	})
	m = m.WithLayoutSnapshot(LayoutSnapshot{
		SessionID:  "s-1",
		WindowID:   "w-1",
		WindowCols: 80,
		WindowRows: 24,
		Panes: []LayoutPane{
			{PaneID: "p-1", Col: 0, Row: 0, Cols: 40, Rows: 24},
			{PaneID: "p-2", Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	})
	m.PaneQuickSelectEnabled = true

	updated, _ := m.handlePaneQuickSelectKey("2")
	got := updated.(Model)
	if got.PaneQuickSelectEnabled {
		t.Fatal("quick select should close after digit selection")
	}
	if len(got.Pending) != 1 {
		t.Fatalf("pending size = %d, want 1", len(got.Pending))
	}
	if got.NextRequest != 1 {
		t.Fatalf("next request = %d, want 1", got.NextRequest)
	}
	if got.Pending[got.NextRequest].Kind != pendingPaneFocus {
		t.Fatalf("pending kind = %v, want pane focus", got.Pending[got.NextRequest].Kind)
	}
}
