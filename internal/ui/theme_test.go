package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	"shux/internal/cfg"
	"shux/internal/protocol"
)

func uiTestModel(ui cfg.UIConfig) Model {
	m := NewModel(ModelConfig{
		SessionID: protocol.SessionID("s-1"),
		WindowID:  protocol.WindowID("w-1"),
		PaneID:    protocol.PaneID("p-1"),
		UI:        ui,
	})
	return m.WithLayoutSnapshot(LayoutSnapshot{
		SessionID:  "s-1",
		WindowID:   "w-1",
		WindowCols: 40,
		WindowRows: 6,
		SyncPanes:  true,
		Panes: []LayoutPane{
			{PaneID: "p-1", Col: 0, Row: 0, Cols: 40, Rows: 5},
		},
		ActivePane: "p-1",
	})
}

func TestViewString_statuslineDisabledOmitsSyncIndicator(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	m := uiTestModel(cfg.UIConfig{
		Statusline:      false,
		PaneBorders:     true,
		PaneLabels:      true,
		StatuslineStyle: cfg.DefaultStatuslineStyle,
	})

	view := m.viewString()
	if strings.Contains(view, "[SYNC:") {
		t.Fatalf("statusline disabled should omit SYNC indicator; got %q", view)
	}
	if strings.Contains(view, "s-1 | 1:w-1") {
		t.Fatalf("statusline disabled should omit default status segments; got %q", view)
	}
	_ = ctx
}

func TestViewString_paneBordersDisabledOmitsBoxDrawing(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	m := uiTestModel(cfg.UIConfig{
		Statusline:      true,
		PaneBorders:     false,
		PaneLabels:      true,
		StatuslineStyle: cfg.DefaultStatuslineStyle,
	})

	view := m.viewString()
	if strings.ContainsRune(view, '┌') || strings.ContainsRune(view, '═') {
		t.Fatalf("pane borders disabled should omit box-drawing chars; got %q", view)
	}
	_ = ctx
}

func TestRenderStatusRow_plainStyleOmitsReverse(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	m := uiTestModel(cfg.UIConfig{
		Statusline:      true,
		PaneBorders:     true,
		PaneLabels:      true,
		StatuslineStyle: "plain",
	})

	plain := m.renderStatusRow(40)
	reverse := m.WithUI(cfg.UIConfig{
		Statusline:      true,
		PaneBorders:     true,
		PaneLabels:      true,
		StatuslineStyle: cfg.DefaultStatuslineStyle,
	}).renderStatusRow(40)

	if strings.Contains(plain, "\x1b[7m") {
		t.Fatalf("plain statusline should not use reverse video; got %q", plain)
	}
	if plain == reverse {
		t.Fatalf("plain and reverse status rows should differ; got %q", plain)
	}
	_ = ctx
}

func (m Model) WithUI(ui cfg.UIConfig) Model {
	m.UI = ui.WithDefaults()
	return m
}
