package shux

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestGetProcessCWD(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("CWD reading only works on Linux")
	}

	// Create a temporary directory to use as CWD
	tmpDir, err := os.MkdirTemp("", "shux-cwd-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Fatalf("Failed to remove temp dir: %v", err)
		}
	}()

	// Start a shell process in that directory
	cmd := exec.Command("/bin/sh", "-c", "sleep 30")
	cmd.Dir = tmpDir
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start test process: %v", err)
	}
	defer func() {
		if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			t.Fatalf("Failed to kill test process: %v", err)
		}
	}()

	// Give the process time to start
	time.Sleep(50 * time.Millisecond)

	// Get CWD
	pid := cmd.Process.Pid
	cwd, err := GetProcessCWD(pid)
	if err != nil {
		t.Fatalf("GetProcessCWD failed: %v", err)
	}

	// Resolve paths for comparison
	resolvedTmp, _ := filepath.EvalSymlinks(tmpDir)
	resolvedCwd, _ := filepath.EvalSymlinks(cwd)

	if resolvedCwd != resolvedTmp {
		t.Errorf("CWD mismatch: got %s, want %s", resolvedCwd, resolvedTmp)
	}
}

func TestGetProcessCWDNonExistent(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("CWD reading only works on Linux")
	}

	// Try to get CWD of a process that doesn't exist (PID 99999 is unlikely)
	_, err := GetProcessCWD(99999)
	if err == nil {
		t.Error("Expected error for non-existent process, got nil")
	}
}

func TestParseStatFields(t *testing.T) {
	// Test case from a real /proc/<pid>/stat line
	statLine := "1234 (bash) S 1233 1233 1233 34816 1233 4194304 1234 0 0 0 10 20 0 0 20 0 1 0 1234567890 1234567 18446744073709551615"

	fields := parseStatFields(statLine)

	// Field 21 (0-indexed: 21) should be start time
	if len(fields) < 22 {
		t.Fatalf("Expected at least 22 fields, got %d", len(fields))
	}

	// Field 0 should be PID
	if fields[0] != "1234" {
		t.Errorf("Field 0 (PID): got %s, want 1234", fields[0])
	}

	// Field 1 should be command with parentheses
	if fields[1] != "(bash)" {
		t.Errorf("Field 1 (command): got %s, want (bash)", fields[1])
	}

	// Field 21 should be start time
	if fields[21] != "1234567890" {
		t.Errorf("Field 21 (start time): got %s, want 1234567890", fields[21])
	}
}

func TestParseStatFieldsWithSpacesInCommand(t *testing.T) {
	// Test case with spaces in command name (e.g., "my app")
	statLine := "1234 (my app with spaces) S 1233 1233 1233 34816 1233 4194304 1234 0 0 0 10 20 0 0 20 0 1 0 1234567890 1234567 18446744073709551615"

	fields := parseStatFields(statLine)

	if len(fields) < 22 {
		t.Fatalf("Expected at least 22 fields, got %d", len(fields))
	}

	if fields[0] != "1234" {
		t.Errorf("Field 0 (PID): got %s, want 1234", fields[0])
	}

	if fields[1] != "(my app with spaces)" {
		t.Errorf("Field 1 (command): got %s, want (my app with spaces)", fields[1])
	}
}

func TestGetProcessStartTime(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Process start time reading only works on Linux")
	}

	// Use our own PID
	ownPid := os.Getpid()

	startTime, err := GetProcessStartTime(ownPid)
	if err != nil {
		t.Fatalf("GetProcessStartTime failed for self: %v", err)
	}

	// Start time should be a positive number (in clock ticks since boot)
	if startTime == 0 {
		t.Error("Expected non-zero start time")
	}

	t.Logf("Our process start time: %d", startTime)
}

func TestGetProcessStartTimeNonExistent(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Process start time reading only works on Linux")
	}

	_, err := GetProcessStartTime(99999)
	if err == nil {
		t.Error("Expected error for non-existent process, got nil")
	}
}

func TestResolveCWD(t *testing.T) {
	// Test normal path
	tmpDir := t.TempDir()
	resolved := ResolveCWD(tmpDir)
	if resolved != tmpDir {
		t.Errorf("ResolveCWD normal: got %s, want %s", resolved, tmpDir)
	}

	// Test deleted directory scenario
	deletedPath := "/home/user/olddir (deleted)"
	resolved = ResolveCWD(deletedPath)
	// It should try to use parent
	if resolved == deletedPath {
		t.Error("ResolveCWD should handle (deleted) suffix")
	}
}

func TestVerifyProcess(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Process verification only works on Linux")
	}

	// Use our own PID
	ownPid := os.Getpid()

	// Get our start time
	startTime, err := GetProcessStartTime(ownPid)
	if err != nil {
		t.Fatalf("Failed to get own start time: %v", err)
	}

	// Verify should succeed with correct start time
	if !VerifyProcess(ownPid, startTime) {
		t.Error("VerifyProcess should succeed for self with correct start time")
	}

	// Verify should fail with wrong start time
	if VerifyProcess(ownPid, startTime+1000) {
		t.Error("VerifyProcess should fail with wrong start time")
	}

	// Verify should fail for non-existent process
	if VerifyProcess(99999, startTime) {
		t.Error("VerifyProcess should fail for non-existent process")
	}
}
