// Package testutil provides shared test infrastructure for shux.
//
// Philosophy: NO UNIT TESTS. Focus on integration, E2E, and stress testing.
// All assertions here are invariants that must hold across the system.
package testutil

import (
	"fmt"
	"testing"
	"time"

	"shux/pkg/shux"
)

// Type aliases for convenience
type (
	SessionRef      = shux.SessionRef
	WindowRef       = shux.WindowRef
	PaneRef         = shux.PaneRef
	SessionSnapshot = shux.SessionSnapshot
	WindowSnapshot  = shux.WindowSnapshot
	PaneSnapshot    = shux.PaneSnapshot
)

// SessionInvariant validates session-level state consistency.
// Panics on violation (Tiger Style - fail fast on bugs).
type SessionInvariant struct {
	SessionID    uint32
	WindowCount  int
	ActiveWindow uint32
	WindowOrder  []uint32
}

// AssertSessionInvariants validates session state or fails the test.
func AssertSessionInvariants(t *testing.T, sessionRef *shux.SessionRef, inv SessionInvariant) {
	t.Helper()

	result := <-sessionRef.Ask(shux.GetSessionSnapshotData{})
	data, ok := result.(shux.SessionSnapshotData)
	if !ok {
		t.Fatalf("expected SessionSnapshotData, got %T", result)
	}

	if data.ID != inv.SessionID {
		t.Errorf("session ID: got %d, want %d", data.ID, inv.SessionID)
	}
	if len(data.WindowOrder) != inv.WindowCount {
		t.Errorf("window count: got %d, want %d", len(data.WindowOrder), inv.WindowCount)
	}
	if inv.ActiveWindow != 0 && data.ActiveWindow != inv.ActiveWindow {
		t.Errorf("active window: got %d, want %d", data.ActiveWindow, inv.ActiveWindow)
	}
}

// WindowInvariant validates window-level state consistency.
type WindowInvariant struct {
	WindowID   uint32
	PaneCount  int
	ActivePane uint32
	PaneOrder  []uint32
	Rows       int
	Cols       int
}

// AssertWindowInvariants validates window state or fails the test.
func AssertWindowInvariants(t *testing.T, windowRef *shux.WindowRef, inv WindowInvariant) {
	t.Helper()

	result := <-windowRef.Ask(shux.GetWindowSnapshotData{})
	data, ok := result.(shux.WindowSnapshot)
	if !ok {
		t.Fatalf("expected WindowSnapshot, got %T", result)
	}

	if data.ID != inv.WindowID {
		t.Errorf("window ID: got %d, want %d", data.ID, inv.WindowID)
	}
	if len(data.PaneOrder) != inv.PaneCount {
		t.Errorf("pane count: got %d, want %d", len(data.PaneOrder), inv.PaneCount)
	}
	if inv.ActivePane != 0 && data.ActivePane != inv.ActivePane {
		t.Errorf("active pane: got %d, want %d", data.ActivePane, inv.ActivePane)
	}
}

// TopologyInvariant validates the complete session/window/pane hierarchy.
func AssertTopologyInvariant(t *testing.T, sessionRef *shux.SessionRef) {
	t.Helper()

	// Session level
	sessionResult := <-sessionRef.Ask(shux.GetSessionSnapshotData{})
	sessionData, ok := sessionResult.(shux.SessionSnapshotData)
	if !ok {
		t.Fatalf("expected SessionSnapshotData, got %T", sessionResult)
	}

	// If no windows, topology is trivially valid
	if len(sessionData.WindowOrder) == 0 {
		if sessionData.ActiveWindow != 0 {
			t.Error("active window should be 0 when no windows exist")
		}
		return
	}

	// Active window must exist in order list
	foundActive := false
	for _, wid := range sessionData.WindowOrder {
		if wid == sessionData.ActiveWindow {
			foundActive = true
			break
		}
	}
	if !foundActive {
		t.Errorf("active window %d not found in window order %v", sessionData.ActiveWindow, sessionData.WindowOrder)
	}

	// Note: Full window state validation requires SessionSnapshot (via GetFullSessionSnapshot)
	// which contains the complete Windows list. This invariant check validates
	// the session-level topology only - caller should use AssertSnapshotInvariant
	// for complete validation after obtaining a full snapshot.
}

