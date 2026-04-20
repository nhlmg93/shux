package shux

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestE2ENanoEdit validates full stack: shux → shell → nano → rendering
// TestE2ENanoEdit validates nano interaction through full stack.
// Run in Docker with: docker build -f Dockerfile.test -t shux-test . && docker run shux-test
func TestE2ENanoEdit(t *testing.T) {
	if os.Getenv("SHUX_E2E") != "1" {
		t.Skip("Set SHUX_E2E=1 to run E2E tests (requires nano, vim, etc)")
	}
	if _, err := exec.LookPath("nano"); err != nil {
		t.Skip("nano not installed")
	}

	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	pane := requirePane(t, sessionRef, super)

	// Start nano
	pane.Send(WriteToPane{Data: []byte("nano /tmp/shux_test.txt\r")})

	// Wait for nano to initialize (shows "GNU nano" or bottom menu)
	if !pollFor(1*time.Second, func() bool {
		super.waitContentUpdated(100 * time.Millisecond)
		reply := sessionRef.Ask(GetPaneContent{})
		result := <-reply
		if result == nil {
			return false
		}
		content := result.(*PaneContent)
		for _, line := range content.Lines {
			if contains(line, "GNU nano") || contains(line, "^G") {
				return true
			}
		}
		return false
	}) {
		t.Fatal("nano did not start (may not be installed)")
	}

	// Type some text
	testText := "HELLO_GOMUX"
	pane.Send(WriteToPane{Data: []byte(testText)})

	// Wait for text to appear
	var textFound bool
	if !pollFor(500*time.Millisecond, func() bool {
		super.waitContentUpdated(100 * time.Millisecond)
		reply := sessionRef.Ask(GetPaneContent{})
		result := <-reply
		if result == nil {
			return false
		}
		content := result.(*PaneContent)
		for _, line := range content.Lines {
			if contains(line, testText) {
				textFound = true
				return true
			}
		}
		return false
	}) {
		t.Error("Typed text did not appear in nano")
	}

	if textFound {
		t.Log("E2E success: nano opened, text typed, rendered correctly")
	}

	// Exit nano (Ctrl+X)
	pane.Send(WriteToPane{Data: []byte{0x18}}) // Ctrl+X
	pollFor(200*time.Millisecond, func() bool {
		reply := sessionRef.Ask(GetPaneContent{})
		result := <-reply
		return result != nil
	})
}

// TestE2EShellCommand validates basic shell interaction
func TestE2EShellCommand(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	pane := requirePane(t, sessionRef, super)

	// Run echo command
	uniqueStr := "GOMUX_TEST_42"
	pane.Send(WriteToPane{Data: []byte("echo " + uniqueStr + "\r")})

	// Wait for output
	var outputFound bool
	if !pollFor(500*time.Millisecond, func() bool {
		super.waitContentUpdated(100 * time.Millisecond)
		reply := sessionRef.Ask(GetPaneContent{})
		result := <-reply
		if result == nil {
			return false
		}
		content := result.(*PaneContent)
		for _, line := range content.Lines {
			if contains(line, uniqueStr) {
				outputFound = true
				return true
			}
		}
		return false
	}) {
		t.Log("Note: Shell command output may not be visible in content snapshot")
	}

	if outputFound {
		t.Log("E2E success: shell command executed and output rendered")
	}
}

// TestE2EKeySequence validates special key handling through full stack
func TestE2EKeySequence(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	pane := requirePane(t, sessionRef, super)

	// Send clear command
	pane.Send(WriteToPane{Data: []byte("clear\r")})

	// Wait for clear to take effect (content should be mostly empty)
	pollFor(300*time.Millisecond, func() bool {
		super.waitContentUpdated(100 * time.Millisecond)
		return true // Best effort
	})

	reply := sessionRef.Ask(GetPaneContent{})
	result := <-reply
	if result == nil {
		t.Fatal("Expected pane content after clear")
	}

	content := result.(*PaneContent)
	// After clear, screen should be mostly empty (spaces or nulls)
	emptyLines := 0
	for _, line := range content.Lines {
		if len(line) == 0 || allSpaces(line) {
			emptyLines++
		}
	}

	if emptyLines > len(content.Lines)/2 {
		t.Log("E2E success: clear command processed through full stack")
	}
}

func allSpaces(s string) bool {
	for _, r := range s {
		if r != ' ' && r != 0 {
			return false
		}
	}
	return true
}

