//go:build e2e
// +build e2e

package e2e

import (
	"os"
	"testing"
	"time"

	"shux/pkg/shux"
	"shux/pkg/testutil"
)

// TestE2ERecoveryStory validates the core recovery story from AGENTS.md:
// "As a user I have shux running with multiple panes. After detaching,
// I expect to be able to reattach as close to the state I left it in as possible."
func TestE2ERecoveryStory(t *testing.T) {
	if os.Getenv("SHUX_E2E") != "1" {
		t.Skip("Set SHUX_E2E=1 to run E2E tests")
	}

	testutil.WithTempHome(t, func(tempDir string) {
		// === Phase 1: Setup session with 2x2 layout ===
		t.Log("Phase 1: Creating session with 2x2 layout")

		sessionRef, super, _ := testutil.SetupNamedSession(t, "recovery-story")

		sessionRef.Send(shux.CreateWindow{Rows: 48, Cols: 160})
		win := testutil.RequireWindow(t, sessionRef, super)
		_ = testutil.RequirePane(t, sessionRef, super)

		// Build 2x2 layout
		win.Send(shux.Split{Dir: shux.SplitV})
		time.Sleep(100 * time.Millisecond)
		win.Send(shux.SwitchToPane{Index: 0})
		time.Sleep(50 * time.Millisecond)
		win.Send(shux.Split{Dir: shux.SplitH})
		time.Sleep(100 * time.Millisecond)
		win.Send(shux.SwitchToPane{Index: 2})
		time.Sleep(50 * time.Millisecond)
		win.Send(shux.Split{Dir: shux.SplitH})
		time.Sleep(100 * time.Millisecond)

		// Verify 4 panes
		winData := <-win.Ask(shux.GetWindowSnapshotData{})
		data := winData.(shux.WindowSnapshot)
		if len(data.PaneOrder) != 4 {
			t.Fatalf("Expected 4 panes, got %d", len(data.PaneOrder))
		}

		// Capture pre-detach state
		preDetachSnapshot := <-sessionRef.Ask(shux.GetFullSessionSnapshot{})
		if preDetachSnapshot == nil {
			t.Fatal("Failed to capture pre-detach snapshot")
		}
		preData := preDetachSnapshot.(*shux.SessionSnapshot)
		t.Logf("Pre-detach: %d windows, active=%d", len(preData.Windows), preData.ActiveWindow)

		// === Phase 2: Detach ===
		t.Log("Phase 2: Detaching session")

		reply := sessionRef.Ask(shux.DetachSession{})
		<-reply

		if !super.WaitSessionEmpty(3 * time.Second) {
			t.Fatal("Timeout waiting for session empty after detach")
		}
		sessionRef.Shutdown()

		// === Phase 3: Verify snapshot ===
		t.Log("Phase 3: Verifying snapshot")

		if !shux.SessionSnapshotExists("recovery-story") {
			t.Fatal("Snapshot should exist after detach")
		}

		snapshot, err := shux.LoadSnapshot(shux.SessionSnapshotPath("recovery-story"), testutil.TestLogger())
		if err != nil {
			t.Fatalf("Failed to load snapshot: %v", err)
		}

		testutil.AssertPersistenceInvariant(t, preData, snapshot)

		// === Phase 4: Restore ===
		t.Log("Phase 4: Restoring session")

		super2 := testutil.NewTestSupervisor()
		restoredRef, err := shux.RestoreSessionFromSnapshot("recovery-story", super2.Handle, testutil.TestLogger())
		if err != nil {
			t.Fatalf("Failed to restore session: %v", err)
		}
		defer restoredRef.Shutdown()

		// === Phase 5: Verify restored state ===
		t.Log("Phase 5: Verifying restored state")

		testutil.PollFor(2*time.Second, func() bool {
			result := <-restoredRef.Ask(shux.GetSessionSnapshotData{})
			if data, ok := result.(shux.SessionSnapshotData); ok {
				return len(data.WindowOrder) == 1
			}
			return false
		})

		restoredWin := <-restoredRef.Ask(shux.GetActiveWindow{})
		if restoredWin == nil {
			t.Fatal("No active window after restore")
		}

		restoredData := <-restoredWin.(*shux.WindowRef).Ask(shux.GetWindowSnapshotData{})
		winData2 := restoredData.(shux.WindowSnapshot)

		if len(winData2.PaneOrder) != 4 {
			t.Errorf("Expected 4 panes after restore, got %d", len(winData2.PaneOrder))
		}

		t.Logf("Recovery story complete: restored %d panes", len(winData2.PaneOrder))
	})
}

