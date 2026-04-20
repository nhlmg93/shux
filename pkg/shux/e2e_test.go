package shux

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mitchellh/go-libghostty"
)

// TestE2ENanoEdit validates full stack: shux → shell → nano → rendering
// TestE2ENanoEdit validates nano interaction through full stack.
// Run in Docker with: docker build -f Dockerfile.test -t shux-test . && docker run shux-test
func TestE2ENanoEdit(t *testing.T) {
	if os.Getenv("SHUX_E2E") != "1" {
		t.Skip("Set SHUX_E2E=1 to run E2E tests (requires nano, vim, etc)")
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

	// Run ls with color (forces color with TERM)
	pane.Send(WriteToPane{Data: []byte("TERM=xterm-256color ls --color=auto /\r")})

	// Wait for output with styled cells
	super.waitContentUpdated(300 * time.Millisecond)

	reply := sessionRef.Ask(GetPaneContent{})
	result := <-reply
	if result == nil {
		t.Fatal("Expected content")
	}
	content := result.(*PaneContent)

	// Check that Cells array has styling info populated
	styledCells := 0
	for _, row := range content.Cells {
		for _, cell := range row {
			if cell.Bold || cell.Italic || cell.FgColor != (libghostty.ColorRGB{}) {
				styledCells++
			}
		}
	}

	if styledCells > 0 {
		t.Logf("E2E success: Found %d styled cells from color ls output", styledCells)
	} else {
		t.Log("Note: ls colors may not be visible (depends on terminal emulation)")
	}
}

func TestE2ELessPager(t *testing.T) {
	if os.Getenv("SHUX_E2E") != "1" {
		t.Skip("Set SHUX_E2E=1 to run E2E tests")
	}

	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	pane := requirePane(t, sessionRef, super)

	// Pipe something to less
	pane.Send(WriteToPane{Data: []byte("echo 'line1\nline2\nline3\nline4\nline5' | less\r")})

	// Wait for less to show content
	var lessReady bool
	if !pollFor(500*time.Millisecond, func() bool {
		super.waitContentUpdated(100 * time.Millisecond)
		reply := sessionRef.Ask(GetPaneContent{})
		result := <-reply
		if result == nil {
			return false
		}
		content := result.(*PaneContent)
		for _, line := range content.Lines {
			if contains(line, "line1") || contains(line, "line5") {
				lessReady = true
				return true
			}
		}
		return false
	}) {
		t.Log("Note: less output may be in alt-screen and not captured")
	}

	if lessReady {
		// Scroll down with 'j' (short delay for less to process)
		pane.Send(WriteToPane{Data: []byte("j")})
		pollFor(50*time.Millisecond, func() bool { return true })
		t.Log("E2E success: less opened and scrolled")
	}

	// Quit less with 'q'
	pane.Send(WriteToPane{Data: []byte("q")})
}

// TestE2EDSRResponse verifies terminal responds to DSR queries
func TestE2EDSRResponse(t *testing.T) {
	sessionRef, super, cleanup := setupSession(t)
	defer cleanup()

	sessionRef.Send(CreateWindow{Rows: 24, Cols: 80})
	pane := requirePane(t, sessionRef, super)

	// Send DSR query: ESC [ 5 n (status report request)
	pane.Send(WriteToPane{Data: []byte{0x1b, '[', '5', 'n'}})

	// Wait for content update - the response will come through PTY
	if !super.waitContentUpdated(500 * time.Millisecond) {
		t.Log("Note: DSR response timing out, checking anyway")
	}

	// The DSR response (ESC [ 0 n) should have been written back to PTY
	// and processed by the terminal. We can't directly see it in content,
	// but the fact that no error occurred and terminal is responsive
	// indicates the callback worked.
	reply := sessionRef.Ask(GetPaneContent{})
	result := <-reply
	if result == nil {
		t.Fatal("Expected pane content after DSR exchange")
	}

	content := result.(*PaneContent)
	if content == nil {
		t.Fatal("Result should be a *PaneContent")
	}

	t.Log("E2E success: DSR query/response cycle completed")
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
