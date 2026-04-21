package integration

import (
	"errors"
	"os"
	"testing"
	"time"

	"shux/pkg/shux"
	"shux/pkg/testutil"
)

// TestRecoverySessionRestart validates session can recover from restart.
func TestRecoverySessionRestart(t *testing.T) {
	testutil.WithTempHome(t, func(tempDir string) {
		// Create and populate session
		sessionRef, super, cleanup := testutil.SetupNamedSession(t, "restart-test")

		sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
		win := testutil.RequireWindow(t, sessionRef, super)
		_ = testutil.RequirePane(t, sessionRef, super)

		win.Send(shux.Split{Dir: shux.SplitV})
		testutil.PollFor(200*time.Millisecond, func() bool {
			result := <-win.Ask(shux.GetWindowSnapshotData{})
			if data, ok := result.(shux.WindowSnapshot); ok {
				return len(data.PaneOrder) == 2
			}
			return false
		})

		// Detach (simulates session "crash" and restart)
		reply := sessionRef.Ask(shux.DetachSession{})
		<-reply
		super.WaitSessionEmpty(2 * time.Second)
		cleanup()

		// "Recover" by restoring from snapshot
		super2 := testutil.NewTestSupervisor()
		recoveredRef, err := shux.RestoreSessionFromSnapshot("restart-test", super2.Handle, testutil.TestLogger())
		if err != nil {
			t.Fatalf("restore session: %v", err)
		}
		defer recoveredRef.Shutdown()

		// Verify recovery
		testutil.PollFor(time.Second, func() bool {
			result := <-recoveredRef.Ask(shux.GetSessionSnapshotData{})
			if data, ok := result.(shux.SessionSnapshotData); ok {
				return len(data.WindowOrder) == 1
			}
			return false
		})

		recoveredWin := <-recoveredRef.Ask(shux.GetActiveWindow{})
		winData := <-recoveredWin.(*shux.WindowRef).Ask(shux.GetWindowSnapshotData{})
		data := winData.(shux.WindowSnapshot)

		if len(data.PaneOrder) != 2 {
			t.Errorf("expected 2 panes after recovery, got %d", len(data.PaneOrder))
		}

		testutil.AssertRecoveryInvariant(t, recoveredRef, testutil.RecoveryInvariant{
			ExpectedWindows: 1,
			ExpectedPanes:   map[uint32]int{1: 2},
		})
	})
}

// TestRecoveryPaneCrash validates session survives a pane crash.
func TestRecoveryPaneCrash(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	win := testutil.RequireWindow(t, sessionRef, super)
	_ = testutil.RequirePane(t, sessionRef, super)

	// Create multiple panes
	for i := 0; i < 2; i++ {
		win.Send(shux.CreatePane{Rows: 24, Cols: 80, Shell: "/bin/sh"})
		testutil.WaitWindowPaneCount(t, win, i+2, 200*time.Millisecond)
	}

	// Get active pane and kill it (simulates controller/runtime death)
	result := <-sessionRef.Ask(shux.GetActivePane{})
	pane := result.(*shux.PaneRef)
	pane.Send(shux.KillPane{})

	// Verify window recovers with 2 panes remaining
	testutil.WaitWindowPaneCount(t, win, 2, time.Second)

	// Session should still be valid
	testutil.AssertSessionInvariants(t, sessionRef, testutil.SessionInvariant{
		SessionID:   1,
		WindowCount: 1,
	})
}

// TestRecoveryLastPaneCrash validates session ends correctly when last pane crashes.
func TestRecoveryLastPaneCrash(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	_ = testutil.RequirePane(t, sessionRef, super)

	// Get and kill the only pane
	result := <-sessionRef.Ask(shux.GetActivePane{})
	pane := result.(*shux.PaneRef)
	pane.Send(shux.KillPane{})

	// Session should signal empty
	if !super.WaitSessionEmpty(2 * time.Second) {
		t.Error("expected SessionEmpty after killing last pane")
	}
}

