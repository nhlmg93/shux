package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"shux/internal/protocol"
)

func TestCopyModeForwardSearchHighlightsMatches(t *testing.T) {
	m := searchTestModel()
	m, _ = m.handlePrefixKey("[")
	m = sendSearchKeys(t, m, "/", "f", "o", "o")

	if m.Search.Query != "foo" {
		t.Fatalf("expected query foo, got %q", m.Search.Query)
	}
	if len(m.Search.Matches) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(m.Search.Matches))
	}
	if m.Search.ActiveIndex != 0 {
		t.Fatalf("expected first match active, got %d", m.Search.ActiveIndex)
	}
	view := m.View().Content
	if !strings.Contains(view, searchActiveANSI) {
		t.Fatal("expected active search highlight ANSI in view")
	}
	if !strings.Contains(view, "[copy] foo  1/3") {
		t.Fatalf("expected copy-mode status with active match index, got %q", view)
	}
}

func TestCopyModeReverseSearchStartsFromLastMatch(t *testing.T) {
	m := searchTestModel()
	m, _ = m.handlePrefixKey("[")
	m = sendSearchKeys(t, m, "?", "f", "o", "o")

	if len(m.Search.Matches) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(m.Search.Matches))
	}
	if m.Search.ActiveIndex != 2 {
		t.Fatalf("expected last match active for reverse search, got %d", m.Search.ActiveIndex)
	}
}

func TestCopyModeSearchHighlightsIncrementallyWhileTyping(t *testing.T) {
	m := searchTestModel()
	m, _ = m.handlePrefixKey("[")

	m = sendKey(t, m, keyText("/"))
	m = sendKey(t, m, keyText("f"))

	if m.Search.Query != "f" {
		t.Fatalf("expected query f, got %q", m.Search.Query)
	}
	if len(m.Search.Matches) != 3 {
		t.Fatalf("expected 3 incremental matches, got %d", len(m.Search.Matches))
	}
	if m.Search.ActiveIndex != 0 {
		t.Fatalf("expected first match active while typing, got %d", m.Search.ActiveIndex)
	}
	view := m.View().Content
	if !strings.Contains(view, searchActiveANSI) {
		t.Fatal("expected active search highlight while typing")
	}
}

func TestCopyModeSearchNavigationNAndShiftN(t *testing.T) {
	m := searchTestModel()
	m, _ = m.handlePrefixKey("[")
	m = sendSearchKeys(t, m, "/", "f", "o", "o")

	m = sendKey(t, m, keyText("n"))
	if m.Search.ActiveIndex != 1 {
		t.Fatalf("expected n to move to second match, got %d", m.Search.ActiveIndex)
	}
	m = sendKey(t, m, keyText("N"))
	if m.Search.ActiveIndex != 0 {
		t.Fatalf("expected N to reverse direction and return to first match, got %d", m.Search.ActiveIndex)
	}
}

func TestCopyModeNewSearchClearsPreviousQuery(t *testing.T) {
	m := searchTestModel()
	m, _ = m.handlePrefixKey("[")
	m = sendSearchKeys(t, m, "/", "f", "o", "o")
	if m.Search.Query != "foo" {
		t.Fatalf("expected query foo, got %q", m.Search.Query)
	}

	m = sendKey(t, m, keyText("/"))
	if m.Search.Query != "" {
		t.Fatalf("expected new search to clear query, got %q", m.Search.Query)
	}
	if len(m.Search.Matches) != 0 {
		t.Fatalf("expected new search to clear matches, got %d", len(m.Search.Matches))
	}
}

func TestSearchMatchesRefreshOnScreenUpdate(t *testing.T) {
	m := searchTestModelWithLines("alpha", "beta")
	m, _ = m.handlePrefixKey("[")
	m = sendSearchKeys(t, m, "/", "f", "o", "o")
	if len(m.Search.Matches) != 0 {
		t.Fatalf("expected no initial matches, got %d", len(m.Search.Matches))
	}

	m = m.WithPaneScreen(screenWithLines(m.SessionID, m.WindowID, m.PaneID, "foo after replay"))
	if len(m.Search.Matches) != 1 {
		t.Fatalf("expected refreshed match after screen update, got %d", len(m.Search.Matches))
	}
}

func TestFindSearchMatchesBoundsRowsAndMatchCount(t *testing.T) {
	lines := []protocol.EventPaneScreenLine{
		styledLine("foo old"),
		styledLine("foo newer"),
		styledLine("foo newest"),
	}

	matches, limited := findSearchMatches(lines, "foo", 2, 1)
	if len(matches) != 1 {
		t.Fatalf("expected one bounded match, got %d", len(matches))
	}
	if !limited {
		t.Fatal("expected bounded search to report limited=true")
	}
	if matches[0].Row != 0 {
		t.Fatalf("expected bounded row to be rebased to 0, got %d", matches[0].Row)
	}
}

func searchTestModel() Model {
	return searchTestModelWithLines("foo bar foo", "baz foo")
}

func searchTestModelWithLines(lines ...string) Model {
	sessionID := protocol.SessionID("s-1")
	windowID := protocol.WindowID("w-1")
	paneID := protocol.PaneID("p-1")
	m := NewModel(ModelConfig{
		SessionID: sessionID,
		WindowID:  windowID,
		PaneID:    paneID,
	}).WithLayoutSnapshot(LayoutSnapshot{
		SessionID:  sessionID,
		WindowID:   windowID,
		WindowCols: 80,
		WindowRows: 24,
		ActivePane: paneID,
		Panes: []LayoutPane{
			{PaneID: paneID, Col: 0, Row: 0, Cols: 80, Rows: 24},
		},
	})
	return m.WithPaneScreen(screenWithLines(sessionID, windowID, paneID, lines...))
}

func screenWithLines(sessionID protocol.SessionID, windowID protocol.WindowID, paneID protocol.PaneID, lines ...string) protocol.EventPaneScreenChanged {
	screenLines := make([]protocol.EventPaneScreenLine, 0, len(lines))
	for _, line := range lines {
		screenLines = append(screenLines, styledLine(line))
	}
	return protocol.EventPaneScreenChanged{
		SessionID: sessionID,
		WindowID:  windowID,
		PaneID:    paneID,
		Revision:  1,
		Cols:      80,
		Rows:      24,
		Lines:     screenLines,
	}
}

func styledLine(text string) protocol.EventPaneScreenLine {
	runes := []rune(text)
	cells := make([]protocol.EventPaneScreenCell, 0, len(runes))
	for _, r := range runes {
		cells = append(cells, protocol.EventPaneScreenCell{Text: string(r)})
	}
	return protocol.EventPaneScreenLine{
		Text:  text,
		Cells: cells,
	}
}

func sendSearchKeys(t *testing.T, m Model, opener string, keys ...string) Model {
	t.Helper()
	m = sendKey(t, m, keyText(opener))
	for _, key := range keys {
		m = sendKey(t, m, keyText(key))
	}
	return sendKey(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
}

func sendKey(t *testing.T, m Model, msg tea.KeyPressMsg) Model {
	t.Helper()
	updated, _ := m.Update(msg)
	next, ok := updated.(Model)
	if !ok {
		t.Fatalf("expected model update to return ui.Model, got %T", updated)
	}
	return next
}

func keyText(text string) tea.KeyPressMsg {
	r, _ := utf8.DecodeRuneInString(text)
	return tea.KeyPressMsg(tea.Key{Text: text, Code: r})
}
