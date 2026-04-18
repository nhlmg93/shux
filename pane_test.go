package main

import (
	"bufio"
	"os/exec"
	"strings"
	"testing"
)

func TestNewPane(t *testing.T) {
	cmd := exec.Command("/bin/echo", "hello")
	pane, err := NewPane(cmd)
	if err != nil {
		t.Fatalf("Failed to create pane: %v", err)
	}
	defer pane.Close()

	if pane.ID == 0 {
		t.Error("Expected pane ID to be non-zero")
	}

	// Read output from pane's PTY
	reader := bufio.NewReader(pane.PTY.TTY)
	output, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read from pane: %v", err)
	}

	if !strings.Contains(output, "hello") {
		t.Errorf("Expected output to contain 'hello', got: %q", output)
	}
}

func TestPaneUniqueIDs(t *testing.T) {
	cmd1 := exec.Command("/bin/true")
	pane1, _ := NewPane(cmd1)
	defer pane1.Close()

	cmd2 := exec.Command("/bin/true")
	pane2, _ := NewPane(cmd2)
	defer pane2.Close()

	if pane1.ID == pane2.ID {
		t.Error("Expected pane IDs to be unique")
	}
}

func TestPaneExited(t *testing.T) {
	pane, _ := NewPane(exec.Command("/bin/true"))

	if pane.Exited() {
		t.Error("Expected pane to not be exited immediately after creation")
	}

	pane.PTY.Wait()

	if !pane.Exited() {
		t.Error("Expected pane to be exited after process finishes")
	}
}
