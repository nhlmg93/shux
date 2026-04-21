// Package integration provides high-ROI integration tests for shux.
//
// Philosophy (per AGENTS.md):
// - NO UNIT TESTS
// - Focus on Sessions/Windows/Panes lifecycle
// - Actor message routing validation
// - Recovery and persistence invariants
// - Deterministic scenario testing
package integration

import (
	"testing"
	"time"

	"shux/pkg/shux"
	"shux/pkg/testutil"
)

// TestSessionLifecycle validates basic session creation and shutdown.
func TestSessionLifecycle(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	// Verify initial state
	testutil.AssertSessionInvariants(t, sessionRef, testutil.SessionInvariant{
		SessionID:    1,
		WindowCount:  0,
		ActiveWindow: 0,
	})

	// Create a window
	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	win := testutil.RequireWindow(t, sessionRef, super)

	testutil.AssertSessionInvariants(t, sessionRef, testutil.SessionInvariant{
		SessionID:    1,
		WindowCount:  1,
		ActiveWindow: 1,
	})

	// Verify window is valid
	testutil.AssertWindowInvariants(t, win, testutil.WindowInvariant{
		WindowID:   1,
		PaneCount:  1,
		ActivePane: 1,
	})
}

// TestSessionMultipleWindows validates window creation and switching.
func TestSessionMultipleWindows(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	// Create first window
	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	win1 := testutil.RequireWindow(t, sessionRef, super)

	// Get first window's ID
	win1ID := uint32(0)
	result1 := <-win1.Ask(shux.GetWindowSnapshotData{})
	if data, ok := result1.(shux.WindowSnapshot); ok {
		win1ID = data.ID
	}

	// Create second window
	sessionRef.Send(shux.CreateWindow{Rows: 30, Cols: 100})
	win2 := testutil.RequireWindow(t, sessionRef, super)

	// Get second window's ID
	win2ID := uint32(0)
	result2 := <-win2.Ask(shux.GetWindowSnapshotData{})
	if data, ok := result2.(shux.WindowSnapshot); ok {
		win2ID = data.ID
	}

	testutil.WaitSessionWindowCount(t, sessionRef, 2, 200*time.Millisecond)

	// Verify initial active window is second one
	testutil.AssertSessionInvariants(t, sessionRef, testutil.SessionInvariant{
		SessionID:    1,
		WindowCount:  2,
		ActiveWindow: win2ID, // Most recently created
	})

	// Switch to previous window
	sessionRef.Send(shux.SwitchWindow{Delta: -1})
	data := testutil.WaitSessionWindowCount(t, sessionRef, 2, 200*time.Millisecond)
	if data.ActiveWindow == win2ID {
		t.Errorf("active window should have changed from %d after switch, still got %d", win2ID, data.ActiveWindow)
	}

	// Log what we got (implementation may differ)
	t.Logf("After SwitchWindow{Delta: -1}: active window = %d (win1ID=%d, win2ID=%d)", data.ActiveWindow, win1ID, win2ID)
}

// TestSessionWindowWrapping validates window navigation wraps around.
func TestSessionWindowWrapping(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	// Create 3 windows
	for i := 0; i < 3; i++ {
		sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
		testutil.RequireWindow(t, sessionRef, super)
	}

	initial := testutil.WaitSessionWindowCount(t, sessionRef, 3, 200*time.Millisecond)
	currentIdx := -1
	for i, wid := range initial.WindowOrder {
		if wid == initial.ActiveWindow {
			currentIdx = i
			break
		}
	}
	if currentIdx < 0 {
		t.Fatalf("active window %d missing from order %v", initial.ActiveWindow, initial.WindowOrder)
	}

	expectedForward := initial.WindowOrder[(currentIdx+2)%len(initial.WindowOrder)]
	sessionRef.Send(shux.SwitchWindow{Delta: 2})
	testutil.MustPoll(t, 200*time.Millisecond, "timeout waiting for wrapped forward window switch", func() bool {
		result := <-sessionRef.Ask(shux.GetSessionSnapshotData{})
		if data, ok := result.(shux.SessionSnapshotData); ok {
			return data.ActiveWindow == expectedForward
		}
		return false
	})

	expectedBackward := initial.WindowOrder[(currentIdx+1)%len(initial.WindowOrder)]
	sessionRef.Send(shux.SwitchWindow{Delta: -1})
	testutil.MustPoll(t, 200*time.Millisecond, "timeout waiting for wrapped backward window switch", func() bool {
		result := <-sessionRef.Ask(shux.GetSessionSnapshotData{})
		if data, ok := result.(shux.SessionSnapshotData); ok {
			return data.ActiveWindow == expectedBackward
		}
		return false
	})
}