// TestRecoveryShellProcessDeath validates real child-process death is contained
// to the affected pane while the session remains usable.
func TestRecoveryShellProcessDeath(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	win := testutil.RequireWindow(t, sessionRef, super)
	_ = testutil.RequirePane(t, sessionRef, super)

	win.Send(shux.Split{Dir: shux.SplitV})
	pre := testutil.WaitWindowPaneCount(t, win, 2, time.Second)
	victimID := pre.ActivePane

	paneResult := <-sessionRef.Ask(shux.GetActivePane{})
	victim := paneResult.(*shux.PaneRef)
	victim.Send(shux.WriteToPane{Data: []byte("kill -9 $$\r")})

	post := testutil.WaitWindowPaneCount(t, win, 1, 2*time.Second)
	for _, paneID := range post.PaneOrder {
		if paneID == victimID {
			t.Fatalf("dead pane %d should have been removed after shell death", victimID)
		}
	}

	if got := <-sessionRef.Ask(shux.GetActiveWindow{}); got == nil {
		t.Fatal("session lost active window after shell death")
	}
}

// TestRecoveryLastShellProcessDeath validates real process death of the final
// pane results in an empty session notification.
func TestRecoveryLastShellProcessDeath(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	_ = testutil.RequirePane(t, sessionRef, super)

	paneResult := <-sessionRef.Ask(shux.GetActivePane{})
	pane := paneResult.(*shux.PaneRef)
	pane.Send(shux.WriteToPane{Data: []byte("kill -9 $$\r")})

	if !super.WaitSessionEmpty(3 * time.Second) {
		t.Fatal("expected SessionEmpty after shell process killed itself")
	}
}

// TestRecoveryWindowCrash validates session survives window crash.
func TestRecoveryWindowCrash(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	// Create two windows
	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	testutil.RequireWindow(t, sessionRef, super)

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	win2 := testutil.RequireWindow(t, sessionRef, super)
	pane2 := testutil.RequirePane(t, sessionRef, super)

	// Kill pane in window 2 (causes window empty)
	pane2.Send(shux.KillPane{})
	time.Sleep(100 * time.Millisecond)

	// Session should have 1 window
	testutil.PollFor(time.Second, func() bool {
		result := <-sessionRef.Ask(shux.GetSessionSnapshotData{})
		if data, ok := result.(shux.SessionSnapshotData); ok {
			return len(data.WindowOrder) == 1
		}
		return false
	})

	// Verify active window is still valid
	result := <-sessionRef.Ask(shux.GetActiveWindow{})
	if result == nil {
		t.Error("expected valid active window after other window crashed")
	}

	if result.(*shux.WindowRef) == win2 {
		t.Error("active window should have switched from closed window")
	}
}

// TestRecoverySnapshotValidation validates corrupted snapshots are rejected.
func TestRecoverySnapshotValidation(t *testing.T) {
	testutil.WithTempHome(t, func(tempDir string) {
		// Create invalid snapshot (window count mismatch)
		invalidSnapshot := &shux.SessionSnapshot{
			Version:      shux.SnapshotVersion,
			SessionName:  "invalid-test",
			ID:           1,
			Shell:        "/bin/sh",
			ActiveWindow: 999, // Non-existent
			WindowOrder:  []uint32{1, 2, 3},
			Windows:      []shux.WindowSnapshot{}, // Empty but order says 3
		}

		if err := shux.EnsureSessionDir("invalid-test"); err != nil {
			t.Fatalf("ensure session dir: %v", err)
		}
		if err := shux.SaveSnapshot(shux.SessionSnapshotPath("invalid-test"), invalidSnapshot, testutil.TestLogger()); err != nil {
			t.Fatalf("save snapshot: %v", err)
		}

		// Attempt to restore - should fail validation
		super := testutil.NewTestSupervisor()
		_, err := shux.RestoreSessionFromSnapshot("invalid-test", super.Handle, testutil.TestLogger())
		if err == nil {
			t.Error("expected error restoring invalid snapshot")
		}
	})
}

