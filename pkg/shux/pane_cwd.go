package shux

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GetProcessCWD reads the current working directory of a process via /proc.
func GetProcessCWD(pid int) (string, error) {
	cwdPath := fmt.Sprintf("/proc/%d/cwd", pid)

	// Read the symlink to get the actual path
	cwd, err := os.Readlink(cwdPath)
	if err != nil {
		return "", fmt.Errorf("failed to read process CWD: %w", err)
	}

	return cwd, nil
}

// GetProcessStartTime reads the process start time for PID reuse detection.
// Returns the start time value from /proc/<pid>/stat (field 22).
func GetProcessStartTime(pid int) (uint64, error) {
	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	data, err := os.ReadFile(statPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read process stat: %w", err)
	}

	// Parse the stat file - start time is field 22
	// The command name is in parentheses and may contain spaces, so we need to parse carefully
	fields := parseStatFields(string(data))
	if len(fields) < 22 {
		return 0, fmt.Errorf("stat file has insufficient fields")
	}

	var startTime uint64
	_, err = fmt.Sscanf(fields[21], "%d", &startTime)
	if err != nil {
		return 0, fmt.Errorf("failed to parse start time: %w", err)
	}

	return startTime, nil
}

// parseStatFields parses /proc/<pid>/stat fields, handling the command name in parentheses.
func parseStatFields(data string) []string {
	// Find the first '(' and the last ')'
	start := 0
	for i, c := range data {
		if c == '(' {
			start = i
			break
		}
	}

	end := len(data)
	for i := len(data) - 1; i >= 0; i-- {
		if data[i] == ')' {
			end = i
			break
		}
	}

	if start >= end {
		// No parentheses found, just split
		return splitFields(data)
	}

	// Split: before '(' + command + after ')'
	before := splitFields(data[:start])
	command := data[start : end+1]
	after := splitFields(data[end+1:])

	result := append(before, command)
	result = append(result, after...)
	return result
}

func splitFields(s string) []string {
	var fields []string
	var current string
	inSpace := true

	for _, c := range s {
		if c == ' ' || c == '\t' || c == '\n' {
			if !inSpace {
				fields = append(fields, current)
				current = ""
				inSpace = true
			}
		} else {
			current += string(c)
			inSpace = false
		}
	}

	if !inSpace {
		fields = append(fields, current)
	}

	return fields
}

// VerifyProcess checks if a PID still refers to the expected process.
// It compares the start time against the expected value to detect PID reuse.
func VerifyProcess(pid int, expectedStartTime uint64) bool {
	startTime, err := GetProcessStartTime(pid)
	if err != nil {
		return false
	}
	return startTime == expectedStartTime
}

// ResolveCWD resolves the CWD, handling cases where the symlink points to a deleted directory.
// If the CWD has been deleted (shows as " (deleted)" suffix), it attempts to find the parent.
func ResolveCWD(cwd string) string {
	// Check if the path exists and is accessible
	if _, err := os.Stat(cwd); err == nil {
		return cwd
	}

	// If path ends with " (deleted)", try the parent
	for strings.HasSuffix(cwd, " (deleted)") {
		cwd = filepath.Dir(strings.TrimSuffix(cwd, " (deleted)"))
		if cwd == "/" || cwd == "." {
			break
		}
		if _, err := os.Stat(cwd); err == nil {
			return cwd
		}
	}

	return cwd
}
