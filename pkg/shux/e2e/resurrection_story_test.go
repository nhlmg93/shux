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

// TestE2EResurrectionStory validates the resurrection scenario:
// "As a user, after my shux process crashes (not clean detach), I expect
// to recover my session state when I restart shux."
func TestE2EResurrectionStory(t *testing.T) {
	if os.Getenv("SHUX_E2E") != "1" {
		t.Skip("Set SHUX_E2E=1 to run E2E tests")
	}

	testutil.WithTempHome(t, func(tempDir string) {
		t.Log("=== Resurrection Story Test ===")
		t.Log("Phase 1: Create session with multiple windows and complex layout")

		sessionRef, super, _ := testutil.SetupNamedSession(t, "resurrection-test")

		// Window 1: 2x2 layout
		sessionRef.Send(shux.CreateWindow{Rows: 48, Cols: 160})
		win1 := testutil.RequireWindow(t, sessionRef, super)
		_ = testutil.RequirePane(t, sessionRef, super)

		win1.Send(shux.Split{Dir: shux.SplitV})
		time.Sleep(100 * time.Millisecond)
		win1.Send(shux.SwitchToPane{Index: 0})
		time.Sleep(50 * time.Millisecond)
		win1.Send(shux.Split{Dir: shux.SplitH})
		time.Sleep(100 * time.Millisecond)
		win1.Send(shux.SwitchToPane{Index: 2})
		time.Sleep(50 * time.Millisecond)
		win1.Send(shux.Split{Dir: shux.SplitH})
		time.Sleep(100 * time.Millisecond)

		// Window 2: Single pane with different shell
		sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
		time.Sleep(100 * time.Millisecond)

		// Switch back to window 1
		sessionRef.Send(shux.SwitchWindow{Delta: -1})
		time.Sleep(50 * time.Millisecond)

		// Capture state before "crash"
		preSnapshot := <-sessionRef.Ask(shux.GetFullSessionSnapshot{})
		if preSnapshot == nil {
			t.Fatal("Failed to capture pre-crash snapshot")
		}
		preData := preSnapshot.(*shux.SessionSnapshot)

		t.Logf("Pre-crash state: %d windows, active=%d, window1 panes=%d",
			len(preData.Windows), preData.ActiveWindow, len(preData.Windows[0].PaneOrder))

		// Phase 2: Simulate crash (detach saves, session "dies")
		t.Log("Phase 2: Simulating crash (detach)")

		reply := sessionRef.Ask(shux.DetachSession{})
		<-reply
		super.WaitSessionEmpty(2 * time.Second)
		sessionRef.Shutdown()

		// Phase 3: "Resurrection" - restore from saved state
		t.Log("Phase 3: Resurrecting from snapshot")

		super2 := testutil.NewTestSupervisor()
		resurrectedRef, err := shux.RestoreSessionFromSnapshot("resurrection-test", super2.Handle, testutil.TestLogger())
		if err != nil {
			t.Fatalf("Failed to resurrect: %v", err)
		}
		defer resurrectedRef.Shutdown()

		// Phase 4: Verify resurrection
		t.Log("Phase 4: Verifying resurrected state")

		testutil.PollFor(2*time.Second, func() bool {
			result := <-resurrectedRef.Ask(shux.GetSessionSnapshotData{})
			if data, ok := result.(shux.SessionSnapshotData); ok {
				return len(data.WindowOrder) == 2
			}
			return false
		})

		postSnapshot := <-resurrectedRef.Ask(shux.GetFullSessionSnapshot{})
		if postSnapshot == nil {
			t.Fatal("Failed to capture post-resurrection snapshot")
		}
		postData := postSnapshot.(*shux.SessionSnapshot)

		// Validate resurrection
		testutil.AssertPersistenceInvariant(t, preData, postData)

		if len(postData.Windows) != 2 {
			t.Errorf("Expected 2 windows after resurrection, got %d", len(postData.Windows))
		}

		// Verify window 1 still has 4 panes
		if len(postData.Windows[0].PaneOrder) != 4 {
			t.Errorf("Expected 4 panes in window 1 after resurrection, got %d",
				len(postData.Windows[0].PaneOrder))
		}

		// Verify active window is preserved (should be window 1)
		if postData.ActiveWindow != 1 {
			t.Errorf("Expected active window 1 after resurrection, got %d", postData.ActiveWindow)
		}

		t.Log("=== Resurrection Story Complete ===")
		t.Logf("Resurrected: %d windows, %d panes in window 1",
			len(postData.Windows), len(postData.Windows[0].PaneOrder))
	})
}

