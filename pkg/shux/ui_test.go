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

func TestModelUsesConfiguredKeymap(t *testing.T) {
	sessionRef, _, cleanup := setupSession(t)
	defer cleanup()

	keymap, err := NewKeymap(KeymapConfig{Prefix: "C-a"})
	if err != nil {
		t.Fatalf("NewKeymap: %v", err)
	}

	model := NewModelWithKeymap(sessionRef, keymap)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model = updated.(Model)

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'a', Mod: tea.ModCtrl}))
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "c", Code: 'c'}))
	model = updated.(Model)

	result, ok := askValue(sessionRef, GetSessionSnapshotData{})
	if !ok {
		t.Fatal("expected session snapshot data reply")
	}
	data, ok := result.(SessionSnapshotData)
	if !ok {
		t.Fatalf("expected SessionSnapshotData, got %T", result)
	}
	if len(data.WindowOrder) != 2 {
		t.Fatalf("expected custom keymap to create second window, got %d windows", len(data.WindowOrder))
	}
}
