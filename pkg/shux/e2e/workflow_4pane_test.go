//go:build e2e
// +build e2e

package e2e

import (
	"os"
	"strings"
	"testing"
	"time"

	"shux/pkg/shux"
	"shux/pkg/testutil"
)

// TestE2EWorkflow4Pane validates the 4-pane workflow from AGENTS.md:
// "As a user I have shux running 4 panes. Top left: less with documentation.
// Top right: long running Node process. Bottom left: Nano editor.
// Bottom right: plain terminal for shell commands."
func TestE2EWorkflow4Pane(t *testing.T) {
	if os.Getenv("SHUX_E2E") != "1" {
		t.Skip("Set SHUX_E2E=1 to run E2E tests")
	}

	testutil.WithTempHome(t, func(tempDir string) {
		t.Log("=== 4-Pane Workflow Test ===")

		// Setup session
		sessionRef, super, _ := testutil.SetupNamedSession(t, "4pane-workflow")

		// === Phase 1: Create 2x2 layout ===
		t.Log("Phase 1: Creating 2x2 layout")

		runner := testutil.NewScenarioRunner(sessionRef, super)
		for _, step := range testutil.FourPaneWorkflowScenario() {
			runner.AddStep(step)
		}
		runner.Run(t)

		winAny := <-sessionRef.Ask(shux.GetActiveWindow{})
		win := winAny.(*shux.WindowRef)
		winData := <-win.Ask(shux.GetWindowSnapshotData{})
		data := winData.(shux.WindowSnapshot)
		if len(data.PaneOrder) != 4 {
			t.Fatalf("Expected 4 panes, got %d", len(data.PaneOrder))
		}

		t.Logf("Created 4-pane layout: %v", data.PaneOrder)

		// === Phase 2: Run applications in each pane ===
		t.Log("Phase 2: Starting applications in each pane")

		// Top-left (pane 1): Run 'echo' to simulate content
		win.Send(shux.SwitchToPane{Index: 0})
		time.Sleep(50 * time.Millisecond)
		pane1 := <-sessionRef.Ask(shux.GetActivePane{})
		if pane1 != nil {
			pane1.(*shux.PaneRef).Send(shux.WriteToPane{Data: []byte("echo 'Documentation Pane'\r")})
		}

		// Top-right (pane 2): Run simple process
		win.Send(shux.SwitchToPane{Index: 1})
		time.Sleep(50 * time.Millisecond)
		pane2 := <-sessionRef.Ask(shux.GetActivePane{})
		if pane2 != nil {
			pane2.(*shux.PaneRef).Send(shux.WriteToPane{Data: []byte("echo 'Node Process Pane'\r")})
		}

		// Bottom-left (pane 3): Editor pane
		win.Send(shux.SwitchToPane{Index: 2})
		time.Sleep(50 * time.Millisecond)
		pane3 := <-sessionRef.Ask(shux.GetActivePane{})
		if pane3 != nil {
			pane3.(*shux.PaneRef).Send(shux.WriteToPane{Data: []byte("echo 'Editor Pane'\r")})
		}

		// Bottom-right (pane 4): Shell pane
		win.Send(shux.SwitchToPane{Index: 3})
		time.Sleep(50 * time.Millisecond)
		pane4 := <-sessionRef.Ask(shux.GetActivePane{})
		if pane4 != nil {
			pane4.(*shux.PaneRef).Send(shux.WriteToPane{Data: []byte("echo 'Shell Pane'\r")})
		}

		time.Sleep(200 * time.Millisecond) // Let content render

		// === Phase 3: Verify view ===
		t.Log("Phase 3: Verifying window view")

		viewData := <-sessionRef.Ask(shux.GetWindowView{})
		view := viewData.(shux.WindowView)

		// Verify render shows dividers
		if !strings.Contains(view.Content, "│") && !strings.Contains(view.Content, "─") {
			t.Error("Expected dividers in 4-pane view")
		}

		t.Logf("Window view rendered with dividers")

		// === Phase 4: Detach ===
		t.Log("Phase 4: Detaching session")

		reply := sessionRef.Ask(shux.DetachSession{})
		<-reply
		super.WaitSessionEmpty(3 * time.Second)
		sessionRef.Shutdown()

		// === Phase 5: Verify snapshot ===
		t.Log("Phase 5: Verifying snapshot")

		if !shux.SessionSnapshotExists("4pane-workflow") {
			t.Fatal("Snapshot should exist after detach")
		}

		snapshot, err := shux.LoadSnapshot(shux.SessionSnapshotPath("4pane-workflow"), testutil.TestLogger())
		if err != nil {
			t.Fatalf("Failed to load snapshot: %v", err)
		}

		if len(snapshot.Windows) != 1 {
			t.Fatalf("Expected 1 window in snapshot, got %d", len(snapshot.Windows))
		}

		if len(snapshot.Windows[0].PaneOrder) != 4 {
			t.Errorf("Expected 4 panes in snapshot, got %d", len(snapshot.Windows[0].PaneOrder))
		}

		t.Logf("Snapshot verified: %d windows, %d panes",
			len(snapshot.Windows), len(snapshot.Windows[0].PaneOrder))

		// === Phase 6: Reattach and verify ===
		t.Log("Phase 6: Reattaching and verifying restoration")

		super2 := testutil.NewTestSupervisor()
		restoredRef, err := shux.RestoreSessionFromSnapshot("4pane-workflow", super2.Handle, testutil.TestLogger())
		if err != nil {
			t.Fatalf("Failed to restore: %v", err)
		}
		defer restoredRef.Shutdown()

		testutil.PollFor(2*time.Second, func() bool {
			result := <-restoredRef.Ask(shux.GetSessionSnapshotData{})
			if data, ok := result.(shux.SessionSnapshotData); ok {
				return len(data.WindowOrder) == 1
			}
			return false
		})

		restoredWin := <-restoredRef.Ask(shux.GetActiveWindow{})
		restoredData := <-restoredWin.(*shux.WindowRef).Ask(shux.GetWindowSnapshotData{})
		winInfo := restoredData.(shux.WindowSnapshot)

		if len(winInfo.PaneOrder) != 4 {
			t.Errorf("Expected 4 panes after restore, got %d", len(winInfo.PaneOrder))
		}

		t.Log("=== 4-Pane Workflow Test Complete ===")
		t.Logf("Successfully restored 4-pane layout with %d panes", len(winInfo.PaneOrder))
	})
}

