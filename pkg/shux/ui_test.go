package shux

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestModelCreatesInitialWindowForFreshSession(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	model := NewModel(sessionRef)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model = updated.(Model)

	if !model.initialized {
		t.Fatal("expected model to initialize after first window size message")
	}

	win := requireWindow(t, sessionRef, super)
	if win == nil {
		t.Fatal("expected initial window to be created")
	}

	pane := requirePane(t, sessionRef, super)
	if pane == nil {
		t.Fatal("expected initial pane to be created")
	}
}

func TestAskValueTreatsTypedNilAsNil(t *testing.T) {
	sessionRef, _, cleanup := setupSession(t)
	defer cleanup()

	result, ok := askValue(sessionRef, GetActiveWindow{})
	if !ok {
		t.Fatal("expected askValue to receive a reply")
	}
	if result != nil {
		t.Fatalf("expected nil result for missing active window, got %T", result)
	}
}