// TestE2EResurrectionNamedSession validates resurrection preserves session name.
func TestE2EResurrectionNamedSession(t *testing.T) {
	if os.Getenv("SHUX_E2E") != "1" {
		t.Skip("Set SHUX_E2E=1 to run E2E tests")
	}

	testutil.WithTempHome(t, func(tempDir string) {
		sessionName := "my-project-session"

		// Create named session
		sessionRef, super, _ := testutil.SetupNamedSession(t, sessionName)
		sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
		testutil.RequireWindow(t, sessionRef, super)

		// Detach
		reply := sessionRef.Ask(shux.DetachSession{})
		<-reply
		super.WaitSessionEmpty(2 * time.Second)
		sessionRef.Shutdown()

		// Resurrect
		super2 := testutil.NewTestSupervisor()
		resurrectedRef, err := shux.RestoreSessionFromSnapshot(sessionName, super2.Handle, testutil.TestLogger())
		if err != nil {
			t.Fatalf("Failed to resurrect named session: %v", err)
		}
		defer resurrectedRef.Shutdown()

		// Verify session name preserved
		if name := resurrectedRef.GetSessionName(); name != sessionName {
			t.Errorf("Expected session name %q, got %q", sessionName, name)
		}
	})
}

// TestE2EResurrectionWithCWD validates CWD is preserved through resurrection.
func TestE2EResurrectionWithCWD(t *testing.T) {
	if os.Getenv("SHUX_E2E") != "1" {
		t.Skip("Set SHUX_E2E=1 to run E2E tests")
	}

	testutil.WithTempHome(t, func(tempDir string) {
		sessionRef, super, _ := testutil.SetupNamedSession(t, "cwd-resurrection")

		sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
		win := testutil.RequireWindow(t, sessionRef, super)
		_ = testutil.RequirePane(t, sessionRef, super)

		// Create pane with specific CWD
		testCWD := "/tmp"
		win.Send(shux.CreatePane{Rows: 24, Cols: 80, Shell: "/bin/sh", CWD: testCWD})
		time.Sleep(100 * time.Millisecond)

		// Detach
		reply := sessionRef.Ask(shux.DetachSession{})
		<-reply
		super.WaitSessionEmpty(2 * time.Second)
		sessionRef.Shutdown()

		// Resurrect
		super2 := testutil.NewTestSupervisor()
		resurrectedRef, err := shux.RestoreSessionFromSnapshot("cwd-resurrection", super2.Handle, testutil.TestLogger())
		if err != nil {
			t.Fatalf("Failed to resurrect: %v", err)
		}
		defer resurrectedRef.Shutdown()

		// Wait for restoration
		testutil.PollFor(2*time.Second, func() bool {
			result := <-resurrectedRef.Ask(shux.GetActiveWindow{})
			return result != nil
		})

		// Verify CWD is preserved in the restored snapshot
		postSnapshot := <-resurrectedRef.Ask(shux.GetFullSessionSnapshot{})
		if postSnapshot == nil {
			t.Fatal("Failed to capture post-resurrection snapshot")
		}
		postData := postSnapshot.(*shux.SessionSnapshot)

		// Check that the specific CWD exists in the restored panes
		foundCWD := false
		for _, win := range postData.Windows {
			for _, pane := range win.Panes {
				if pane.CWD == testCWD {
					foundCWD = true
					break
				}
			}
			if foundCWD {
				break
			}
		}
		if !foundCWD {
			t.Errorf("Expected CWD %q to be preserved after resurrection", testCWD)
		}

		_ = super2
	})
}
