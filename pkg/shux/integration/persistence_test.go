package integration

import (
	"os"
	"testing"
	"time"

	"shux/pkg/shux"
	"shux/pkg/testutil"
)

// TestPersistenceSaveLoad validates snapshot save and load cycle.
func TestPersistenceSaveLoad(t *testing.T) {
	testutil.WithTempHome(t, func(tempDir string) {
		// Create a snapshot manually
		snapshot := testutil.BuildTestSnapshot("persist-test", 2)

		// Save
		path := shux.SessionSnapshotPath("persist-test")
		if err := shux.EnsureSessionDir("persist-test"); err != nil {
			t.Fatalf("ensure session dir: %v", err)
		}
		if err := shux.SaveSnapshot(path, snapshot, testutil.TestLogger()); err != nil {
			t.Fatalf("save snapshot: %v", err)
		}

		// Verify file exists
		if !shux.SessionSnapshotExists("persist-test") {
			t.Fatal("snapshot should exist after save")
		}

		// Load
		loaded, err := shux.LoadSnapshot(path, testutil.TestLogger())
		if err != nil {
			t.Fatalf("load snapshot: %v", err)
		}

		// Validate persistence invariant
		testutil.AssertPersistenceInvariant(t, snapshot, loaded)
		testutil.AssertSnapshotInvariant(t, loaded)
	})
}

// TestPersistenceSnapshotExists validates snapshot existence check.
func TestPersistenceSnapshotExists(t *testing.T) {
	testutil.WithTempHome(t, func(tempDir string) {
		// Non-existent session
		if shux.SessionSnapshotExists("nonexistent") {
			t.Error("non-existent session should not exist")
		}

		// Create snapshot
		snapshot := &shux.SessionSnapshot{
			Version:     shux.SnapshotVersion,
			SessionName: "exists-test",
			ID:          1,
		}

		if err := shux.EnsureSessionDir("exists-test"); err != nil {
			t.Fatalf("ensure session dir: %v", err)
		}
		if err := shux.SaveSnapshot(shux.SessionSnapshotPath("exists-test"), snapshot, testutil.TestLogger()); err != nil {
			t.Fatalf("save snapshot: %v", err)
		}

		if !shux.SessionSnapshotExists("exists-test") {
			t.Error("existing session should exist")
		}
	})
}

// TestPersistenceSessionDirCreation validates session directory creation.
func TestPersistenceSessionDirCreation(t *testing.T) {
	testutil.WithTempHome(t, func(tempDir string) {
		if err := shux.EnsureSessionDir("dir-test"); err != nil {
			t.Fatalf("ensure session dir: %v", err)
		}

		// Directory should exist
		dir := shux.SessionDir("dir-test")
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("session dir should exist: %v", err)
		}
	})
}

// TestPersistenceDetachCycle validates full detach/restore cycle.
func TestPersistenceDetachCycle(t *testing.T) {
	testutil.WithTempHome(t, func(tempDir string) {
		sessionRef, super, cleanup := testutil.SetupNamedSession(t, "detach-cycle")
		defer cleanup()

		// Create some state
		sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
		win := testutil.RequireWindow(t, sessionRef, super)
		_ = testutil.RequirePane(t, sessionRef, super)

		// Add more panes
		win.Send(shux.Split{Dir: shux.SplitV})
		testutil.WaitWindowPaneCount(t, win, 2, 200*time.Millisecond)

		// Get pre-detach snapshot
		preDetach := <-sessionRef.Ask(shux.GetFullSessionSnapshot{})
		if preDetach == nil {
			t.Fatal("failed to get pre-detach snapshot")
		}
		preSnapshot := preDetach.(*shux.SessionSnapshot)

		// Detach
		reply := sessionRef.Ask(shux.DetachSession{})
		<-reply

		if !super.WaitSessionEmpty(2 * time.Second) {
			t.Fatal("timeout waiting for session empty")
		}
		sessionRef.Shutdown()

		// Load saved snapshot
		loaded, err := shux.LoadSnapshot(shux.SessionSnapshotPath("detach-cycle"), testutil.TestLogger())
		if err != nil {
			t.Fatalf("load snapshot: %v", err)
		}

		// Validate persistence invariant
		testutil.AssertPersistenceInvariant(t, preSnapshot, loaded)
	})
}

