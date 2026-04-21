package testutil

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"shux/pkg/shux"
)

// TestSupervisor captures events via channels for synchronization.
type TestSupervisor struct {
	SessionEmpty   chan uint32
	ContentUpdated chan uint32
}

// NewTestSupervisor creates a new test supervisor with buffered channels.
func NewTestSupervisor() *TestSupervisor {
	return &TestSupervisor{
		SessionEmpty:   make(chan uint32, 10),
		ContentUpdated: make(chan uint32, 10),
	}
}

// Handle processes messages from the session/window/pane actors.
func (s *TestSupervisor) Handle(msg any) {
	switch m := msg.(type) {
	case shux.SessionEmpty:
		select {
		case s.SessionEmpty <- m.ID:
		default:
		}
	case shux.PaneContentUpdated:
		select {
		case s.ContentUpdated <- m.ID:
		default:
		}
	}
}

// WaitSessionEmpty waits for a SessionEmpty event or returns false on timeout.
func (s *TestSupervisor) WaitSessionEmpty(timeout time.Duration) bool {
	select {
	case <-s.SessionEmpty:
		return true
	case <-time.After(timeout):
		return false
	}
}

// WaitContentUpdated waits for a PaneContentUpdated event or returns false on timeout.
func (s *TestSupervisor) WaitContentUpdated(timeout time.Duration) bool {
	select {
	case <-s.ContentUpdated:
		return true
	case <-time.After(timeout):
		return false
	}
}

// SetupSession creates a test session with supervisor and cleanup function.
// This is the primary entry point for integration tests.
func SetupSession(t *testing.T) (*shux.SessionRef, *TestSupervisor, func()) {
	t.Helper()

	super := NewTestSupervisor()
	logger := shux.NoOpLogger{}
	sessionRef := shux.StartSession(1, super.Handle, logger)

	cleanup := func() {
		sessionRef.Shutdown()
	}

	return sessionRef, super, cleanup
}

// SetupNamedSession creates a named test session.
func SetupNamedSession(t *testing.T, name string) (*shux.SessionRef, *TestSupervisor, func()) {
	t.Helper()

	super := NewTestSupervisor()
	logger := shux.NoOpLogger{}
	sessionRef := shux.StartNamedSessionWithShell(1, name, "/bin/sh", super.Handle, logger)

	cleanup := func() {
		sessionRef.Shutdown()
	}

	return sessionRef, super, cleanup
}

// RequirePane waits for and returns the active pane, failing on timeout.
func RequirePane(t *testing.T, sessionRef *shux.SessionRef, super *TestSupervisor) *shux.PaneRef {
	t.Helper()

	var paneRef any
	if !PollFor(500*time.Millisecond, func() bool {
		reply := sessionRef.Ask(shux.GetActivePane{})
		paneRef = <-reply
		return paneRef != nil
	}) {
		t.Fatal("timeout waiting for pane creation")
	}
	return paneRef.(*shux.PaneRef)
}

// RequireWindow waits for and returns the active window, failing on timeout.
func RequireWindow(t *testing.T, sessionRef *shux.SessionRef, super *TestSupervisor) *shux.WindowRef {
	t.Helper()

	var winRef any
	if !PollFor(500*time.Millisecond, func() bool {
		reply := sessionRef.Ask(shux.GetActiveWindow{})
		winRef = <-reply
		return winRef != nil
	}) {
		t.Fatal("timeout waiting for window creation")
	}
	return winRef.(*shux.WindowRef)
}

