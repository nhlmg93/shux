package shux

import (
	"bufio"
	"errors"
	"io"
	"os/exec"
	"strings"
	"syscall"
	"testing"
)

func TestPTYStart(t *testing.T) {
	cmd := exec.Command("/bin/echo", "hello")
	pty, err := Start(cmd)
	if err != nil {
		t.Fatalf("Failed to start PTY: %v", err)
	}
	defer pty.Close()

	// Read output from PTY
	reader := bufio.NewReader(pty.TTY)
	output, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read from PTY: %v", err)
	}

	if !strings.Contains(output, "hello") {
		t.Errorf("Expected output to contain 'hello', got: %q", output)
	}
}

func TestPTYStartWithSize(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-lc", "stty size")
	pty, err := StartWithSize(cmd, 7, 33)
	if err != nil {
		t.Fatalf("Failed to start sized PTY: %v", err)
	}
	defer pty.Close()

	output, err := io.ReadAll(pty.TTY)
	if err != nil && !errors.Is(err, syscall.EIO) {
		t.Fatalf("Failed to read sized PTY output: %v", err)
	}

	got := strings.TrimSpace(string(output))
	if !strings.Contains(got, "7 33") {
		t.Fatalf("expected sized PTY to report '7 33', got %q", got)
	}
}