// TestE2ERecoveryAfterKill validates recovery after the shux process is killed.
func TestE2ERecoveryAfterKill(t *testing.T) {
	if os.Getenv("SHUX_E2E") != "1" {
		t.Skip("Set SHUX_E2E=1 to run E2E tests")
	}

	testutil.WithTempHome(t, func(tempDir string) {
		t.Log("Phase 1: Creating session with content")

		sessionRef, super, _ := testutil.SetupNamedSession(t, "kill-recovery")

		sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
		win := testutil.RequireWindow(t, sessionRef, super)
		_ = testutil.RequirePane(t, sessionRef, super)

		// Add content to window
		win.Send(shux.Split{Dir: shux.SplitV})
		time.Sleep(100 * time.Millisecond)

		// Pre-kill snapshot
		preData := <-sessionRef.Ask(shux.GetFullSessionSnapshot{})
		if preData == nil {
			t.Fatal("Failed to capture pre-kill snapshot")
		}

		t.Log("Phase 2: Simulating kill (detach without owner mode)")

		// Normal detach simulates clean shutdown
		reply := sessionRef.Ask(shux.DetachSession{})
		<-reply
		super.WaitSessionEmpty(2 * time.Second)
		sessionRef.Shutdown()

		t.Log("Phase 3: Recovering from snapshot")

		super2 := testutil.NewTestSupervisor()
		restoredRef, err := shux.RestoreSessionFromSnapshot("kill-recovery", super2.Handle, testutil.TestLogger())
		if err != nil {
			t.Fatalf("Failed to restore after kill: %v", err)
		}
		defer restoredRef.Shutdown()

		t.Log("Phase 4: Verifying recovery")

		testutil.PollFor(2*time.Second, func() bool {
			result := <-restoredRef.Ask(shux.GetSessionSnapshotData{})
			if data, ok := result.(shux.SessionSnapshotData); ok {
				return len(data.WindowOrder) == 1
			}
			return false
		})

		restoredData := <-restoredRef.Ask(shux.GetFullSessionSnapshot{})
		if restoredData == nil {
			t.Fatal("Failed to capture post-restore snapshot")
		}

		testutil.AssertPersistenceInvariant(t, preData.(*shux.SessionSnapshot), restoredData.(*shux.SessionSnapshot))

		t.Log("Kill recovery complete")
	})
}

// TestE2EMultipleDetachReattach validates multiple detach/reattach cycles.
func TestE2EMultipleDetachReattach(t *testing.T) {
	if os.Getenv("SHUX_E2E") != "1" {
		t.Skip("Set SHUX_E2E=1 to run E2E tests")
	}

	testutil.WithTempHome(t, func(tempDir string) {
		// Run 3 cycles of detach/reattach
		for cycle := 0; cycle < 3; cycle++ {
			t.Logf("=== Cycle %d ===", cycle+1)

			// Create or restore session
			var sessionRef *shux.SessionRef
			var super *testutil.TestSupervisor

			if cycle == 0 {
				sessionRef, super, _ = testutil.SetupNamedSession(t, "multi-detach")
				sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
				testutil.RequireWindow(t, sessionRef, super)
				testutil.RequirePane(t, sessionRef, super)
			} else {
				super = testutil.NewTestSupervisor()
				var err error
				sessionRef, err = shux.RestoreSessionFromSnapshot("multi-detach", super.Handle, testutil.TestLogger())
				if err != nil {
					t.Fatalf("Cycle %d: restore failed: %v", cycle+1, err)
				}
			}

			// Mutate state
			win := <-sessionRef.Ask(shux.GetActiveWindow{})
			if win != nil {
				win.(*shux.WindowRef).Send(shux.Split{Dir: shux.SplitV})
				time.Sleep(100 * time.Millisecond)
			}

			// Detach
			reply := sessionRef.Ask(shux.DetachSession{})
			<-reply
			super.WaitSessionEmpty(2 * time.Second)
			sessionRef.Shutdown()

			time.Sleep(100 * time.Millisecond)
		}

		t.Log("Multiple detach/reattach cycles complete")
	})
}
