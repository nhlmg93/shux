package shux

import (
	"os"
	"testing"
	"time"
)

func waitForWindowCount(t *testing.T, sessionRef *SessionRef, want int) SessionSnapshotData {
	t.Helper()

	var data SessionSnapshotData
	if !pollFor(500*time.Millisecond, func() bool {
		result := <-sessionRef.Ask(GetSessionSnapshotData{})
		snapshot, ok := result.(SessionSnapshotData)
		if !ok {
			return false
		}
		data = snapshot
		return len(snapshot.WindowOrder) == want
	}) {
		t.Fatalf("timeout waiting for %d windows", want)
	}
	return data
}

func TestActionKillWindowRemovesActiveWindow(t *testing.T) {
	sessionRef, _, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	before := waitForWindowCount(t, sessionRef, 2)
	killedID := before.ActiveWindow

	resultAny := <-sessionRef.Ask(ActionMsg{Action: ActionKillWindow})
	result, ok := resultAny.(ActionResult)
	if !ok {
		t.Fatalf("expected ActionResult, got %T", resultAny)
	}
	if result.Err != nil {
		t.Fatalf("kill-window returned error: %v", result.Err)
	}
	if result.Quit {
		t.Fatal("kill-window should not request UI quit")
	}

	after := waitForWindowCount(t, sessionRef, 1)
	if after.ActiveWindow == killedID {
		t.Fatalf("expected active window to change after kill-window, still %d", killedID)
	}
	if after.WindowOrder[0] == killedID {
		t.Fatalf("expected killed window %d to be removed from order %v", killedID, after.WindowOrder)
	}
}

func TestActionLastWindowUsesSelectionHistory(t *testing.T) {
	sessionRef, _, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	before := waitForWindowCount(t, sessionRef, 3)
	original := before.ActiveWindow
	target := before.WindowOrder[2]

	resultAny := <-sessionRef.Ask(ActionMsg{Action: ActionSelectWindow2})
	result, ok := resultAny.(ActionResult)
	if !ok {
		t.Fatalf("expected ActionResult, got %T", resultAny)
	}
	if result.Err != nil {
		t.Fatalf("select-window returned error: %v", result.Err)
	}
	selected := (<-sessionRef.Ask(GetSessionSnapshotData{})).(SessionSnapshotData)
	if selected.ActiveWindow != target {
		t.Fatalf("expected select-window to activate %d, got %d", target, selected.ActiveWindow)
	}

	resultAny = <-sessionRef.Ask(ActionMsg{Action: ActionLastWindow})
	result, ok = resultAny.(ActionResult)
	if !ok {
		t.Fatalf("expected ActionResult, got %T", resultAny)
	}
	if result.Err != nil {
		t.Fatalf("last-window returned error: %v", result.Err)
	}
	after := (<-sessionRef.Ask(GetSessionSnapshotData{})).(SessionSnapshotData)
	if after.ActiveWindow != original {
		t.Fatalf("expected last-window to return to %d, got %d", original, after.ActiveWindow)
	}
}

func TestActionDetachFailureDoesNotQuit(t *testing.T) {
	homeFile, err := os.CreateTemp("", "shux-home")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	_ = homeFile.Close()
	t.Setenv("HOME", homeFile.Name())

	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	_ = requireWindow(t, sessionRef, super)

	resultAny := <-sessionRef.Ask(ActionMsg{Action: ActionDetach})
	result, ok := resultAny.(ActionResult)
	if !ok {
		t.Fatalf("expected ActionResult, got %T", resultAny)
	}
	if result.Err == nil {
		t.Fatal("expected detach failure to return an error")
	}
	if result.Quit {
		t.Fatal("detach failure should not request UI quit")
	}
	if <-sessionRef.Ask(GetActiveWindow{}) == nil {
		t.Fatal("expected session to remain alive after failed detach")
	}
}

func TestKeymapCommandAliasesRemainSupported(t *testing.T) {
	tests := map[string]Action{
		"next-window":     ActionNextWindow,
		"previous-window": ActionPrevWindow,
		"prev-window":     ActionPrevWindow,
		"detach-client":   ActionDetach,
	}

	for command, want := range tests {
		t.Run(command, func(t *testing.T) {
			keymap, err := NewKeymap(KeymapConfig{Bind: map[string]string{"x": command}})
			if err != nil {
				t.Fatalf("NewKeymap(%q): %v", command, err)
			}
			binding, ok := keymap.BindingFor("x")
			if !ok {
				t.Fatalf("expected binding for %q", command)
			}
			if binding.Action != want {
				t.Fatalf("binding action = %q, want %q", binding.Action, want)
			}
		})
	}
}
