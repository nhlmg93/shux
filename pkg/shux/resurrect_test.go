package shux

import (
	"os"
	"testing"
	"time"
)

// testLogger returns a logger for tests. Returns NoOpLogger if global Logger is not initialized.
func testLogger() ShuxLogger {
	if Logger == nil {
		return NoOpLogger{}
	}
	return &StdLogger{Logger}
}

func TestStartSessionFromSnapshot(t *testing.T) {
	super := newTestSupervisor()
	logger := testLogger()

	snapshot := &SessionSnapshot{
		Version:      SnapshotVersion,
		SessionName:  "restored",
		ID:           1,
		Shell:        "/bin/sh",
		ActiveWindow: 1,
		WindowOrder:  []uint32{1},
		Windows: []WindowSnapshot{{
			ID:         1,
			ActivePane: 1,
			PaneOrder:  []uint32{1},
			Panes: []PaneSnapshot{{
				ID: 1, Shell: "/bin/sh", Rows: 24, Cols: 80, CWD: "/tmp",
			}},
		}},
	}

	sessionRef := StartSessionFromSnapshot(snapshot, super.handle, logger)
	if sessionRef == nil {
		t.Fatal("StartSessionFromSnapshot returned nil")
	}
	defer sessionRef.Shutdown()

	time.Sleep(100 * time.Millisecond)

	result := <-sessionRef.Ask(GetSessionSnapshotData{})
	data, ok := result.(SessionSnapshotData)
	if !ok {
		t.Fatalf("Expected SessionSnapshotData, got %T", result)
	}
	if data.ID != 1 {
		t.Errorf("Session ID: got %d, want 1", data.ID)
	}
	if data.Shell != "/bin/sh" {
		t.Errorf("Session Shell: got %s, want /bin/sh", data.Shell)
	}
	if len(data.WindowOrder) != 1 {
		t.Errorf("Window count: got %d, want 1", len(data.WindowOrder))
	}
}

func TestSessionSnapshotDataRoundTrip(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	win := requireWindow(t, sessionRef, super)

	result := <-sessionRef.Ask(GetSessionSnapshotData{})
	data, ok := result.(SessionSnapshotData)
	if !ok {
		t.Fatalf("Expected SessionSnapshotData, got %T", result)
	}
	if data.ID != 1 {
		t.Errorf("ID: got %d, want 1", data.ID)
	}
	if len(data.WindowOrder) != 1 {
		t.Errorf("WindowOrder length: got %d, want 1", len(data.WindowOrder))
	}

	winResult := <-win.Ask(GetWindowSnapshotData{})
	winData, ok := winResult.(WindowSnapshot)
	if !ok {
		t.Fatalf("Expected WindowSnapshot, got %T", winResult)
	}
	if winData.ID == 0 {
		t.Error("Window ID should not be 0")
	}
	if len(winData.PaneOrder) != 1 {
		t.Errorf("PaneOrder length: got %d, want 1", len(winData.PaneOrder))
	}
}

func TestBuildSnapshotEmptySession(t *testing.T) {
	sessionRef, _, cleanup := setupSession(t)
	defer cleanup()

	result := <-sessionRef.Ask(GetSessionSnapshotData{})
	data, ok := result.(SessionSnapshotData)
	if !ok {
		t.Fatalf("Expected SessionSnapshotData, got %T", result)
	}
	if len(data.WindowOrder) != 0 {
		t.Errorf("Empty session should have 0 windows, got %d", len(data.WindowOrder))
	}
}

func TestRestoreSessionFromSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := setTestHome(t, tmpDir)
	defer restoreHome(t, oldHome)

	super := newTestSupervisor()

	snapshot := &SessionSnapshot{
		Version:      SnapshotVersion,
		SessionName:  "test-session",
		ID:           42,
		Shell:        "/bin/sh",
		ActiveWindow: 1,
		WindowOrder:  []uint32{1, 2},
		Windows: []WindowSnapshot{
			{ID: 1, ActivePane: 1, PaneOrder: []uint32{1}, Panes: []PaneSnapshot{{ID: 1, Shell: "/bin/sh", Rows: 24, Cols: 80}}},
			{ID: 2, ActivePane: 2, PaneOrder: []uint32{2}, Panes: []PaneSnapshot{{ID: 2, Shell: "/bin/sh", Rows: 30, Cols: 100}}},
		},
	}

	if err := EnsureSessionDir("test-session"); err != nil {
		t.Fatalf("EnsureSessionDir failed: %v", err)
	}
	logger := testLogger()
	if err := SaveSnapshot(SessionSnapshotPath("test-session"), snapshot, logger); err != nil {
		t.Fatalf("SaveSnapshot failed: %v", err)
	}

	sessionRef, err := RestoreSessionFromSnapshot("test-session", super.handle, logger)
	if err != nil {
		t.Fatalf("RestoreSessionFromSnapshot failed: %v", err)
	}
	if sessionRef == nil {
		t.Fatal("RestoreSessionFromSnapshot returned nil ref")
	}
	defer sessionRef.Shutdown()

	time.Sleep(200 * time.Millisecond)

	result := <-sessionRef.Ask(GetSessionSnapshotData{})
	data, ok := result.(SessionSnapshotData)
	if !ok {
		t.Fatalf("Expected SessionSnapshotData, got %T", result)
	}
	if data.ID != 42 {
		t.Errorf("Session ID: got %d, want 42", data.ID)
	}
	if len(data.WindowOrder) != 2 {
		t.Errorf("Window count: got %d, want 2", len(data.WindowOrder))
	}
}

func TestRestoreSessionFromSnapshotNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := setTestHome(t, tmpDir)
	defer restoreHome(t, oldHome)

	super := newTestSupervisor()
	logger := testLogger()
	if _, err := RestoreSessionFromSnapshot("nonexistent", super.handle, logger); err == nil {
		t.Error("Expected error for non-existent snapshot")
	}
}

func setTestHome(t *testing.T, dir string) string {
	t.Helper()
	old := os.Getenv("HOME")
	if err := os.Setenv("HOME", dir); err != nil {
		t.Fatalf("set HOME: %v", err)
	}
	return old
}

func restoreHome(t *testing.T, old string) {
	t.Helper()
	if err := os.Setenv("HOME", old); err != nil {
		t.Fatalf("restore HOME: %v", err)
	}
}

func TestSessionDetachSavesNamedSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := setTestHome(t, tmpDir)
	defer restoreHome(t, oldHome)

	super := newTestSupervisor()
	logger := testLogger()
	sessionRef := StartNamedSessionWithShell(1, "work", "/bin/sh", super.handle, logger)
	defer sessionRef.Shutdown()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	requirePane(t, sessionRef, super)

	if err, _ := (<-sessionRef.Ask(DetachSession{})).(error); err != nil {
		t.Fatalf("detach failed: %v", err)
	}

	path := SessionSnapshotPath("work")
	if !pollFor(500*time.Millisecond, func() bool {
		_, err := os.Stat(path)
		return err == nil
	}) {
		t.Fatalf("expected snapshot at %s", path)
	}

	snapshot, err := LoadSnapshot(path, testLogger())
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if snapshot.SessionName != "work" {
		t.Fatalf("expected snapshot session name %q, got %q", "work", snapshot.SessionName)
	}
}

func TestRestoreSessionPreservesSelectionsAndCWD(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := setTestHome(t, tmpDir)
	defer restoreHome(t, oldHome)

	cwd := t.TempDir()
	super := newTestSupervisor()

	snapshot := &SessionSnapshot{
		Version:      SnapshotVersion,
		SessionName:  "project",
		ID:           42,
		Shell:        "/bin/sh",
		ActiveWindow: 2,
		WindowOrder:  []uint32{2, 5},
		Windows: []WindowSnapshot{
			{
				ID:         2,
				ActivePane: 7,
				PaneOrder:  []uint32{3, 7},
				Panes: []PaneSnapshot{
					{ID: 3, Shell: "/bin/sh", Rows: 24, Cols: 80, CWD: cwd},
					{ID: 7, Shell: "/bin/sh", Rows: 24, Cols: 80, CWD: cwd},
				},
				Layout: &SplitTreeSnapshot{
					Dir:    SplitV,
					Ratio:  0.5,
					First:  &SplitTreeSnapshot{PaneID: 3},
					Second: &SplitTreeSnapshot{PaneID: 7},
				},
			},
			{
				ID:         5,
				ActivePane: 8,
				PaneOrder:  []uint32{8},
				Panes:      []PaneSnapshot{{ID: 8, Shell: "/bin/sh", Rows: 30, Cols: 100, CWD: cwd}},
				Layout:     &SplitTreeSnapshot{PaneID: 8},
			},
		},
	}
	logger := testLogger()
	if err := SaveSnapshot(SessionSnapshotPath("project"), snapshot, logger); err != nil {
		t.Fatalf("SaveSnapshot failed: %v", err)
	}

	sessionRef, err := RestoreSessionFromSnapshot("project", super.handle, logger)
	if err != nil {
		t.Fatalf("RestoreSessionFromSnapshot failed: %v", err)
	}
	defer sessionRef.Shutdown()

	if !pollFor(2*time.Second, func() bool {
		result := <-sessionRef.Ask(GetSessionSnapshotData{})
		data, ok := result.(SessionSnapshotData)
		if !ok {
			return false
		}
		return data.ActiveWindow == 2 && len(data.WindowOrder) == 2 && data.WindowOrder[0] == 2 && data.WindowOrder[1] == 5
	}) {
		t.Fatal("expected restored session ordering and active window")
	}

	activeWinAny := <-sessionRef.Ask(GetActiveWindow{})
	if activeWinAny == nil {
		t.Fatal("expected active window after restore")
	}
	activeWin := activeWinAny.(*WindowRef)

	winResult := <-activeWin.Ask(GetWindowSnapshotData{})
	winData, ok := winResult.(WindowSnapshot)
	if !ok {
		t.Fatalf("expected WindowSnapshot, got %T", winResult)
	}
	if winData.ActivePane != 7 {
		t.Fatalf("expected active pane 7, got %d", winData.ActivePane)
	}
	if len(winData.PaneOrder) != 2 || winData.PaneOrder[0] != 3 || winData.PaneOrder[1] != 7 {
		t.Fatalf("unexpected pane order: %#v", winData.PaneOrder)
	}
	if !equalSplitTreeSnapshot(winData.Layout, snapshot.Windows[0].Layout) {
		t.Fatalf("expected restored layout %#v, got %#v", snapshot.Windows[0].Layout, winData.Layout)
	}

	if !pollFor(2*time.Second, func() bool {
		paneAny := <-sessionRef.Ask(GetActivePane{})
		if paneAny == nil {
			return false
		}
		pane := paneAny.(*PaneRef)
		snapAny := <-pane.Ask(GetPaneSnapshotData{})
		paneData, ok := snapAny.(PaneSnapshotData)
		if !ok {
			return false
		}
		return paneData.ID == 7 && paneData.CWD == cwd
	}) {
		t.Fatal("expected restored active pane and cwd")
	}
}