// SnapshotInvariant validates that a snapshot is restorable and preserves state.
func AssertSnapshotInvariant(t *testing.T, snapshot *shux.SessionSnapshot) {
	t.Helper()

	if snapshot == nil {
		t.Fatal("snapshot is nil")
	}

	if snapshot.Version != shux.SnapshotVersion {
		t.Errorf("snapshot version: got %d, want %d", snapshot.Version, shux.SnapshotVersion)
	}

	// Window order consistency
	if len(snapshot.WindowOrder) != len(snapshot.Windows) {
		t.Errorf("windowOrder (%d) != windows count (%d)", len(snapshot.WindowOrder), len(snapshot.Windows))
	}

	// Each window in order must exist
	windowIDs := make(map[uint32]struct{}, len(snapshot.Windows))
	for _, win := range snapshot.Windows {
		windowIDs[win.ID] = struct{}{}

		// Pane order consistency
		if len(win.PaneOrder) != len(win.Panes) {
			t.Errorf("window %d: paneOrder (%d) != panes count (%d)", win.ID, len(win.PaneOrder), len(win.Panes))
		}

		// Active pane must exist in panes
		if win.ActivePane != 0 {
			found := false
			for _, pane := range win.Panes {
				if pane.ID == win.ActivePane {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("window %d: active pane %d not found in panes", win.ID, win.ActivePane)
			}
		}
	}

	// Active window must exist
	if snapshot.ActiveWindow != 0 {
		if _, ok := windowIDs[snapshot.ActiveWindow]; !ok {
			t.Errorf("active window %d not found in windows", snapshot.ActiveWindow)
		}
	}

	// Window order entries must all exist
	for _, wid := range snapshot.WindowOrder {
		if _, ok := windowIDs[wid]; !ok {
			t.Errorf("window order references missing window %d", wid)
		}
	}
}

// MessageRoutingInvariant validates that the actor message system is functioning.
func AssertMessageRoutingInvariant(t *testing.T, sessionRef *shux.SessionRef) {
	t.Helper()

	// Basic ask/reply should work
	result := <-sessionRef.Ask(shux.GetSessionSnapshotData{})
	if result == nil {
		t.Fatal("message routing failed: nil response to GetSessionSnapshotData")
	}

	// Send should not block or panic
	done := make(chan struct{})
	go func() {
		defer close(done)
		sessionRef.Send(shux.CreateWindow{Rows: 10, Cols: 10})
	}()

	select {
	case <-done:
		// OK
	case <-time.After(100 * time.Millisecond):
		t.Fatal("message routing failed: Send blocked")
	}
}

// RecoveryInvariant validates post-recovery state is consistent.
type RecoveryInvariant struct {
	ExpectedWindows int
	ExpectedPanes   map[uint32]int // windowID -> pane count
}

// AssertRecoveryInvariant validates state after crash/recovery.
func AssertRecoveryInvariant(t *testing.T, sessionRef *shux.SessionRef, inv RecoveryInvariant) {
	t.Helper()

	result := <-sessionRef.Ask(shux.GetSessionSnapshotData{})
	data, ok := result.(shux.SessionSnapshotData)
	if !ok {
		t.Fatalf("expected SessionSnapshotData, got %T", result)
	}

	if len(data.WindowOrder) != inv.ExpectedWindows {
		t.Errorf("recovery: window count = %d, want %d", len(data.WindowOrder), inv.ExpectedWindows)
	}
}

// PersistenceInvariant validates that state survives save/load cycle.
func AssertPersistenceInvariant(t *testing.T, original, restored *shux.SessionSnapshot) {
	t.Helper()

	if original == nil || restored == nil {
		t.Fatal("persistence invariant: nil snapshot")
	}

	if original.ID != restored.ID {
		t.Errorf("persistence: session ID mismatch: %d vs %d", original.ID, restored.ID)
	}

	if original.SessionName != restored.SessionName {
		t.Errorf("persistence: session name mismatch: %q vs %q", original.SessionName, restored.SessionName)
	}

	if len(original.Windows) != len(restored.Windows) {
		t.Errorf("persistence: window count mismatch: %d vs %d", len(original.Windows), len(restored.Windows))
	}

	if len(original.WindowOrder) != len(restored.WindowOrder) {
		t.Errorf("persistence: window order length mismatch: %d vs %d", len(original.WindowOrder), len(restored.WindowOrder))
	}

	// Check window order preservation
	for i, wid := range original.WindowOrder {
		if i >= len(restored.WindowOrder) {
			break
		}
		if restored.WindowOrder[i] != wid {
			t.Errorf("persistence: window order[%d] = %d, want %d", i, restored.WindowOrder[i], wid)
		}
	}
}

// DrainEvents drains all pending events from a supervisor channel.
func DrainEvents(ch chan any, timeout time.Duration) []any {
	var events []any
	deadline := time.After(timeout)
	done := false
	for !done {
		select {
		case evt := <-ch:
			events = append(events, evt)
		case <-deadline:
			done = true
		default:
			done = true
		}
	}
	return events
}

// FormatInvariantFailure formats an invariant failure message.
func FormatInvariantFailure(invType string, details string) string {
	return fmt.Sprintf("INVARIANT VIOLATION [%s]: %s", invType, details)
}

// FatalOnInvariantViolation fails the test immediately on invariant violation.
// This is Tiger Style - crash on unexpected state.
func FatalOnInvariantViolation(t *testing.T, invType string, format string, args ...interface{}) {
	t.Helper()
	msg := FormatInvariantFailure(invType, fmt.Sprintf(format, args...))
	t.Fatal(msg)
}
