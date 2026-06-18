package ui

import (
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"shux/internal/cfg"
	"shux/internal/protocol"
)

func TestTreeViewBuildAndNavigate(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	_ = ctx

	m := NewModel(ModelConfig{
		SessionID: protocol.SessionID("s-1"),
		WindowID:  protocol.WindowID("w-1"),
		PaneID:    protocol.PaneID("p-1"),
		Keymaps:   cfg.DefaultKeymaps(),
	}).WithWindowIDs([]protocol.WindowID{"w-1", "w-2"})
	m = m.WithLayoutSnapshot(LayoutSnapshot{
		SessionID: "s-1", WindowID: "w-1", WindowCols: 80, WindowRows: 24,
		Panes: []LayoutPane{{PaneID: "p-1", Col: 0, Row: 0, Cols: 80, Rows: 24}},
		ActivePane: "p-1",
	})
	m = m.WithLayoutSnapshot(LayoutSnapshot{
		SessionID: "s-1", WindowID: "w-2", WindowCols: 80, WindowRows: 24,
		Panes: []LayoutPane{
			{PaneID: "p-2", Col: 0, Row: 0, Cols: 40, Rows: 24},
			{PaneID: "p-3", Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
		ActivePane: "p-2",
	})
	m.ActivePaneID = "p-1"

	m, _ = m.startTreeView(treeViewWindowsCollapsed)
	if !m.TreeView.Open {
		t.Fatal("tree view should open")
	}
	if len(m.TreeView.Items) < 3 {
		t.Fatalf("expected session + windows; got %d items", len(m.TreeView.Items))
	}

	updated, _ := m.handleTreeViewKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	got := updated.(Model)
	if got.TreeView.Cursor == 0 {
		t.Fatal("down should move cursor")
	}

	updated, _ = got.handleTreeViewKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	got = updated.(Model)
	if !got.TreeView.Items[got.TreeView.Cursor].expanded {
		t.Fatal("right should expand node with children")
	}
}

func TestTreeViewPrefixBindings(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	_ = ctx

	m := NewModel(ModelConfig{
		SessionID: protocol.SessionID("s-1"),
		WindowID:  protocol.WindowID("w-1"),
		PaneID:    protocol.PaneID("p-1"),
		Keymaps:   cfg.DefaultKeymaps(),
	})
	m.Prefix = true

	updated, cmd := m.handlePrefixKey("s")
	if cmd != nil {
		t.Fatal("unexpected cmd")
	}
	if !updated.TreeView.Open || updated.TreeView.Mode != treeViewSessionsCollapsed {
		t.Fatal("prefix s should open sessions-collapsed tree")
	}

	m = NewModel(ModelConfig{
		SessionID: protocol.SessionID("s-1"),
		WindowID:  protocol.WindowID("w-1"),
		PaneID:    protocol.PaneID("p-1"),
		Keymaps:   cfg.DefaultKeymaps(),
	})
	m.Prefix = true
	updated, cmd = m.handlePrefixKey("w")
	if cmd != nil {
		t.Fatal("unexpected cmd")
	}
	if !updated.TreeView.Open || updated.TreeView.Mode != treeViewWindowsCollapsed {
		t.Fatal("prefix w should open windows-collapsed tree")
	}
}

func TestTreeViewSelectWindow(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	_ = ctx

	m := NewModel(ModelConfig{
		SessionID: protocol.SessionID("s-1"),
		WindowID:  protocol.WindowID("w-1"),
		PaneID:    protocol.PaneID("p-1"),
	}).WithWindowIDs([]protocol.WindowID{"w-1", "w-2"})
	m = m.WithLayoutSnapshot(LayoutSnapshot{
		SessionID: "s-1", WindowID: "w-1", WindowCols: 80, WindowRows: 24,
		Panes: []LayoutPane{{PaneID: "p-1", Col: 0, Row: 0, Cols: 80, Rows: 24}},
	})
	m = m.WithLayoutSnapshot(LayoutSnapshot{
		SessionID: "s-1", WindowID: "w-2", WindowCols: 80, WindowRows: 24,
		Panes: []LayoutPane{{PaneID: "p-2", Col: 0, Row: 0, Cols: 80, Rows: 24}},
	})
	m, _ = m.startTreeView(treeViewDefault)
	for i, it := range m.TreeView.Items {
		if it.kind == treeItemWindow && it.windowID == "w-2" {
			m.TreeView.Cursor = i
			break
		}
	}

	updated, cmd := m.handleTreeViewKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	got := updated.(Model)
	if got.TreeView.Open {
		t.Fatal("enter should close tree view")
	}
	if got.WindowID != protocol.WindowID("w-2") {
		t.Fatalf("enter on window should switch; got %q", got.WindowID)
	}
	if cmd != nil {
		// resize cmd optional without supervisor
	}
}