// TestE2EWorkflowMultipleWindows validates multi-window workflow.
func TestE2EWorkflowMultipleWindows(t *testing.T) {
	if os.Getenv("SHUX_E2E") != "1" {
		t.Skip("Set SHUX_E2E=1 to run E2E tests")
	}

	testutil.WithTempHome(t, func(tempDir string) {
		sessionRef, super, _ := testutil.SetupNamedSession(t, "multi-window-workflow")

		// Window 1: Development
		sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
		win1 := testutil.RequireWindow(t, sessionRef, super)
		win1.Send(shux.Split{Dir: shux.SplitV})
		testutil.WaitWindowPaneCount(t, win1, 2, time.Second)

		// Window 2: Monitoring
		sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
		testutil.WaitSessionWindowCount(t, sessionRef, 2, time.Second)

		// Window 3: Documentation
		sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
		testutil.WaitSessionWindowCount(t, sessionRef, 3, time.Second)

		// Verify 3 windows
		result := <-sessionRef.Ask(shux.GetSessionSnapshotData{})
		data := result.(shux.SessionSnapshotData)
		if len(data.WindowOrder) != 3 {
			t.Fatalf("Expected 3 windows, got %d", len(data.WindowOrder))
		}

		// Switch between windows
		for i := 0; i < 5; i++ {
			sessionRef.Send(shux.SwitchWindow{Delta: 1})
			time.Sleep(50 * time.Millisecond)
		}

		// Detach and restore
		reply := sessionRef.Ask(shux.DetachSession{})
		<-reply
		super.WaitSessionEmpty(2 * time.Second)
		sessionRef.Shutdown()

		super2 := testutil.NewTestSupervisor()
		restoredRef, err := shux.RestoreSessionFromSnapshot("multi-window-workflow", super2.Handle, testutil.TestLogger())
		if err != nil {
			t.Fatalf("Failed to restore: %v", err)
		}
		defer restoredRef.Shutdown()

		testutil.PollFor(2*time.Second, func() bool {
			result := <-restoredRef.Ask(shux.GetSessionSnapshotData{})
			if data, ok := result.(shux.SessionSnapshotData); ok {
				return len(data.WindowOrder) == 3
			}
			return false
		})

		finalData := <-restoredRef.Ask(shux.GetSessionSnapshotData{})
		final := finalData.(shux.SessionSnapshotData)
		if len(final.WindowOrder) != 3 {
			t.Errorf("Expected 3 windows after restore, got %d", len(final.WindowOrder))
		}
	})
}

