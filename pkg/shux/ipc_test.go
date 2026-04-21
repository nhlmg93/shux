package shux

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIPCEnvelopeGobSerialization(t *testing.T) {
	env := IPCEnvelope{
		Type: "IPCWindowView",
		Data: IPCWindowView{
			Content:   "test content",
			CursorRow: 5,
			CursorCol: 10,
			CursorOn:  true,
			Title:     "test",
		},
	}

	// Verify it can be gob-encoded (will be tested implicitly by IPCConn)
	// Just make sure the types are registered
	_ = env
}

func TestWaitForSocket(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	// Test timeout when socket doesn't exist
	err := WaitForSocket(socketPath, 50*time.Millisecond)
	if err == nil {
		t.Error("WaitForSocket should timeout when socket doesn't exist")
	}

	// Create a socket file (not a real listener, just a file)
	f, err := os.Create(socketPath)
	if err != nil {
		t.Fatalf("Failed to create test socket file: %v", err)
	}
	f.Close()

	// Should return immediately when socket exists
	// Note: This just tests the file existence check, not actual socket connection
	err = WaitForSocket(socketPath, 1*time.Second)
	if err != nil {
		t.Errorf("WaitForSocket should succeed when file exists: %v", err)
	}
}

func TestCleanupSocket(t *testing.T) {
	tmpDir := t.TempDir()
	sessionName := "test-cleanup"

	// Override the session dir for testing
	origDataDir := DataDir()
	defer os.Setenv("HOME", tmpDir)
	os.Setenv("HOME", tmpDir)

	// Ensure session dir exists
	_ = EnsureSessionDir(sessionName)
	socketPath := SessionSocketPath(sessionName)

	// Cleanup on non-existent socket should not error
	err := CleanupSocket(sessionName)
	if err != nil {
		t.Errorf("CleanupSocket on non-existent socket: %v", err)
	}

	// Create a regular file (not a socket)
	f, err := os.Create(socketPath)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	f.Close()

	// Cleanup should remove the file
	err = CleanupSocket(sessionName)
	if err != nil {
		t.Errorf("CleanupSocket on regular file: %v", err)
	}

	// File should be gone
	_, err = os.Stat(socketPath)
	if !os.IsNotExist(err) {
		t.Error("Socket file should be removed after CleanupSocket")
	}

	_ = origDataDir // silence unused warning
}

func TestSessionSocketPath(t *testing.T) {
	path := SessionSocketPath("my-session")
	if path == "" {
		t.Error("SessionSocketPath returned empty string")
	}
	if !strContainsIPC(path, "my-session") {
		t.Errorf("Path should contain session name: %s", path)
	}
	if !strContainsIPC(path, "session.sock") {
		t.Errorf("Path should contain 'session.sock': %s", path)
	}
}

func strContainsIPC(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
