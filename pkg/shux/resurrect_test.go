package shux

import (
	"os"
	"testing"
	"time"

	"github.com/nhlmg93/gotor/actor"
)

func TestSpawnSessionFromSnapshot(t *testing.T) {
	super := &testSupervisor{}
	superRef := actor.Spawn(super, 10)
	defer superRef.Shutdown()

	snapshot := &SessionSnapshot{
		Version:      SnapshotVersion,
		SessionName:  "restored",
		ID:           1,
		Shell:        "/bin/sh",
		ActiveWindow: 1,
		WindowOrder:  []uint32{1},
		Windows: []WindowSnapshot{
			{
				ID:         1,
				ActivePane: 1,
				PaneOrder:  []uint32{1},
				Panes: []PaneSnapshot{
					{ID: 1, Shell: "/bin/sh", Rows: 24, Cols: 80, CWD: "/tmp"},
				},
			},
		},
	}

	sessionRef := SpawnSessionFromSnapshot(snapshot, superRef)
	if sessionRef == nil {
		t.Fatal("SpawnSessionFromSnapshot returned nil")
	}

	// Give time for initialization
	time.Sleep(100 * time.Millisecond)

	// Verify session exists and has the right ID
	reply := sessionRef.Ask(GetSessionSnapshotData{})
	result := <-reply

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

	// Verify windows were restored
	if len(data.WindowOrder) != 1 {
		t.Errorf("Window count: got %d, want 1", len(data.WindowOrder))
	}

	sessionRef.Shutdown()
}

func TestSessionSnapshotDataRoundTrip(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	// Create a window
	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	win := requireWindow(t, sessionRef, super)

	// Get session snapshot data
	reply := sessionRef.Ask(GetSessionSnapshotData{})
	result := <-reply

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

	// Get window snapshot data
	winReply := win.Ask(GetWindowSnapshotData{})
	winResult := <-winReply

	winData, ok := winResult.(WindowSnapshot)
	if !ok {
		t.Fatalf("Expected WindowSnapshot, got %T", winResult)
	}

	if winData.ID == 0 {
		t.Error("Window ID should not be 0")
	}

	// Verify pane exists in window data
	if len(winData.PaneOrder) != 1 {
		t.Errorf("PaneOrder length: got %d, want 1", len(winData.PaneOrder))
	}
}

func TestBuildSnapshotEmptySession(t *testing.T) {
	sessionRef, _, cleanup := setupSession(t)
	defer cleanup()

	// Don't create any windows
	reply := sessionRef.Ask(GetSessionSnapshotData{})
	result := <-reply

	data, ok := result.(SessionSnapshotData)
	if !ok {
		t.Fatalf("Expected SessionSnapshotData, got %T", result)
	}

	if len(data.WindowOrder) != 0 {
		t.Errorf("Empty session should have 0 windows, got %d", len(data.WindowOrder))
	}
}

func TestRestoreSessionFromSnapshot(t *testing.T) {
	// This test requires a saved snapshot on disk
	tmpDir := t.TempDir()
	oldHome := setTestHome(tmpDir)
	defer restoreHome(oldHome)

	super := &testSupervisor{}
	superRef := actor.Spawn(super, 10)
	defer superRef.Shutdown()

	// Create and save a snapshot
	snapshot := &SessionSnapshot{
		Version:      SnapshotVersion,
		SessionName:  "test-session",
		ID:           42,
		Shell:        "/bin/sh",
		ActiveWindow: 1,
		WindowOrder:  []uint32{1, 2},
		Windows: []WindowSnapshot{
			{
				ID:         1,
				ActivePane: 1,
				PaneOrder:  []uint32{1},
				Panes:      []PaneSnapshot{{ID: 1, Shell: "/bin/sh", Rows: 24, Cols: 80}},
			},
			{
				ID:         2,
				ActivePane: 1,
				PaneOrder:  []uint32{1},
				Panes:      []PaneSnapshot{{ID: 1, Shell: "/bin/sh", Rows: 30, Cols: 100}},
			},
		},
	}

	if err := EnsureSessionDir("test-session"); err != nil {
		t.Fatalf("EnsureSessionDir failed: %v", err)
	}

	if err := SaveSnapshot(SessionSnapshotPath("test-session"), snapshot); err != nil {
		t.Fatalf("SaveSnapshot failed: %v", err)
	}

	// Restore from snapshot
	sessionRef, err := RestoreSessionFromSnapshot("test-session", superRef)
	if err != nil {
		t.Fatalf("RestoreSessionFromSnapshot failed: %v", err)
	}

	if sessionRef == nil {
		t.Fatal("RestoreSessionFromSnapshot returned nil ref")
	}

	// Give time for initialization
	time.Sleep(200 * time.Millisecond)

	// Verify session was restored
	reply := sessionRef.Ask(GetSessionSnapshotData{})
	result := <-reply

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

	sessionRef.Shutdown()
}

func TestRestoreSessionFromSnapshotNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := setTestHome(tmpDir)
	defer restoreHome(oldHome)

	super := &testSupervisor{}
	superRef := actor.Spawn(super, 10)
	defer superRef.Shutdown()

	_, err := RestoreSessionFromSnapshot("nonexistent", superRef)
	if err == nil {
		t.Error("Expected error for non-existent snapshot")
	}
}

func setTestHome(dir string) string {
	old := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	return old
}

func restoreHome(old string) {
	os.Setenv("HOME", old)
}

func TestSessionDetachSavesNamedSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := setTestHome(tmpDir)
	defer restoreHome(oldHome)

	super := newTestSupervisor()
	superRef := actor.Spawn(super, 10)
	defer superRef.Shutdown()

	sessionRef := SpawnNamedSessionWithShell(1, "work", "/bin/sh", superRef)
	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	requirePane(t, sessionRef, super)

	reply := sessionRef.Ask(DetachSession{})
	if err, _ := (<-reply).(error); err != nil {
		t.Fatalf("detach failed: %v", err)
	}

	path := SessionSnapshotPath("work")
	if !pollFor(500*time.Millisecond, func() bool {
		_, err := os.Stat(path)
		return err == nil
	}) {
		t.Fatalf("expected snapshot at %s", path)
	}

	snapshot, err := LoadSnapshot(path)
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if snapshot.SessionName != "work" {
		t.Fatalf("expected snapshot session name %q, got %q", "work", snapshot.SessionName)
	}
}

func TestRestoreSessionPreservesSelectionsAndCWD(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := setTestHome(tmpDir)
	defer restoreHome(oldHome)

	cwd := t.TempDir()
	super := newTestSupervisor()
	superRef := actor.Spawn(super, 10)
	defer superRef.Shutdown()

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
			},
			{
				ID:         5,
				ActivePane: 1,
				PaneOrder:  []uint32{1},
				Panes:      []PaneSnapshot{{ID: 1, Shell: "/bin/sh", Rows: 30, Cols: 100, CWD: cwd}},
			},
		},
	}
	if err := SaveSnapshot(SessionSnapshotPath("project"), snapshot); err != nil {
		t.Fatalf("SaveSnapshot failed: %v", err)
	}

	sessionRef, err := RestoreSessionFromSnapshot("project", superRef)
	if err != nil {
		t.Fatalf("RestoreSessionFromSnapshot failed: %v", err)
	}
	defer sessionRef.Shutdown()

	if !pollFor(2*time.Second, func() bool {
		reply := sessionRef.Ask(GetSessionSnapshotData{})
		result := <-reply
		data, ok := result.(SessionSnapshotData)
		if !ok {
			return false
		}
		return data.ActiveWindow == 2 && len(data.WindowOrder) == 2 && data.WindowOrder[0] == 2 && data.WindowOrder[1] == 5
	}) {
		t.Fatal("expected restored session ordering and active window")
	}

	activeWinReply := sessionRef.Ask(GetActiveWindow{})
	activeWinAny := <-activeWinReply
	if activeWinAny == nil {
		t.Fatal("expected active window after restore")
	}
	activeWin := activeWinAny.(*actor.Ref)

	winReply := activeWin.Ask(GetWindowSnapshotData{})
	winResult := <-winReply
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

	if !pollFor(2*time.Second, func() bool {
		paneReply := sessionRef.Ask(GetActivePane{})
		paneAny := <-paneReply
		if paneAny == nil {
			return false
		}
		pane := paneAny.(*actor.Ref)
		snapReply := pane.Ask(GetPaneSnapshotData{})
		snapAny := <-snapReply
		paneData, ok := snapAny.(PaneSnapshotData)
		if !ok {
			return false
		}
		return paneData.ID == 7 && paneData.CWD == cwd
	}) {
		t.Fatal("expected restored active pane and cwd")
	}
}