// TestE2EWorkflowKillShuxResurrect validates the full kill and resurrect workflow.
func TestE2EWorkflowKillShuxResurrect(t *testing.T) {
	if os.Getenv("SHUX_E2E") != "1" {
		t.Skip("Set SHUX_E2E=1 to run E2E tests")
	}

	testutil.WithTempHome(t, func(tempDir string) {
		t.Log("=== Kill Shux and Resurrect Workflow ===")

		// === Phase 1: Create complex session ===
		t.Log("Phase 1: Creating complex session")

		sessionRef, super, _ := testutil.SetupNamedSession(t, "kill-resurrect-workflow")

		// Multiple windows
		sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
		win1 := testutil.RequireWindow(t, sessionRef, super)
		win1.Send(shux.Split{Dir: shux.SplitV})
		testutil.WaitWindowPaneCount(t, win1, 2, time.Second)

		sessionRef.Send(shux.CreateWindow{Rows: 30, Cols: 100})
		testutil.WaitSessionWindowCount(t, sessionRef, 2, time.Second)

		// Capture state
		preSnapshot := <-sessionRef.Ask(shux.GetFullSessionSnapshot{})
		if preSnapshot == nil {
			t.Fatal("Failed to capture pre-kill snapshot")
		}
		preData := preSnapshot.(*shux.SessionSnapshot)

		t.Logf("Pre-kill: %d windows", len(preData.Windows))

		// === Phase 2: Kill (detach and shutdown) ===
		t.Log("Phase 2: Killing shux (detach and shutdown)")

		reply := sessionRef.Ask(shux.DetachSession{})
		<-reply
		super.WaitSessionEmpty(2 * time.Second)
		sessionRef.Shutdown()

		time.Sleep(100 * time.Millisecond)

		// === Phase 3: Resurrect ===
		t.Log("Phase 3: Resurrecting session")

		super2 := testutil.NewTestSupervisor()
		restoredRef, err := shux.RestoreSessionFromSnapshot("kill-resurrect-workflow", super2.Handle, testutil.TestLogger())
		if err != nil {
			t.Fatalf("Failed to resurrect: %v", err)
		}
		defer restoredRef.Shutdown()

		// === Phase 4: Verify ===
		t.Log("Phase 4: Verifying resurrection")

		testutil.PollFor(2*time.Second, func() bool {
			result := <-restoredRef.Ask(shux.GetSessionSnapshotData{})
			if data, ok := result.(shux.SessionSnapshotData); ok {
				return len(data.WindowOrder) == 2
			}
			return false
		})

		postSnapshot := <-restoredRef.Ask(shux.GetFullSessionSnapshot{})
		if postSnapshot == nil {
			t.Fatal("Failed to capture post-resurrection snapshot")
		}
		postData := postSnapshot.(*shux.SessionSnapshot)

		testutil.AssertPersistenceInvariant(t, preData, postData)

		if len(postData.Windows) != 2 {
			t.Errorf("Expected 2 windows after resurrection, got %d", len(postData.Windows))
		}

		t.Log("=== Kill and Resurrect Workflow Complete ===")
	})
}