// TestPersistenceNestedSplitRestore validates complex layout survives persist.
func TestPersistenceNestedSplitRestore(t *testing.T) {
	testutil.WithTempHome(t, func(tempDir string) {
		sessionRef, super, cleanup := testutil.SetupNamedSession(t, "nested-split")
		defer cleanup()

		runner := testutil.NewScenarioRunner(sessionRef, super)
		for _, step := range testutil.FourPaneWorkflowScenario() {
			runner.AddStep(step)
		}
		runner.Run(t)

		winAny := <-sessionRef.Ask(shux.GetActiveWindow{})
		win := winAny.(*shux.WindowRef)

		// Verify 4 panes
		result := <-win.Ask(shux.GetWindowSnapshotData{})
		preData := result.(shux.WindowSnapshot)
		if len(preData.PaneOrder) != 4 {
			t.Fatalf("expected 4 panes, got %d", len(preData.PaneOrder))
		}

		// Detach and restore
		reply := sessionRef.Ask(shux.DetachSession{})
		<-reply
		super.WaitSessionEmpty(2 * time.Second)
		sessionRef.Shutdown()

		// Restore
		super2 := testutil.NewTestSupervisor()
		restoredRef, err := shux.RestoreSessionFromSnapshot("nested-split", super2.Handle, testutil.TestLogger())
		if err != nil {
			t.Fatalf("restore session: %v", err)
		}
		defer restoredRef.Shutdown()

		// Verify restored state
		testutil.PollFor(time.Second, func() bool {
			result := <-restoredRef.Ask(shux.GetActiveWindow{})
			return result != nil
		})

		restoredWin := <-restoredRef.Ask(shux.GetActiveWindow{})
		postData := <-restoredWin.(*shux.WindowRef).Ask(shux.GetWindowSnapshotData{})
		postWin := postData.(shux.WindowSnapshot)

		if len(postWin.PaneOrder) != 4 {
			t.Errorf("expected 4 panes after restore, got %d", len(postWin.PaneOrder))
		}

		// Verify layout preserved
		if postWin.Layout == nil {
			t.Error("expected layout after restore")
		}
	})
}

// TestPersistenceWindowOrder validates window order is preserved.
func TestPersistenceWindowOrder(t *testing.T) {
	testutil.WithTempHome(t, func(tempDir string) {
		sessionRef, super, cleanup := testutil.SetupNamedSession(t, "window-order")
		defer cleanup()

		// Create 3 windows
		for i := 0; i < 3; i++ {
			sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
			testutil.RequireWindow(t, sessionRef, super)
		}

		// Switch to window 1
		sessionRef.Send(shux.SwitchWindow{Delta: -2})
		testutil.PollFor(100*time.Millisecond, func() bool {
			result := <-sessionRef.Ask(shux.GetSessionSnapshotData{})
			if data, ok := result.(shux.SessionSnapshotData); ok {
				return data.ActiveWindow == 1
			}
			return false
		})

		// Detach
		reply := sessionRef.Ask(shux.DetachSession{})
		<-reply
		super.WaitSessionEmpty(2 * time.Second)
		sessionRef.Shutdown()

		// Load and verify
		loaded, err := shux.LoadSnapshot(shux.SessionSnapshotPath("window-order"), testutil.TestLogger())
		if err != nil {
			t.Fatalf("load snapshot: %v", err)
		}

		if len(loaded.WindowOrder) != 3 {
			t.Errorf("expected 3 windows, got %d", len(loaded.WindowOrder))
		}

		// Window order should be preserved
		for i, wid := range loaded.WindowOrder {
			if wid != uint32(i+1) {
				t.Errorf("window order[%d] = %d, expected %d", i, wid, i+1)
			}
		}
	})
}

