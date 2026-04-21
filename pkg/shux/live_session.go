package shux

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ReadProcessStartTime reads the start time of a process from /proc.
// Returns the start time in clock ticks for comparison.
func ReadProcessStartTime(pid int) (uint64, error) {
	if pid <= 0 {
		return 0, fmt.Errorf("invalid PID: %d", pid)
	}

	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	data, err := os.ReadFile(statPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read process stat: %w", err)
	}

	// Parse the stat file - field 22 is starttime
	// Format: pid (comm) state ppid pgrp session tty_nr tpgid flags minflt cminflt majflt cmajflt utime stime cutime cstime priority nice num_threads itrealvalue starttime ...
	// The challenge is that comm can contain spaces and parentheses
	fields := parseProcStat(string(data))
	if len(fields) < 22 {
		return 0, fmt.Errorf("stat file has insufficient fields")
	}

	// Field 22 (index 21) is starttime
	startTime, err := strconv.ParseUint(fields[21], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse start time: %w", err)
	}

	return startTime, nil
}

// parseProcStat parses /proc/PID/stat, handling the comm field which may contain spaces.
func parseProcStat(data string) []string {
	// Find the last ')' which ends the comm field
	endIdx := strings.LastIndex(data, ")")
	if endIdx == -1 {
		// Malformed, just split by space
		return strings.Fields(data)
	}

	// Split into: before comm, comm itself, after comm
	before := data[:endIdx+1]
	after := data[endIdx+1:]

	// Remove leading ( from comm for cleaner parsing
	beforeFields := strings.Fields(before)
	if len(beforeFields) >= 2 {
		// beforeFields[0] = pid, beforeFields[1] = (comm)
		beforeFields = beforeFields[:1] // Keep just pid
	}

	// The rest are space-separated fields
	afterFields := strings.Fields(after)

	return append(beforeFields, afterFields...)
}

// IsProcessRunning checks if a process with the given PID is currently running.
func IsProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	_, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
	return err == nil
}

// IsLiveOwner checks if the snapshot points to a valid live owner.
// This verifies both that the process exists and that start time matches
// (preventing false positives from PID reuse).
func IsLiveOwner(snapshot *SessionSnapshot) bool {
	if snapshot == nil || !snapshot.Live {
		return false
	}

	// Quick check: process must exist
	if !IsProcessRunning(snapshot.OwnerPID) {
		return false
	}

	// Verify start time to guard against PID reuse
	currentStartTime, err := ReadProcessStartTime(snapshot.OwnerPID)
	if err != nil {
		return false
	}

	return currentStartTime == snapshot.OwnerStartTime
}

// ValidateLiveOwner performs full validation of a live owner and returns detailed error.
func ValidateLiveOwner(snapshot *SessionSnapshot) error {
	if snapshot == nil {
		return fmt.Errorf("snapshot is nil")
	}

	if !snapshot.Live {
		return fmt.Errorf("snapshot is not marked as live")
	}

	if snapshot.OwnerPID <= 0 {
		return fmt.Errorf("invalid owner PID: %d", snapshot.OwnerPID)
	}

	if !IsProcessRunning(snapshot.OwnerPID) {
		return fmt.Errorf("owner process %d is not running", snapshot.OwnerPID)
	}

	currentStartTime, err := ReadProcessStartTime(snapshot.OwnerPID)
	if err != nil {
		return fmt.Errorf("cannot read owner process start time: %w", err)
	}

	if currentStartTime != snapshot.OwnerStartTime {
		return fmt.Errorf("owner PID %d has different start time (possible PID reuse)", snapshot.OwnerPID)
	}

	if snapshot.SocketPath == "" {
		return fmt.Errorf("snapshot has no socket path")
	}

	// Check socket exists and is accessible
	if _, err := os.Stat(snapshot.SocketPath); err != nil {
		return fmt.Errorf("socket not accessible: %w", err)
	}

	return nil
}

// TryDialLiveOwner attempts to connect to a live owner.
// Returns the connected IPC client or an error.
func TryDialLiveOwner(snapshot *SessionSnapshot) (*IPCClient, error) {
	if err := ValidateLiveOwner(snapshot); err != nil {
		return nil, err
	}

	client, err := DialIPC(snapshot.SocketPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to dial live owner: %w", err)
	}

	return client, nil
}

// MarkSnapshotLive updates a snapshot with live owner metadata.
// This should be called when an owner process starts.
func MarkSnapshotLive(snapshot *SessionSnapshot, pid int, socketPath string) error {
	if snapshot == nil {
		return fmt.Errorf("snapshot is nil")
	}

	startTime, err := ReadProcessStartTime(pid)
	if err != nil {
		return fmt.Errorf("failed to get process start time: %w", err)
	}

	snapshot.Live = true
	snapshot.OwnerPID = pid
	snapshot.OwnerStartTime = startTime
	snapshot.SocketPath = socketPath
	// Generate a simple token (not cryptographically secure, but sufficient for v1)
	snapshot.AttachToken = fmt.Sprintf("%d-%d", pid, time.Now().UnixNano())

	return nil
}

// MarkSnapshotDead clears live owner metadata from a snapshot.
// This should be called when an owner process exits.
func MarkSnapshotDead(snapshot *SessionSnapshot) {
	if snapshot == nil {
		return
	}
	snapshot.Live = false
	snapshot.OwnerPID = 0
	snapshot.OwnerStartTime = 0
	snapshot.SocketPath = ""
	snapshot.AttachToken = ""
}

// GenerateSocketPath generates a unique socket path for a session.
// Uses the session directory to keep sockets organized.
func GenerateSocketPath(sessionName string) string {
	return SessionSocketPath(sessionName)
}

// CleanupSocket removes a stale socket file if the owner is not running.
func CleanupSocket(sessionName string) error {
	socketPath := SessionSocketPath(sessionName)

	// Check if socket file exists
	info, err := os.Stat(socketPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Already clean
		}
		return err
	}

	// If it's a socket, try to connect to verify it's not in use
	if info.Mode()&os.ModeSocket != 0 {
		// Try to dial with timeout - if it fails, socket is stale
		conn, err := net.DialTimeout("unix", socketPath, 2*time.Second)
		if err == nil {
			_ = conn.Close()
			return nil // Socket is active
		}
		// Failed to connect - socket is stale, remove it
	}

	return os.Remove(socketPath)
}

// LockSessionDir attempts to create a lock file to prevent concurrent owner startups.
// Returns the lock file path and a cleanup function, or an error.
func LockSessionDir(sessionName string) (string, func(), error) {
	lockPath := filepath.Join(SessionDir(sessionName), ".owner.lock")

	// Try to create lock file exclusively
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", nil, fmt.Errorf("session lock failed (another owner starting?): %w", err)
	}

	// Write our PID to the lock file
	pid := os.Getpid()
	_, _ = fmt.Fprintf(file, "%d\n", pid)
	_ = file.Close()

	cleanup := func() {
		_ = os.Remove(lockPath)
	}

	return lockPath, cleanup, nil
}
