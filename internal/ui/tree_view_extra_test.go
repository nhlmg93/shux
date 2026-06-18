package ui

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"shux/internal/cfg"
	"shux/internal/protocol"
)

func treeKey(text string) tea.KeyPressMsg {
	r, _ := utf8.DecodeRuneInString(text)
	return tea.KeyPressMsg(tea.Key{Text: text, Code: r})
}

func TestTreeViewTagFilterSort(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	_ = ctx

	m := NewModel(ModelConfig{
		SessionID: protocol.SessionID("s-1"),
		WindowID:  protocol.WindowID("w-1"),
		PaneID:    protocol.PaneID("p-1"),
		Keymaps:   cfg.DefaultKeymaps(),
	}).WithWindowIDs([]protocol.WindowID{"w-1", "w-2"})
	m.WindowNames = map[protocol.WindowID]string{"w-1": "alpha", "w-2": "beta"}
	m, _ = m.startTreeView(treeViewDefault)

	updated, _ := m.handleTreeViewKey(treeKey("t"))
	got := updated.(Model)
	if len(got.TreeView.Tagged) != 1 {
		t.Fatalf("t should tag one item, got %d tags", len(got.TreeView.Tagged))
	}

	updated, _ = got.handleTreeViewKey(treeKey("T"))
	got = updated.(Model)
	if len(got.TreeView.Tagged) != 0 {
		t.Fatal("T should clear all tags")
	}

	updated, _ = got.handleTreeViewKey(treeKey("f"))
	got = updated.(Model)
	if got.TreeView.PromptKind != treePromptFilter {
		t.Fatal("f should open filter prompt")
	}
	updated, _ = got.handleTreeViewKey(treeKey("a"))
	got = updated.(Model)
	updated, _ = got.handleTreeViewKey(treeKey("l"))
	got = updated.(Model)
	updated, _ = got.handleTreeViewKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	got = updated.(Model)
	if got.TreeView.Filter != "al" {
		t.Fatalf("filter = %q, want al", got.TreeView.Filter)
	}
	foundAlpha := false
	for _, it := range got.TreeView.Items {
		if it.kind == treeItemWindow && it.label == "alpha" {
			foundAlpha = true
		}
		if it.kind == treeItemWindow && it.label == "beta" {
			t.Fatal("filter should hide beta window")
		}
	}
	if !foundAlpha {
		t.Fatal("filter should keep alpha window")
	}

	updated, _ = got.handleTreeViewKey(treeKey("O"))
	got = updated.(Model)
	if got.TreeView.SortOrder != treeSortName {
		t.Fatal("O should cycle sort to name")
	}
}

func TestTreeViewShortcutKeySelect(t *testing.T) {
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

	var windowIdx int
	for i, it := range m.TreeView.Items {
		if it.kind == treeItemWindow && it.windowID == "w-2" {
			windowIdx = i
			break
		}
	}
	key := m.TreeView.Items[windowIdx].shortcutKey
	if key == "" {
		t.Fatal("expected shortcut key on window item")
	}

	updated, _ := m.handleTreeViewKey(treeKey(key))
	got := updated.(Model)
	if got.TreeView.Open {
		t.Fatal("shortcut select should close tree")
	}
	if got.WindowID != "w-2" {
		t.Fatalf("shortcut select window = %q, want w-2", got.WindowID)
	}
}

func TestTreeViewBoxDrawingLine(t *testing.T) {
	m := Model{}
	it := treeItem{
		depth:         2,
		hasChildren:   false,
		lastSibling:   true,
		ancestorsLast: []bool{false, false},
		label:         "pane",
		detail:        "#1",
		shortcutKey:   "3",
	}
	line := m.treeItemLine(it, 80)
	if !strings.Contains(line, "└─") || !strings.Contains(line, "(3)") || !strings.Contains(line, "pane") {
		t.Fatalf("unexpected line %q", line)
	}
}