func TestRestoreSessionPreservesNestedSplitTree(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := setTestHome(t, tmpDir)
	defer restoreHome(t, oldHome)

	super := newTestSupervisor()
	snapshot := &SessionSnapshot{
		Version:      SnapshotVersion,
		SessionName:  "nested",
		ID:           7,
		Shell:        "/bin/sh",
		ActiveWindow: 1,
		WindowOrder:  []uint32{1},
		Windows: []WindowSnapshot{{
			ID:         1,
			ActivePane: 4,
			PaneOrder:  []uint32{1, 2, 3, 4},
			Panes: []PaneSnapshot{
				{ID: 1, Shell: "/bin/sh", Rows: 12, Cols: 40},
				{ID: 2, Shell: "/bin/sh", Rows: 12, Cols: 39},
				{ID: 3, Shell: "/bin/sh", Rows: 11, Cols: 40},
				{ID: 4, Shell: "/bin/sh", Rows: 11, Cols: 39},
			},
			Layout: &SplitTreeSnapshot{
				Dir:   SplitV,
				Ratio: 0.5,
				First: &SplitTreeSnapshot{
					Dir:    SplitH,
					Ratio:  0.5,
					First:  &SplitTreeSnapshot{PaneID: 1},
					Second: &SplitTreeSnapshot{PaneID: 3},
				},
				Second: &SplitTreeSnapshot{
					Dir:    SplitH,
					Ratio:  0.5,
					First:  &SplitTreeSnapshot{PaneID: 2},
					Second: &SplitTreeSnapshot{PaneID: 4},
				},
			},
		}},
	}
	logger := testLogger()
	if err := SaveSnapshot(SessionSnapshotPath("nested"), snapshot, logger); err != nil {
		t.Fatalf("SaveSnapshot failed: %v", err)
	}

	sessionRef, err := RestoreSessionFromSnapshot("nested", super.handle, logger)
	if err != nil {
		t.Fatalf("RestoreSessionFromSnapshot failed: %v", err)
	}
	defer sessionRef.Shutdown()

	activeWinAny := <-sessionRef.Ask(GetActiveWindow{})
	if activeWinAny == nil {
		t.Fatal("expected active window after restore")
	}
	activeWin := activeWinAny.(*WindowRef)

	if !pollFor(2*time.Second, func() bool {
		winResult := <-activeWin.Ask(GetWindowSnapshotData{})
		winData, ok := winResult.(WindowSnapshot)
		return ok && equalSplitTreeSnapshot(winData.Layout, snapshot.Windows[0].Layout)
	}) {
		t.Fatal("expected nested split tree to be restored")
	}

	viewResult := <-sessionRef.Ask(GetWindowView{})
	view, ok := viewResult.(WindowView)
	if !ok || view.Content == "" {
		t.Fatal("expected non-empty restored window view")
	}
	if !contains(view.Content, "┼") {
		t.Fatal("expected restored nested split view to contain cross intersection")
	}
}