// TestPersistenceActiveWindowSelection validates active window is preserved.
func TestPersistenceActiveWindowSelection(t *testing.T) {
	testutil.WithTempHome(t, func(tempDir string) {
		sessionRef, super, cleanup := testutil.SetupNamedSession(t, "active-window")
		defer cleanup()

		// Create 2 windows
		sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
		testutil.RequireWindow(t, sessionRef, super)

		sessionRef.Send(shux.CreateWindow{Rows: 30, Cols: 100})
		testutil.RequireWindow(t, sessionRef, super)

		// Get pre-detach active window
		result := <-sessionRef.Ask(shux.GetFullSessionSnapshot{})
		preSnapshot := result.(*shux.SessionSnapshot)
		preActiveWindow := preSnapshot.ActiveWindow
		if preActiveWindow == 0 {
			t.Fatal("expected an active window before detach")
		}

		// Detach
		detachReply := sessionRef.Ask(shux.DetachSession{})
		<-detachReply
		super.WaitSessionEmpty(2 * time.Second)
		sessionRef.Shutdown()

		// Load and verify
		loaded, err := shux.LoadSnapshot(shux.SessionSnapshotPath("active-window"), testutil.TestLogger())
		if err != nil {
			t.Fatalf("load snapshot: %v", err)
		}

		// Active window should be preserved from pre-detach
		if loaded.ActiveWindow != preActiveWindow {
			t.Errorf("expected active window %d after restore, got %d", preActiveWindow, loaded.ActiveWindow)
		}
	})
}

// TestPersistencePaneCWD validates pane CWD is preserved.
func TestPersistencePaneCWD(t *testing.T) {
	testutil.WithTempHome(t, func(tempDir string) {
		sessionRef, super, cleanup := testutil.SetupNamedSession(t, "pane-cwd")
		defer cleanup()

		sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
		win := testutil.RequireWindow(t, sessionRef, super)
		_ = testutil.RequirePane(t, sessionRef, super)

		// Create pane with specific CWD
		specificCWD := "/tmp"
		win.Send(shux.CreatePane{Rows: 24, Cols: 80, Shell: "/bin/sh", CWD: specificCWD})
		testutil.WaitWindowPaneCount(t, win, 2, 200*time.Millisecond)

		// Detach
		reply := sessionRef.Ask(shux.DetachSession{})
		<-reply
		super.WaitSessionEmpty(2 * time.Second)
		sessionRef.Shutdown()

		// Load and verify CWD
		loaded, err := shux.LoadSnapshot(shux.SessionSnapshotPath("pane-cwd"), testutil.TestLogger())
		if err != nil {
			t.Fatalf("load snapshot: %v", err)
		}

		if len(loaded.Windows) == 0 || len(loaded.Windows[0].Panes) < 2 {
			t.Fatal("expected at least 2 panes in snapshot")
		}

		found := false
		for _, pane := range loaded.Windows[0].Panes {
			if pane.CWD == specificCWD {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected pane with CWD %q in snapshot", specificCWD)
		}
	})
}

// TestPersistenceDetachFailureLeavesSessionAlive validates snapshot-write failure
// is surfaced without tearing down the live session.
func TestPersistenceDetachFailureLeavesSessionAlive(t *testing.T) {
	homeFile, err := os.CreateTemp("", "shux-home-file")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	_ = homeFile.Close()
	defer func() { _ = os.Remove(homeFile.Name()) }()
	oldHome := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", oldHome) }()
	if err := os.Setenv("HOME", homeFile.Name()); err != nil {
		t.Fatalf("set HOME: %v", err)
	}

	sessionRef, super, cleanup := testutil.SetupSession(t)
	defer cleanup()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	_ = testutil.RequireWindow(t, sessionRef, super)

	result := <-sessionRef.Ask(shux.ActionMsg{Action: shux.ActionDetach})
	action, ok := result.(shux.ActionResult)
	if !ok {
		t.Fatalf("expected ActionResult, got %T", result)
	}
	if action.Err == nil {
		t.Fatal("expected detach to fail when HOME points to a file")
	}
	if action.Quit {
		t.Fatal("detach failure should not request quit")
	}

	if got := <-sessionRef.Ask(shux.GetActiveWindow{}); got == nil {
		t.Fatal("session should remain alive after failed detach")
	}
}
