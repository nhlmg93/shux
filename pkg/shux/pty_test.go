package shux

import (
	"bufio"
	"os/exec"
	"strings"
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