// PollFor polls a condition until it's true or timeout expires.
func PollFor(timeout time.Duration, condition func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// MustPoll fails the test if the condition doesn't become true within timeout.
func MustPoll(t *testing.T, timeout time.Duration, msg string, condition func() bool) {
	t.Helper()
	if !PollFor(timeout, condition) {
		t.Fatal(msg)
	}
}

// CreateWindowWithPane creates a window and waits for the pane to be ready.
func CreateWindowWithPane(t *testing.T, sessionRef *shux.SessionRef, super *TestSupervisor) *shux.WindowRef {
	t.Helper()

	sessionRef.Send(shux.CreateWindow{Rows: 24, Cols: 80})
	win := RequireWindow(t, sessionRef, super)
	_ = RequirePane(t, sessionRef, super)
	return win
}

// CreateMultiPaneWindow creates a window with multiple panes.
func CreateMultiPaneWindow(t *testing.T, sessionRef *shux.SessionRef, super *TestSupervisor, paneCount int) *shux.WindowRef {
	t.Helper()

	win := CreateWindowWithPane(t, sessionRef, super)

	for i := 1; i < paneCount; i++ {
		win.Send(shux.Split{Dir: shux.SplitV})
		WaitWindowPaneCount(t, win, i+1, 100*time.Millisecond)
	}

	return win
}

// WaitSessionWindowCount waits until a session reports the expected number of windows.
func WaitSessionWindowCount(t *testing.T, sessionRef *shux.SessionRef, want int, timeout time.Duration) shux.SessionSnapshotData {
	t.Helper()

	var data shux.SessionSnapshotData
	if !PollFor(timeout, func() bool {
		result := <-sessionRef.Ask(shux.GetSessionSnapshotData{})
		snapshot, ok := result.(shux.SessionSnapshotData)
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

// WaitWindowPaneCount waits until a window reports the expected number of panes.
func WaitWindowPaneCount(t *testing.T, windowRef *shux.WindowRef, want int, timeout time.Duration) shux.WindowSnapshot {
	t.Helper()

	var data shux.WindowSnapshot
	if !PollFor(timeout, func() bool {
		result := <-windowRef.Ask(shux.GetWindowSnapshotData{})
		snapshot, ok := result.(shux.WindowSnapshot)
		if !ok {
			return false
		}
		data = snapshot
		return len(snapshot.PaneOrder) == want
	}) {
		t.Fatalf("timeout waiting for %d panes", want)
	}
	return data
}

// SetTestHome sets HOME to a temp directory for isolated test state.
func SetTestHome(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("set HOME: %v", err)
	}
	return oldHome
}

// RestoreHome restores the original HOME directory.
func RestoreHome(t *testing.T, oldHome string) {
	t.Helper()

	if err := os.Setenv("HOME", oldHome); err != nil {
		t.Fatalf("restore HOME: %v", err)
	}
}

// WithTempHome runs a test with a temporary HOME directory.
func WithTempHome(t *testing.T, fn func(tempDir string)) {
	t.Helper()

	oldHome := SetTestHome(t)
	defer RestoreHome(t, oldHome)

	fn(os.Getenv("HOME"))
}

// BuildTestSnapshot creates a valid snapshot for testing.
func BuildTestSnapshot(sessionName string, windowCount int) *shux.SessionSnapshot {
	snapshot := &shux.SessionSnapshot{
		Version:      shux.SnapshotVersion,
		SessionName:  sessionName,
		ID:           1,
		Shell:        "/bin/sh",
		ActiveWindow: 0,
		WindowOrder:  make([]uint32, 0, windowCount),
		Windows:      make([]shux.WindowSnapshot, 0, windowCount),
	}

	for i := 0; i < windowCount; i++ {
		winID := uint32(i + 1)
		paneID := uint32(i*10 + 1)

		win := shux.WindowSnapshot{
			ID:         winID,
			ActivePane: paneID,
			PaneOrder:  []uint32{paneID},
			Panes: []shux.PaneSnapshot{
				{
					ID:    paneID,
					Shell: "/bin/sh",
					Rows:  24,
					Cols:  80,
					CWD:   "/tmp",
				},
			},
			Layout: &shux.SplitTreeSnapshot{PaneID: paneID},
		}

		snapshot.WindowOrder = append(snapshot.WindowOrder, winID)
		snapshot.Windows = append(snapshot.Windows, win)

		if i == 0 {
			snapshot.ActiveWindow = winID
		}
	}

	return snapshot
}

// BuildTestSnapshotWithLayout creates a snapshot with a specific split layout.
func BuildTestSnapshotWithLayout(sessionName string, layout *shux.SplitTreeSnapshot) *shux.SessionSnapshot {
	paneIDs := collectPaneIDsFromLayout(layout)
	panes := make([]shux.PaneSnapshot, len(paneIDs))
	for i, pid := range paneIDs {
		panes[i] = shux.PaneSnapshot{
			ID:    pid,
			Shell: "/bin/sh",
			Rows:  24,
			Cols:  80,
			CWD:   "/tmp",
		}
	}

	return &shux.SessionSnapshot{
		Version:      shux.SnapshotVersion,
		SessionName:  sessionName,
		ID:           1,
		Shell:        "/bin/sh",
		ActiveWindow: 1,
		WindowOrder:  []uint32{1},
		Windows: []shux.WindowSnapshot{
			{
				ID:         1,
				ActivePane: paneIDs[0],
				PaneOrder:  paneIDs,
				Panes:      panes,
				Layout:     layout,
			},
		},
	}
}

func collectPaneIDsFromLayout(layout *shux.SplitTreeSnapshot) []uint32 {
	if layout == nil {
		return nil
	}
	if layout.First == nil && layout.Second == nil {
		return []uint32{layout.PaneID}
	}
	result := collectPaneIDsFromLayout(layout.First)
	result = append(result, collectPaneIDsFromLayout(layout.Second)...)
	return result
}

// SnapshotPath returns the path where test snapshots are stored.
func SnapshotPath(tempDir, sessionName string) string {
	return filepath.Join(tempDir, ".local", "share", "shux", "sessions", sessionName, "snapshot.gob")
}

// EnsureTestSnapshotDir creates the snapshot directory for a test session.
func EnsureTestSnapshotDir(tempDir, sessionName string) error {
	dir := filepath.Join(tempDir, ".local", "share", "shux", "sessions", sessionName)
	return os.MkdirAll(dir, 0o750)
}

// TestLogger returns a logger for tests. Returns NoOpLogger if global Logger is not initialized.
func TestLogger() shux.ShuxLogger {
	if shux.Logger == nil {
		return shux.NoOpLogger{}
	}
	return &shux.StdLogger{Logger: shux.Logger}
}