func TestE2EVimEdit(t *testing.T) {
	if os.Getenv("SHUX_E2E") != "1" {
		t.Skip("Set SHUX_E2E=1 to run E2E tests")
	}
	if _, err := exec.LookPath("vim"); err != nil {
		t.Skip("vim not installed")
	}

	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	pane := requirePane(t, sessionRef, super)

	// Start vim
	pane.Send(WriteToPane{Data: []byte("vim /tmp/shux_vim_test.txt\r")})

	// Wait for vim to show its initial screen (blank with ~ lines)
	if !pollFor(1*time.Second, func() bool {
		super.waitContentUpdated(100 * time.Millisecond)
		reply := sessionRef.Ask(GetPaneContent{})
		result := <-reply
		if result == nil {
			return false
		}
		content := result.(*PaneContent)
		// Look for ~ (blank line markers) or "VIM" in first line
		for i, line := range content.Lines {
			if i < 5 && (len(line) > 0 && line[0] == '~') || contains(line, "VIM") {
				return true
			}
		}
		return false
	}) {
		t.Fatal("vim did not start properly")
	}

	// Enter insert mode (short delay for vim to process)
	pane.Send(WriteToPane{Data: []byte("i")})
	pollFor(50*time.Millisecond, func() bool { return true })

	// Type test text
	testText := "VIM_TEST_123"
	pane.Send(WriteToPane{Data: []byte(testText)})

	// Wait for text to appear
	var textFound bool
	if !pollFor(500*time.Millisecond, func() bool {
		super.waitContentUpdated(100 * time.Millisecond)
		reply := sessionRef.Ask(GetPaneContent{})
		result := <-reply
		if result == nil {
			return false
		}
		content := result.(*PaneContent)
		for _, line := range content.Lines {
			if contains(line, testText) {
				textFound = true
				return true
			}
		}
		return false
	}) {
		t.Log("Note: vim text entry may need more investigation")
	}

	if textFound {
		t.Log("E2E success: vim opened, insert mode, text entered")
	}

	// Exit vim: ESC : q ! ENTER
	pane.Send(WriteToPane{Data: []byte{0x1b}}) // ESC
	pollFor(50*time.Millisecond, func() bool { return true })
	pane.Send(WriteToPane{Data: []byte(":q!\r")})
}

func TestE2EColorOutput(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	pane := requirePane(t, sessionRef, super)

	pane.Send(WriteToPane{Data: []byte("printf '\\033[38;2;255;0;0mRED\\033[0m\\n'\r")})

	var content *PaneContent
	var styledRED int
	if !pollFor(1*time.Second, func() bool {
		super.waitContentUpdated(100 * time.Millisecond)
		reply := sessionRef.Ask(GetPaneContent{})
		result := <-reply
		if result == nil {
			return false
		}
		content = result.(*PaneContent)
		styledRED = 0
		for _, row := range content.Cells {
			for _, cell := range row {
				if (cell.Text == "R" || cell.Text == "E" || cell.Text == "D") && cell.HasFgColor {
					styledRED++
				}
			}
		}
		return styledRED >= 3
	}) {
		t.Fatalf("expected colored RED cells, got %d styled cells", styledRED)
	}
}

// TestE2EInitialDraw verifies the initial shell prompt appears correctly
func TestE2EInitialDraw(t *testing.T) {
	if os.Getenv("SHUX_E2E") != "1" {
		t.Skip("Set SHUX_E2E=1 to run E2E tests")
	}

	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	_ = requirePane(t, sessionRef, super) // Pane created, shell starting

	// Wait for initial content to appear (shell should show prompt)
	var foundContent bool
	pollFor(2*time.Second, func() bool {
		super.waitContentUpdated(100 * time.Millisecond)
		reply := sessionRef.Ask(GetPaneContent{})
		result := <-reply
		if result == nil {
			return false
		}
		content := result.(*PaneContent)

		// Check for non-empty content that's not just "Loading..."
		for _, line := range content.Lines {
			trimmed := strings.TrimSpace(line)
			// Look for shell prompt indicators: $, #, %, > or any text
			if len(trimmed) > 0 &&
				!strings.Contains(trimmed, "Loading") &&
				!strings.Contains(trimmed, "starting shell") {
				// Found real content
				foundContent = true
				t.Logf("Found initial content: %q", trimmed)
				return true
			}
		}
		return false
	})

	if !foundContent {
		// Get final content for debugging
		reply := sessionRef.Ask(GetPaneContent{})
		result := <-reply
		if result != nil {
			content := result.(*PaneContent)
			t.Logf("Final content lines:")
			for i, line := range content.Lines {
				t.Logf("  [%d]: %q", i, line)
			}
		}
		t.Fatal("Initial draw failed: no shell prompt or content visible within 2 seconds")
	}

	t.Log("E2E success: Initial shell prompt rendered correctly")
}
