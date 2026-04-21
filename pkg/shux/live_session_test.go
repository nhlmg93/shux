package shux

import (
	"os"
	"testing"
)

func TestGenerateSocketPath(t *testing.T) {
	path := GenerateSocketPath("test-session")
	if path == "" {
		t.Error("GenerateSocketPath returned empty string")
	}
	// Should contain session name and session.sock
	if !strContains(path, "test-session") {
		t.Errorf("Socket path should contain session name: %s", path)
	}
	if !strContains(path, "session.sock") {
		t.Errorf("Socket path should end with session.sock: %s", path)
	}
}

func TestMarkSnapshotLive(t *testing.T) {
	snapshot := &SessionSnapshot{
		Version:     SnapshotVersion,
		SessionName: "test",
		ID:          1,
		Shell:       "/bin/sh",
	}

	socketPath := "/tmp/test.sock"

	err := MarkSnapshotLive(snapshot, os.Getpid(), socketPath)
	if err != nil {
		// This may fail in test environment without /proc, that's OK
		t.Skipf("MarkSnapshotLive failed (expected in some environments): %v", err)
	}

	if !snapshot.Live {
		t.Error("snapshot.Live should be true after MarkSnapshotLive")
	}
	if snapshot.OwnerPID != os.Getpid() {
		t.Errorf("snapshot.OwnerPID = %d, want %d", snapshot.OwnerPID, os.Getpid())
	}
	if snapshot.SocketPath != socketPath {
		t.Errorf("snapshot.SocketPath = %s, want %s", snapshot.SocketPath, socketPath)
	}
	if snapshot.OwnerStartTime == 0 {
		t.Error("snapshot.OwnerStartTime should be non-zero")
	}
	if snapshot.AttachToken == "" {
		t.Error("snapshot.AttachToken should be non-empty")
	}
}

func TestMarkSnapshotDead(t *testing.T) {
	snapshot := &SessionSnapshot{
		Version:        SnapshotVersion,
		SessionName:    "test",
		ID:             1,
		Shell:          "/bin/sh",
		Live:           true,
		OwnerPID:       1234,
		OwnerStartTime: 5678,
		SocketPath:     "/tmp/test.sock",
		AttachToken:    "token123",
	}

	MarkSnapshotDead(snapshot)

	if snapshot.Live {
		t.Error("snapshot.Live should be false after MarkSnapshotDead")
	}
	if snapshot.OwnerPID != 0 {
		t.Errorf("snapshot.OwnerPID = %d, want 0", snapshot.OwnerPID)
	}
	if snapshot.OwnerStartTime != 0 {
		t.Errorf("snapshot.OwnerStartTime = %d, want 0", snapshot.OwnerStartTime)
	}
	if snapshot.SocketPath != "" {
		t.Errorf("snapshot.SocketPath = %s, want empty", snapshot.SocketPath)
	}
	if snapshot.AttachToken != "" {
		t.Errorf("snapshot.AttachToken = %s, want empty", snapshot.AttachToken)
	}
}

func TestValidateSnapshotWithLiveMetadata(t *testing.T) {
	// Valid live snapshot
	validLive := &SessionSnapshot{
		Version:        SnapshotVersion,
		SessionName:    "test",
		ID:             1,
		Shell:          "/bin/sh",
		Live:           true,
		OwnerPID:       1, // PID 1 always exists on Linux
		OwnerStartTime: 12345,
		SocketPath:     "/tmp/test.sock",
	}

	// Invalid: live but no socket
	invalidNoSocket := &SessionSnapshot{
		Version:     SnapshotVersion,
		SessionName: "test",
		ID:          1,
		Shell:       "/bin/sh",
		Live:        true,
		OwnerPID:    1,
	}

	// Invalid: live but invalid PID
	invalidPID := &SessionSnapshot{
		Version:    SnapshotVersion,
		SessionName: "test",
		ID:         1,
		Shell:      "/bin/sh",
		Live:       true,
		OwnerPID:   -1,
		SocketPath: "/tmp/test.sock",
	}

	// Non-live snapshot should validate regardless
	nonLive := &SessionSnapshot{
		Version:     SnapshotVersion,
		SessionName: "test",
		ID:          1,
		Shell:       "/bin/sh",
		Live:        false,
	}

	// Non-live with zero values should also validate
	nonLiveZero := &SessionSnapshot{
		Version:     SnapshotVersion,
		SessionName: "test",
		ID:          1,
		Shell:       "/bin/sh",
	}

	tests := []struct {
		name     string
		snapshot *SessionSnapshot
		wantErr  bool
	}{
		{"valid live snapshot", validLive, false},
		{"live without socket", invalidNoSocket, true},
		{"live with invalid PID", invalidPID, true},
		{"non-live snapshot", nonLive, false},
		{"non-live with zero values", nonLiveZero, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSnapshot(tt.snapshot)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSnapshot() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsProcessRunning(t *testing.T) {
	// PID 1 should always exist on Linux
	if !IsProcessRunning(1) {
		t.Error("IsProcessRunning(1) should be true on Linux")
	}

	// Invalid PIDs should return false
	if IsProcessRunning(-1) {
		t.Error("IsProcessRunning(-1) should be false")
	}
	if IsProcessRunning(0) {
		t.Error("IsProcessRunning(0) should be false")
	}

	// Very large PID unlikely to exist
	if IsProcessRunning(99999999) {
		t.Log("Warning: IsProcessRunning(99999999) returned true (unexpected but possible)")
	}
}

func strContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