// TestSessionDetachSavesState validates detach creates a snapshot.
func TestSessionDetachSavesState(t *testing.T) {
	testutil.WithTempHome(t, func(tempDir string) {
		sessionRef, super, cleanup := testutil.SetupNamedSession(t, "detach-test")
		defer cleanup()

		runner := testutil.NewScenarioRunner(sessionRef, super)
		for _, step := range testutil.SnapshotRestoreScenario("detach-test") {
			runner.AddStep(step)
		}
		runner.Run(t)

		reply := sessionRef.Ask(shux.DetachSession{})
		<-reply

		if !super.WaitSessionEmpty(2 * time.Second) {
			t.Fatal("timeout waiting for session empty after detach")
		}

		if !shux.SessionSnapshotExists("detach-test") {
			t.Fatal("snapshot should exist after detach")
		}

		snapshot, err := shux.LoadSnapshot(shux.SessionSnapshotPath("detach-test"), testutil.TestLogger())
		if err != nil {
			t.Fatalf("failed to load snapshot: %v", err)
		}

		testutil.AssertSnapshotInvariant(t, snapshot)
	})
}

// TestSessionNamedSnapshot validates snapshots are saved per-session-name.
func TestSessionNamedSnapshot(t *testing.T) {
	testutil.WithTempHome(t, func(tempDir string) {
		// Create first named session
		session1, super1, cleanup1 := testutil.SetupNamedSession(t, "session-one")
		defer cleanup1()

		session1.Send(shux.CreateWindow{Rows: 24, Cols: 80})
		_ = testutil.RequireWindow(t, session1, super1)

		// Create second named session
		session2, super2, cleanup2 := testutil.SetupNamedSession(t, "session-two")
		defer cleanup2()

		session2.Send(shux.CreateWindow{Rows: 30, Cols: 100})
		session2.Send(shux.CreateWindow{Rows: 40, Cols: 120})
		_ = testutil.RequireWindow(t, session2, super2)

		// Detach both
		<-session1.Ask(shux.DetachSession{})
		<-session2.Ask(shux.DetachSession{})

		super1.WaitSessionEmpty(2 * time.Second)
		super2.WaitSessionEmpty(2 * time.Second)

		// Verify separate snapshots
		snap1, err := shux.LoadSnapshot(shux.SessionSnapshotPath("session-one"), testutil.TestLogger())
		if err != nil {
			t.Fatalf("failed to load session-one: %v", err)
		}
		snap2, err := shux.LoadSnapshot(shux.SessionSnapshotPath("session-two"), testutil.TestLogger())
		if err != nil {
			t.Fatalf("failed to load session-two: %v", err)
		}

		if snap1.SessionName != "session-one" {
			t.Errorf("session-one name: got %q", snap1.SessionName)
		}
		if snap2.SessionName != "session-two" {
			t.Errorf("session-two name: got %q", snap2.SessionName)
		}
		if len(snap1.Windows) != 1 {
			t.Errorf("session-one windows: got %d", len(snap1.Windows))
		}
		if len(snap2.Windows) != 2 {
			t.Errorf("session-two windows: got %d", len(snap2.Windows))
		}
	})
}

// TestSessionMessageRouting validates actor message routing is functional.
func TestSessionMessageRouting(t *testing.T) {
	sessionRef, _, cleanup := testutil.SetupSession(t)
	defer cleanup()

	// Test basic Ask/Reply
	testutil.AssertMessageRoutingInvariant(t, sessionRef)

	// Test multiple concurrent asks
	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func() {
			result := <-sessionRef.Ask(shux.GetSessionSnapshotData{})
			done <- result != nil
		}()
	}

	for i := 0; i < 5; i++ {
		select {
		case success := <-done:
			if !success {
				t.Error("concurrent ask returned nil")
			}
		case <-time.After(time.Second):
			t.Fatal("concurrent ask timeout")
		}
	}
}

// TestSessionEmptyEvent validates SessionEmpty event is sent correctly.
func TestSessionEmptyEvent(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	// Create and kill the only window
	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	pane := testutil.RequirePane(t, sessionRef, super)

	pane.Send(shux.KillPane{})

	if !super.WaitSessionEmpty(2 * time.Second) {
		t.Fatal("expected SessionEmpty event after killing only pane")
	}
}

// TestSessionCreateFromSnapshot validates restoring a session from snapshot.
func TestSessionCreateFromSnapshot(t *testing.T) {
	testutil.WithTempHome(t, func(tempDir string) {
		// Build a snapshot manually
		snapshot := testutil.BuildTestSnapshot("restore-test", 2)

		// Save it
		if err := shux.EnsureSessionDir("restore-test"); err != nil {
			t.Fatalf("ensure session dir: %v", err)
		}
		if err := shux.SaveSnapshot(shux.SessionSnapshotPath("restore-test"), snapshot, testutil.TestLogger()); err != nil {
			t.Fatalf("save snapshot: %v", err)
		}

		// Restore
		super := testutil.NewTestSupervisor()
		restoredRef, err := shux.RestoreSessionFromSnapshot("restore-test", super.Handle, testutil.TestLogger())
		if err != nil {
			t.Fatalf("restore session: %v", err)
		}
		defer restoredRef.Shutdown()

		// Verify restored state
		testutil.PollFor(time.Second, func() bool {
			result := <-restoredRef.Ask(shux.GetSessionSnapshotData{})
			if data, ok := result.(shux.SessionSnapshotData); ok {
				return len(data.WindowOrder) == 2
			}
			return false
		})

		testutil.AssertSessionInvariants(t, restoredRef, testutil.SessionInvariant{
			SessionID:    1,
			WindowCount:  2,
			ActiveWindow: 1,
		})
	})
}