// TestRecoveryCorruptedSnapshotData validates corrupted data doesn't crash restore.
func TestRecoveryCorruptedSnapshotData(t *testing.T) {
	testutil.WithTempHome(t, func(tempDir string) {
		// Create corrupted snapshot file
		if err := shux.EnsureSessionDir("corrupt-test"); err != nil {
			t.Fatalf("ensure session dir: %v", err)
		}

		// Write garbage data
		path := shux.SessionSnapshotPath("corrupt-test")
		if err := os.WriteFile(path, []byte("not a valid gob"), 0o644); err != nil {
			t.Fatalf("write corrupt file: %v", err)
		}

		// Attempt to load - should return error, not panic
		_, err := shux.LoadSnapshot(path, testutil.TestLogger())
		if err == nil {
			t.Error("expected error loading corrupted snapshot")
		}
	})
}

// TestRecoveryActorRestart validates actor message system survives restart.
func TestRecoveryActorRestart(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	win := testutil.RequireWindow(t, sessionRef, super)
	_ = testutil.RequirePane(t, sessionRef, super)

	// Send many messages to stress the actor system
	for i := 0; i < 20; i++ {
		win.Send(shux.NavigatePane{Dir: shux.PaneNavRight})
		time.Sleep(5 * time.Millisecond)
	}

	// System should still be responsive
	result := <-win.Ask(shux.GetWindowSnapshotData{})
	if result == nil {
		t.Fatal("window should still respond after message burst")
	}
}

// TestRecoveryConcurrentAccess validates concurrent access patterns.
func TestRecoveryConcurrentAccess(t *testing.T) {
	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	win := testutil.RequireWindow(t, sessionRef, super)
	_ = testutil.RequirePane(t, sessionRef, super)

	// Create multiple panes for concurrent access
	win.Send(shux.Split{Dir: shux.SplitV})
	testutil.PollFor(200*time.Millisecond, func() bool {
		result := <-win.Ask(shux.GetWindowSnapshotData{})
		if data, ok := result.(shux.WindowSnapshot); ok {
			return len(data.PaneOrder) == 2
		}
		return false
	})

	// Concurrent operations from multiple goroutines
	done := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				result := <-sessionRef.Ask(shux.GetSessionSnapshotData{})
				if result == nil {
					done <- errors.New("nil result from concurrent ask")
					return
				}
				time.Sleep(5 * time.Millisecond)
			}
			done <- nil
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 5; i++ {
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("concurrent access error: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("concurrent access timeout")
		}
	}
}

// TestRecoveryPartialState validates partial state handling.
func TestRecoveryPartialState(t *testing.T) {
	testutil.WithTempHome(t, func(tempDir string) {
		// Create snapshot with minimal state
		minimalSnapshot := &shux.SessionSnapshot{
			Version:      shux.SnapshotVersion,
			SessionName:  "minimal-test",
			ID:           1,
			Shell:        "/bin/sh",
			ActiveWindow: 0,
			WindowOrder:  []uint32{},
			Windows:      []shux.WindowSnapshot{},
		}

		if err := shux.EnsureSessionDir("minimal-test"); err != nil {
			t.Fatalf("ensure session dir: %v", err)
		}
		if err := shux.SaveSnapshot(shux.SessionSnapshotPath("minimal-test"), minimalSnapshot, testutil.TestLogger()); err != nil {
			t.Fatalf("save snapshot: %v", err)
		}

		// Restore minimal session
		super := testutil.NewTestSupervisor()
		recoveredRef, err := shux.RestoreSessionFromSnapshot("minimal-test", super.Handle, testutil.TestLogger())
		if err != nil {
			t.Fatalf("restore minimal session: %v", err)
		}
		defer recoveredRef.Shutdown()

		// Verify empty but valid session
		testutil.PollFor(time.Second, func() bool {
			result := <-recoveredRef.Ask(shux.GetSessionSnapshotData{})
			if data, ok := result.(shux.SessionSnapshotData); ok {
				return data.ID == 1 && len(data.WindowOrder) == 0
			}
			return false
		})
	})
}
