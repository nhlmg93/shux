package shux

import (
	"testing"
	"time"

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
	_, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "c", Code: 'c'}))

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

func TestModelMouseModeFollowsOption(t *testing.T) {
	sessionRef, _, cleanup := setupSession(t)
	defer cleanup()

	enabled := NewModelWithOptions(sessionRef, DefaultKeymap(), true)
	enabledView := enabled.View()
	if enabledView.MouseMode != tea.MouseModeAllMotion {
		t.Fatalf("expected mouse mode all-motion when enabled, got %v", enabledView.MouseMode)
	}

	disabled := NewModelWithOptions(sessionRef, DefaultKeymap(), false)
	disabledView := disabled.View()
	if disabledView.MouseMode != tea.MouseModeNone {
		t.Fatalf("expected mouse mode none when disabled, got %v", disabledView.MouseMode)
	}
}

func TestModelTmuxResizeBindingResizesPane(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	win := requireWindow(t, sessionRef, super)

	win.Send(Split{Dir: SplitV})
	sessionRef.Send(ResizeMsg{Rows: 24, Cols: 80})
	super.waitContentUpdated(200 * time.Millisecond)

	widthAt := func(index int) int {
		win.Send(SwitchToPane{Index: index})
		result := <-sessionRef.Ask(GetPaneContent{})
		if result == nil {
			return 0
		}
		content := result.(*PaneContent)
		if len(content.Cells) == 0 {
			return 0
		}
		return len(content.Cells[0])
	}

	beforeLeft := widthAt(0)
	beforeRight := widthAt(1)

	model := NewModel(sessionRef)
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 'b', Mod: tea.ModCtrl}))
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight, Mod: tea.ModCtrl}))
	model = updated.(Model)
	_ = model
	super.waitContentUpdated(200 * time.Millisecond)

	afterLeft := widthAt(0)
	afterRight := widthAt(1)
	if afterLeft <= beforeLeft {
		t.Fatalf("expected left pane width to grow after ctrl+right from left pane, before=%d after=%d", beforeLeft, afterLeft)
	}
	if afterRight >= beforeRight {
		t.Fatalf("expected right pane width to shrink after ctrl+right from left pane, before=%d after=%d", beforeRight, afterRight)
	}
}