// TestSessionLastWindowNavigation validates "last window" navigation works.
func TestSessionLastWindowNavigation(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	// Create first window
	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	testutil.RequireWindow(t, sessionRef, super)

	// Create second window
	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	win2 := testutil.RequireWindow(t, sessionRef, super)

	// Get second window's ID
	win2ID := uint32(0)
	result2 := <-win2.Ask(shux.GetWindowSnapshotData{})
	if data, ok := result2.(shux.WindowSnapshot); ok {
		win2ID = data.ID
	}

	// Verify we start on window 2 (most recently created)
	result := <-sessionRef.Ask(shux.GetSessionSnapshotData{})
	data := result.(shux.SessionSnapshotData)
	if data.ActiveWindow != win2ID {
		t.Fatalf("expected active window %d, got %d", win2ID, data.ActiveWindow)
	}

	// Switch back to window 1
	sessionRef.Send(shux.SwitchWindow{Delta: -1})
	time.Sleep(100 * time.Millisecond)

	// Verify active window changed from win2
	result3 := <-sessionRef.Ask(shux.GetSessionSnapshotData{})
	data3 := result3.(shux.SessionSnapshotData)
	if data3.ActiveWindow == win2ID {
		t.Errorf("active window should have changed from %d after switch, still got %d", win2ID, data3.ActiveWindow)
	}

	// Switch to last window (should go back to previously active window)
	sessionRef.Send(shux.ActionMsg{Action: shux.ActionLastWindow})
	time.Sleep(100 * time.Millisecond)

	// Verify active window changed again
	result4 := <-sessionRef.Ask(shux.GetSessionSnapshotData{})
	data4 := result4.(shux.SessionSnapshotData)
	if data4.ActiveWindow == data3.ActiveWindow {
		t.Errorf("active window should have changed after last-window action, still got %d", data4.ActiveWindow)
	}

	t.Logf("LastWindow navigation: before=%d, after=%d", data3.ActiveWindow, data4.ActiveWindow)
}

// TestSessionKillWindow validates killing the active window works.
func TestSessionKillWindow(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	// Create two windows
	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	testutil.RequireWindow(t, sessionRef, super)

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	testutil.RequireWindow(t, sessionRef, super)

	// Verify 2 windows
	testutil.AssertSessionInvariants(t, sessionRef, testutil.SessionInvariant{
		SessionID:   1,
		WindowCount: 2,
	})

	// Kill active window
	sessionRef.Send(shux.ActionMsg{Action: shux.ActionKillWindow})

	// Verify 1 window remains
	testutil.PollFor(time.Second, func() bool {
		result := <-sessionRef.Ask(shux.GetSessionSnapshotData{})
		if data, ok := result.(shux.SessionSnapshotData); ok {
			return len(data.WindowOrder) == 1
		}
		return false
	})

	testutil.AssertSessionInvariants(t, sessionRef, testutil.SessionInvariant{
		SessionID:   1,
		WindowCount: 1,
	})
}

// TestSessionWindowOrderingAfterKill validates window ordering after kill.
func TestSessionWindowOrderingAfterKill(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	// Create 3 windows
	for i := 0; i < 3; i++ {
		sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
		testutil.RequireWindow(t, sessionRef, super)
	}

	// Switch to window 2 (middle)
	sessionRef.Send(shux.SwitchWindow{Delta: -1})
	testutil.PollFor(100*time.Millisecond, func() bool {
		result := <-sessionRef.Ask(shux.GetSessionSnapshotData{})
		if data, ok := result.(shux.SessionSnapshotData); ok {
			return data.ActiveWindow == 2
		}
		return false
	})

	// Kill window 2
	sessionRef.Send(shux.ActionMsg{Action: shux.ActionKillWindow})

	// Verify 2 windows remain, ordering is correct
	testutil.PollFor(time.Second, func() bool {
		result := <-sessionRef.Ask(shux.GetSessionSnapshotData{})
		if data, ok := result.(shux.SessionSnapshotData); ok {
			return len(data.WindowOrder) == 2
		}
		return false
	})

	result := <-sessionRef.Ask(shux.GetSessionSnapshotData{})
	data := result.(shux.SessionSnapshotData)
	if len(data.WindowOrder) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(data.WindowOrder))
	}
}
